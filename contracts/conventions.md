# Artemis API Conventions

This document describes the API conventions that Artemis already follows. These conventions are observed from the existing codebase and should guide the design of new endpoints.

---

## ID Format

**Convention:** All resource IDs use UUID v4 format.

**Implementation:** UUIDs are generated in the repository layer using `uuid.NewString()` from the `github.com/google/uuid` package.

**Example from code:**
```go
// db/repository.go:14
func CreateProfile(db *sql.DB, name string) (*Profile, error) {
    id := uuid.NewString()  // e.g., "a3c5e8b9-4f72-4b1e-9a8d-2c6f8e4b9a7c"
    // ...
}
```

---

## Timestamp Format

**Convention:** All timestamps are serialized in ISO 8601 format with timezone (`YYYY-MM-DDTHH:MM:SSZ`).

**Implementation:** The `time.Time` fields in model structs are formatted using `Format("2006-01-02T15:04:05Z")` when building detail responses. This ensures consistent ISO 8601 representation across all endpoints.

**Example from code:**
```go
// handlers/profile.go:134-135
resp := profileDetailResponse{
    CreatedAt: profile.CreatedAt.Format("2006-01-02T15:04:05Z"),
    UpdatedAt: profile.UpdatedAt.Format("2006-01-02T15:04:05Z"),
}
```

**Example response:**
```json
{
  "id": "a3c5e8b9-4f72-4b1e-9a8d-2c6f8e4b9a7c",
  "name": "Shakur",
  "createdAt": "2024-11-01T18:32:45Z",
  "updatedAt": "2024-11-01T18:32:45Z"
}
```

---

## Error Response Shape

**Convention:** All error responses use a consistent JSON shape: `{"error": "message here"}` with an appropriate HTTP status code.

**Implementation:** The `writeError(w, status, message)` helper in `handlers/helpers.go` is used throughout all handlers to ensure consistency.

**Status codes used:**
- `400 Bad Request` — Invalid request body, missing required fields, validation failures
- `404 Not Found` — Resource not found (detected via `isNotFound(err)` helper)
- `500 Internal Server Error` — Database failures, external service failures, unexpected errors

**Example from code:**
```go
// handlers/helpers.go:20-24
func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, map[string]string{"error": message})
}
```

**Example error response:**
```json
{
  "error": "Profile not found"
}
```

---

## List Endpoints

**Convention:** List endpoints return an array of resources directly (no wrapper object). Empty lists return `[]` instead of `null`.

**Implementation:** All list handlers normalize `nil` slices to empty slices before returning them. This ensures clients always receive a valid JSON array, never `null`.

**Pagination:** Not currently implemented. All list endpoints return the full set of results.

**Example from code:**
```go
// handlers/room.go:138-140
if rooms == nil {
    rooms = []db.Room{}
}
writeJSON(w, http.StatusOK, rooms)
```

**Example response:**
```json
[
  {
    "id": "room-uuid-1",
    "name": "Living Room",
    "icon": "sofa"
  },
  {
    "id": "room-uuid-2",
    "name": "Bedroom",
    "icon": "bed.double"
  }
]
```

---

## Route Naming and Base Path

**Convention:** All API routes are prefixed with a configurable base path (default: `/api`). Routes follow RESTful naming patterns with plural nouns for collections and singular nouns for specific resources.

**Base path:** Configured via `API_BASE_PATH` environment variable (defaults to `/api` in `config/config.go:72`).

**Route patterns:**
- Resource creation: `POST /api/{parent}/{parentId}/{resources}`
- Resource listing: `GET /api/{parent}/{parentId}/{resources}`
- Resource retrieval: `GET /api/{resource}/{id}`
- Resource update: `PUT /api/{resource}/{id}`
- Resource deletion: `DELETE /api/{resource}/{id}`
- Special operations: `PUT /api/{resource}/{id}/{operation}` or `POST /api/{resource}/{id}/{operation}`

**Example routes from code:**
```go
// main.go:83-87
mux.HandleFunc("POST "+cfg.APIBasePath+"/profile", profileHandler.HandleCreateProfile)
mux.HandleFunc("GET "+cfg.APIBasePath+"/profile/{id}", profileHandler.HandleGetProfile)
mux.HandleFunc("GET "+cfg.APIBasePath+"/profiles", profileHandler.HandleListProfiles)
mux.HandleFunc("PUT "+cfg.APIBasePath+"/profile/{id}", profileHandler.HandleUpdateProfile)
mux.HandleFunc("DELETE "+cfg.APIBasePath+"/profile/{id}", profileHandler.HandleDeleteProfile)

// main.go:90-91
mux.HandleFunc("POST "+cfg.APIBasePath+"/profile/{profileId}/rooms", roomHandler.HandleCreateRoom)
mux.HandleFunc("GET "+cfg.APIBasePath+"/profile/{profileId}/rooms", roomHandler.HandleListRooms)

// main.go:94
mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{id}/beacon", roomHandler.HandleUpdateRoomBeacon)
```

---

## Relationship: Business, Location, and Room

**Convention:** Artemis supports two modes: **home mode** (Profile-based) and **shop mode** (Business-based).

**Home mode hierarchy:**
- `Profile` → owns `Room` → has `Device`
- Rooms have a nullable `profileId` field (non-null in home mode)

**Shop mode hierarchy:**
- `Business` → has `Location` → has `Room`
- Rooms have a nullable `locationId` field (non-null in shop mode)

**Implementation:** The `Room` model has both nullable `profileId` and `locationId` fields. Only one is set per room, determining which mode it belongs to.

**Example from code:**
```go
// db/models.go:19-31
type Room struct {
    ID         string    `json:"id"`
    ProfileID  *string   `json:"profileId,omitempty"`  // nullable — nil for shop mode rooms
    Name       string    `json:"name"`
    Icon       string    `json:"icon"`
    BeaconUUID *string   `json:"beaconUuid,omitempty"`
    BeaconMajor *int     `json:"beaconMajor,omitempty"`
    BeaconMinor *int     `json:"beaconMinor,omitempty"`
    LocationID *string   `json:"locationId,omitempty"` // nullable — for shop mode rooms
    CreatedAt  time.Time `json:"createdAt"`
    UpdatedAt  time.Time `json:"updatedAt"`
}
```

**Creation endpoints:**
- Home mode room: `POST /api/profile/{profileId}/rooms` (handlers/room.go:69)
- Shop mode room: `POST /api/location/{locationId}/rooms` (handlers/location.go:252)

---

## Beacon Data Storage

**Convention:** BLE beacon data (UUID, major, minor) is stored directly on the `Room` model as nullable fields.

**Implementation:** The three beacon fields (`beaconUuid`, `beaconMajor`, `beaconMinor`) are optional and can be set via a dedicated endpoint. All three are nullable to support rooms without beacon configuration.

**Example from code:**
```go
// db/models.go:24-26
BeaconUUID  *string `json:"beaconUuid,omitempty"`  // iBeacon proximity UUID
BeaconMajor *int    `json:"beaconMajor,omitempty"` // iBeacon major value
BeaconMinor *int    `json:"beaconMinor,omitempty"` // iBeacon minor value
```

**Beacon update endpoint:**
```go
// main.go:94
mux.HandleFunc("PUT "+cfg.APIBasePath+"/room/{id}/beacon", roomHandler.HandleUpdateRoomBeacon)
```

**Request example:**
```json
{
  "uuid": "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
  "major": 1,
  "minor": 100
}
```

---

## HTTP Status Codes

**Convention:** Standard RESTful status codes are used consistently across all endpoints.

**Status codes by operation:**
- `200 OK` — Successful GET or PUT (resource returned in response body)
- `201 Created` — Successful POST (created resource returned in response body)
- `204 No Content` — Successful DELETE (no response body)
- `400 Bad Request` — Validation failure or malformed request
- `404 Not Found` — Resource does not exist
- `500 Internal Server Error` — Server-side failures (database errors, external service failures)

**Example from code:**
```go
// handlers/profile.go:80 (create)
writeJSON(w, http.StatusCreated, profile)

// handlers/profile.go:138 (get)
writeJSON(w, http.StatusOK, resp)

// handlers/profile.go:221 (delete)
w.WriteHeader(http.StatusNoContent)
```

---

## Gaps

The following inconsistencies exist across handlers:

1. **Timestamps in list responses:** List endpoints return raw `time.Time` values that get JSON-marshaled by the encoder, while detail (single-resource) responses manually format timestamps as ISO 8601 strings. This causes inconsistency: list responses show RFC3339Nano format (e.g., `2024-11-01T18:32:45.123456Z`), while detail responses show the manually formatted version (`2024-11-01T18:32:45Z`).
   - **Example:** `GET /api/profiles` returns Profile objects with `time.Time` fields; `GET /api/profile/{id}` returns a `profileDetailResponse` with manually formatted strings.
   - **Impact:** Clients must handle two different timestamp formats from the same API.

2. **Inconsistent JSON field casing for multi-word fields:** Some fields use camelCase (e.g., `beaconUuid`, `menuColor`), while the database columns use snake_case. The models correctly use Go struct tags to map to camelCase JSON, but there is no documented standard for new fields.
   - **Example:** `db/models.go:24` uses `json:"beaconUuid,omitempty"` (camelCase) for `BeaconUUID`.

3. **No standard for null vs. omitempty in JSON responses:** Some nullable fields use `omitempty` tags (e.g., `profileId,omitempty`), which omits the field entirely when null, while others might serialize as `null`. There is no documented rule for when to use `omitempty` vs. explicit `null`.

4. **Content-Type not validated on incoming requests:** Handlers parse JSON request bodies but do not explicitly check for `Content-Type: application/json`. This is forgiving but could lead to confusing errors if clients send the wrong content type.

---

## Proposed for New Endpoints

### Pagination Convention

**Proposed convention:** New list endpoints that may grow large should support optional pagination via query parameters.

**Query parameters:**
- `limit` (integer, default: 50) — Maximum number of items to return per page
- `offset` (integer, default: 0) — Number of items to skip

**Response format:**
```json
{
  "data": [ /* array of resources */ ],
  "pagination": {
    "offset": 0,
    "limit": 50,
    "total": 127
  }
}
```

**When to paginate:**
- Endpoints that list devices, menu options, or triggers may grow unbounded in production.
- Profile, Business, Location, and Room lists are expected to remain small and do not require pagination.

**Backward compatibility:** Existing unpaginated list endpoints should remain unchanged. New endpoints may adopt this convention from the start.

---

### Authentication Placeholder Approach

**Proposed convention:** New contracts should include an `Authorization` header placeholder to prepare for future authentication, even if not enforced yet.

**Header format:**
```
Authorization: Bearer <token>
```

**Current state:** No authentication is implemented. All endpoints are publicly accessible.

**Future direction:**
- Authentication will likely use JWT tokens or session-based auth.
- Contracts should document the `Authorization` header as "optional for now, required in future versions."
- When auth is implemented, endpoints will return `401 Unauthorized` for missing or invalid tokens.

**Example contract snippet:**
```markdown
### Authentication

**Header:** `Authorization: Bearer <token>` (optional, reserved for future use)

Once authentication is implemented, this header will be required. Endpoints will return `401 Unauthorized` if the token is missing or invalid.
```

---

**Document Version:** 1.0
**Last Updated:** 2026-11-01
