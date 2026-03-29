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
│   ├── helpers.go      # Shared JSON response utilities
│   ├── profile.go      # Profile CRUD endpoints
│   ├── room.go         # Room CRUD + beacon config endpoints
│   ├── device.go       # Device CRUD + assign/unassign endpoints
│   ├── profile_test.go # Profile handler tests
│   ├── room_test.go    # Room handler tests
│   ├── device_test.go  # Device handler tests
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

### Profile, Room & Device Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/profile` | Create a new profile |
| GET | `/api/profile/{id}` | Get profile with rooms and devices |
| GET | `/api/profiles` | List all profiles |
| PUT | `/api/profile/{id}` | Update profile name |
| DELETE | `/api/profile/{id}` | Delete profile (cascades to rooms & devices) |
| POST | `/api/profile/{profileId}/rooms` | Create a room |
| GET | `/api/profile/{profileId}/rooms` | List rooms for a profile |
| GET | `/api/room/{id}` | Get room with its devices |
| PUT | `/api/room/{id}` | Update room name and icon |
| PUT | `/api/room/{id}/beacon` | Set iBeacon config for a room |
| DELETE | `/api/room/{id}` | Delete room (unassigns devices) |
| POST | `/api/profile/{profileId}/devices` | Register a new device |
| GET | `/api/profile/{profileId}/devices` | List devices for a profile |
| GET | `/api/device/{id}` | Get a device |
| PUT | `/api/device/{id}` | Update device name |
| PUT | `/api/device/{id}/assign` | Assign device to a room |
| PUT | `/api/device/{id}/unassign` | Remove device from room |
| DELETE | `/api/device/{id}` | Delete a device |

#### Example: Full onboarding flow via curl

```bash
# 1. Create a profile
curl -s -X POST http://localhost:8080/api/profile \
  -H 'Content-Type: application/json' \
  -d '{"name": "Shakur"}' | jq .

# 2. Create rooms (use the profile ID from step 1)
curl -s -X POST http://localhost:8080/api/profile/<PROFILE_ID>/rooms \
  -H 'Content-Type: application/json' \
  -d '{"name": "Living Room", "icon": "sofa"}' | jq .

curl -s -X POST http://localhost:8080/api/profile/<PROFILE_ID>/rooms \
  -H 'Content-Type: application/json' \
  -d '{"name": "Office", "icon": "desktopcomputer"}' | jq .

# 3. Register a device
curl -s -X POST http://localhost:8080/api/profile/<PROFILE_ID>/devices \
  -H 'Content-Type: application/json' \
  -d '{"name": "Desk Lamp", "deviceType": "govee_light", "model": "H6160"}' | jq .

# 4. Assign device to a room
curl -s -X PUT http://localhost:8080/api/device/<DEVICE_ID>/assign \
  -H 'Content-Type: application/json' \
  -d '{"roomId": "<ROOM_ID>"}' | jq .

# 5. Set beacon config for a room
curl -s -X PUT http://localhost:8080/api/room/<ROOM_ID>/beacon \
  -H 'Content-Type: application/json' \
  -d '{"uuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0", "major": 1, "minor": 1}' | jq .

# 6. Get the full profile (enriched with rooms + devices)
curl -s http://localhost:8080/api/profile/<PROFILE_ID> | jq .
```

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

Run specific test packages with verbose output:
```bash
go test ./db/ -v        # 40 DB repository tests
go test ./handlers/ -v  # 50 HTTP handler tests
```

Current test coverage:
- `db/repository_test.go` — 40 tests covering all CRUD operations, cascade deletes, beacon configuration, device assignment, and full integration flows
- `handlers/profile_test.go` — Profile handler tests (create, get enriched, list, update, delete, cascade)
- `handlers/room_test.go` — Room handler tests (create, list, get enriched, update, beacon config, delete, unassign on delete)
- `handlers/device_test.go` — Device handler tests (create, list, get, update, assign, unassign, delete, full lifecycle flow)

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
