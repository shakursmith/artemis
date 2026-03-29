# Artemis

The backend API server for Pantheon - powering environment-based integrations.

## Overview

Artemis is a Go-based HTTP server that provides API endpoints for the Apollo iOS app. It handles integration logic, state management, and communication with external services. Profile, room, and device data is persisted in a local SQLite database.

## Project Structure

```
artemis/
├── main.go              # Application entry point and server setup
├── config/              # Configuration management
│   └── config.go       # Environment variable loading
├── db/                  # SQLite database layer
│   ├── database.go     # Database initialization (WAL mode, FK enforcement)
│   ├── migrations.go   # Schema definitions (profiles, rooms, devices)
│   ├── models.go       # Go structs for database entities
│   ├── repository.go   # CRUD operations for all entities
│   └── repository_test.go  # 40 tests covering all operations
├── handlers/            # HTTP request handlers
│   ├── lightbulb.go    # Lightbulb toggle endpoint
│   ├── govee.go        # Govee smart light endpoints
│   ├── firetv.go       # Fire TV remote control endpoints
│   └── camera.go       # Wyze camera endpoints
├── middleware/          # HTTP middleware
│   ├── cors.go         # CORS headers for frontend requests
│   └── logging.go      # Request logging middleware
├── govee/              # Govee API client
├── firetv/             # Fire TV microservice client
├── camera/             # Wyze Bridge client
├── .env                 # Environment configuration (not committed)
├── .env.example         # Example environment configuration
└── go.mod              # Go module dependencies
```

## Getting Started

### Prerequisites

- Go 1.24 or higher
- Git
- GCC / C compiler (required by `go-sqlite3` — comes preinstalled on macOS)

### Installation

1. Clone the repository (if not already cloned)
2. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```

### Running the Server

Start the server with:
```bash
go run main.go
```

The server will start on the configured port (default: 8080). On startup it:
- Initializes the SQLite database (creates `./pantheon.db` if it doesn't exist)
- Runs schema migrations (safe to run repeatedly)
- Connects to external service clients (Govee, Fire TV, Wyze)

```
🗄️  Database initialized at ./pantheon.db
🗄️  Database ready at ./pantheon.db
💡 Primary Govee client initialized
🚀 Starting Artemis server in development mode
📍 Server will be available at http://0.0.0.0:8080
✅ Server is listening on 0.0.0.0:8080
```

### Building for Production

Build a binary:
```bash
go build -o artemis
./artemis
```

## Database

Artemis uses SQLite for local persistence of profiles, rooms, and devices. The database is created automatically on first run.

### Schema

Three tables with foreign key relationships:

```
profiles
├── id (TEXT PK)
├── name
├── created_at
└── updated_at

rooms
├── id (TEXT PK)
├── profile_id → profiles(id) ON DELETE CASCADE
├── name
├── icon (SF Symbol name)
├── beacon_uuid (iBeacon UUID, optional)
├── beacon_major (iBeacon major, optional)
├── beacon_minor (iBeacon minor, optional)
├── created_at
└── updated_at

devices
├── id (TEXT PK)
├── profile_id → profiles(id) ON DELETE CASCADE
├── room_id → rooms(id) ON DELETE SET NULL
├── name
├── device_type ("govee_light", "fire_tv", "wyze_camera", "generic")
├── external_id (third-party service ID, optional)
├── model (optional)
├── metadata (JSON blob, optional)
├── created_at
└── updated_at
```

**Cascade behavior:**
- Deleting a profile deletes all its rooms and devices
- Deleting a room unassigns its devices (sets `room_id` to NULL)

### Inspecting the Database

You can use the `sqlite3` CLI (preinstalled on macOS) to inspect the database:

```bash
# Open the database
sqlite3 pantheon.db

# Show all tables
.tables

# Show schema for a table
.schema profiles
.schema rooms
.schema devices

# List all profiles
SELECT * FROM profiles;

# List rooms with their profile
SELECT r.name, r.icon, r.beacon_uuid, p.name as profile_name
FROM rooms r JOIN profiles p ON r.profile_id = p.id;

# List devices and which room they're in
SELECT d.name, d.device_type, r.name as room_name
FROM devices d LEFT JOIN rooms r ON d.room_id = r.id;

# Pretty-print mode
.mode column
.headers on
SELECT * FROM rooms;

# Exit
.quit
```

**Tip:** The database uses WAL mode, so you can read from it while the server is running without conflicts.

### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_PATH` | Path to SQLite database file | `./pantheon.db` |

Set `DB_PATH=:memory:` for an ephemeral in-memory database (useful for testing).

## Configuration

All configuration is managed through environment variables. Copy `.env.example` to `.env` and modify as needed.

### Available Configuration Options

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `HOST` | Server host address | `0.0.0.0` |
| `ENVIRONMENT` | Runtime environment (development/staging/production) | `development` |
| `API_BASE_PATH` | Base path for API routes | `/api` |
| `ENABLE_REQUEST_LOGGING` | Enable HTTP request logging | `true` |
| `GOVEE_API_KEY` | Govee API key (required) | — |
| `GOVEE_API_KEY_SECONDARY` | Second Govee account key (optional) | — |
| `FIRETV_SERVICE_URL` | Fire TV Python service URL | `http://localhost:9090` |
| `WYZE_BRIDGE_URL` | Wyze Bridge URL | `http://localhost:5050` |
| `WYZE_BRIDGE_API_KEY` | Wyze Bridge API key (optional) | — |
| `DB_PATH` | SQLite database path | `./pantheon.db` |

**Note:** After changing `.env`, restart the server for changes to take effect.

## API Endpoints

### Integration Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/lightbulb/toggle` | Toggle lightbulb state |
| GET | `/api/govee/devices` | List all Govee devices |
| POST | `/api/govee/devices/control` | Control Govee device |
| GET | `/api/govee/devices/state` | Query device state |
| GET | `/api/firetv/discover` | Discover Fire TV devices |
| POST | `/api/firetv/pair` | Pair with Fire TV |
| POST | `/api/firetv/command` | Send Fire TV command |
| GET | `/api/cameras` | List Wyze cameras |
| GET | `/api/cameras/stream` | Get camera stream URLs |
| GET | `/api/health` | Health check |

### GET /api/health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "service": "artemis"
}
```

## Development

### Running with Auto-Reload

For development, you can use tools like `air` for auto-reloading:
```bash
go install github.com/cosmtrek/air@latest
air
```

### Testing

Run all tests:
```bash
go test ./...
```

Run database tests with verbose output:
```bash
go test ./db/ -v
```

Current test coverage:
- `db/repository_test.go` — 40 tests covering all CRUD operations, cascade deletes, beacon configuration, device assignment, and full integration flows

## Deployment

When deploying to production:

1. Set `ENVIRONMENT=production` in your `.env`
2. Configure appropriate `HOST` and `PORT` values
3. Build a production binary with `go build -o artemis`
4. Run the binary or use a process manager like systemd

## Connecting with Frontend

The frontend (Apollo) should be configured to point to this server's address. For local development:
- If testing on simulator: `http://localhost:8080`
- If testing on physical device: `http://<your-computer-ip>:8080`

Make sure CORS is enabled (it is by default) to allow the frontend to make requests.
