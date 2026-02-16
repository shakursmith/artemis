# Artemis

The backend API server for Pantheon - powering environment-based integrations.

## Overview

Artemis is a Go-based HTTP server that provides API endpoints for the Apollo iOS app. It handles integration logic, state management, and communication with external services.

## Project Structure

```
artemis/
├── main.go              # Application entry point and server setup
├── config/              # Configuration management
│   └── config.go       # Environment variable loading
├── handlers/            # HTTP request handlers
│   └── lightbulb.go    # Lightbulb toggle endpoint
├── middleware/          # HTTP middleware
│   ├── cors.go         # CORS headers for frontend requests
│   └── logging.go      # Request logging middleware
├── .env                 # Environment configuration (not committed)
├── .env.example         # Example environment configuration
└── go.mod              # Go module dependencies
```

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Git

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

The server will start on the configured port (default: 8080) and log its status:
```
🚀 Starting Artemis server in development mode
📍 Server will be available at http://0.0.0.0:8080
✅ Server is listening on 0.0.0.0:8080
📝 API endpoints:
   - POST /api/lightbulb/toggle - Toggle lightbulb state
   - GET  /api/health - Health check
```

### Building for Production

Build a binary:
```bash
go build -o artemis
./artemis
```

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

### Changing Configuration

To change the server port, edit `.env`:
```bash
PORT=3000
```

To change the API base path:
```bash
API_BASE_PATH=/v1
```

**Note:** After changing `.env`, restart the server for changes to take effect.

## API Endpoints

### POST /api/lightbulb/toggle

Toggles the lightbulb state and logs the event.

**Request Body:**
```json
{
  "isOn": true
}
```

**Response:**
```json
{
  "success": true,
  "message": "Lightbulb state updated successfully",
  "isOn": true,
  "timestamp": "2026-02-15T17:45:00Z"
}
```

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

Run tests with:
```bash
go test ./...
```

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
