# API Reference

All API routes except `/health` and `/api/v1/auth/login` require a JWT bearer
token.

```http
Authorization: Bearer <jwt>
```

List endpoints use the standard envelope:

```json
{
  "data": [],
  "total": 0,
  "page": 1,
  "page_size": 0
}
```

## Health

### `GET /health`

Unauthenticated process health.

Response:

```json
{
  "status": "ok",
  "version": "dev"
}
```

## Auth

### `POST /api/v1/auth/login`

Request:

```json
{
  "username": "admin",
  "password": "secret"
}
```

Response:

```json
{
  "token": "<jwt>",
  "expires_at": "2026-07-08T18:00:00Z",
  "admin": {
    "id": "01...",
    "username": "admin"
  }
}
```

### `GET /api/v1/auth/me`

Response:

```json
{
  "id": "01...",
  "username": "admin"
}
```

## Managed Servers

### `GET /api/v1/admin/servers`

Returns enrolled servers.

Response item:

```json
{
  "id": "01...",
  "name": "panel-01",
  "base_url": "https://panel-01.example.com",
  "token_id": "01...",
  "scopes": ["read:*"],
  "version": "v0.1.0",
  "status": "active",
  "credential_status": "valid",
  "created_at": "2026-07-08T18:00:00Z",
  "updated_at": "2026-07-08T18:00:00Z"
}
```

### `POST /api/v1/admin/servers`

Enrolls a managed Jabali Panel server. The API validates the base URL and probes
`/health` before storing the server.

Request:

```json
{
  "name": "panel-01",
  "base_url": "https://panel-01.example.com",
  "token_id": "01...",
  "token_secret": "secret-shown-once-by-panel",
  "scopes": ["read:domains", "read:users", "read:status", "read:metrics"]
}
```

If `scopes` is omitted or null, Sounder stores `["read:*"]`.

### `GET /api/v1/admin/servers/:id`

Returns one enrolled server.

### `PATCH /api/v1/admin/servers/:id`

Updates mutable server fields.

Request:

```json
{
  "name": "new-name",
  "base_url": "https://new-url.example.com",
  "scopes": ["read:*"]
}
```

All fields are optional.

### `DELETE /api/v1/admin/servers/:id`

Soft-disables a server by setting `status = "disabled"`.

Response:

```json
{
  "id": "01...",
  "disabled": true
}
```

### `POST /api/v1/admin/servers/:id/check`

Probes the managed server `/health` and HMAC-protected
`/api/v1/automation/status`, then updates Sounder server status fields.

Response:

```json
{
  "reachable": true,
  "healthy": true,
  "credential_valid": true,
  "version": "v0.1.0",
  "health_code": 200,
  "status_code": 200
}
```

## Dashboard

### `GET /api/v1/admin/dashboard`

Returns one summary row per enrolled server.

Response item:

```json
{
  "id": "01...",
  "name": "panel-01",
  "base_url": "https://panel-01.example.com",
  "status": "active",
  "credential_status": "valid",
  "version": "v0.1.0",
  "healthy": true
}
```

## Inventory

### `GET /api/v1/admin/domains`

Aggregates domains from active servers via managed Panel
`GET /api/v1/automation/domains`.

Response item:

```json
{
  "id": "01...",
  "name": "example.com",
  "user_id": "01...",
  "is_enabled": true,
  "server_id": "01...",
  "server_name": "panel-01"
}
```

### `GET /api/v1/admin/users`

Aggregates users from active servers via managed Panel
`GET /api/v1/automation/users`.

Response item:

```json
{
  "id": "01...",
  "email": "user@example.com",
  "username": "user",
  "package_id": "01...",
  "is_admin": false,
  "server_id": "01...",
  "server_name": "panel-01"
}
```

## Monitor

### `GET /api/v1/admin/monitor/live`

Returns live per-server metrics from managed Panel
`GET /api/v1/automation/status`.

Response item:

```json
{
  "server": {
    "id": "01...",
    "name": "panel-01",
    "base_url": "https://panel-01.example.com",
    "status": "active",
    "credential_status": "valid",
    "version": "v0.1.0"
  },
  "available": true,
  "as_of": "2026-07-08T18:00:00Z",
  "cpu_percent": 7.4,
  "ram_used_bytes": 1073741824,
  "ram_total_bytes": 4294967296,
  "ram_percent": 25,
  "io_wait_percent": 0.1,
  "io_read_bps": 1024,
  "io_write_bps": 2048,
  "load1": 0.5,
  "load5": 0.4,
  "load15": 0.3,
  "warming_up": false
}
```

When metrics are unavailable, `available` is false and `error` explains the
per-server failure.

### `GET /api/v1/admin/monitor/summary`

Returns slower-changing server summary data:

- Disk usage from `/api/v1/automation/status`.
- Account count from `/api/v1/automation/users`.
- Domain count from `/api/v1/automation/domains`.
- Application count from `/api/v1/automation/applications`.

Response item:

```json
{
  "server": {},
  "available": true,
  "disk_used_bytes": 2147483648,
  "disk_total_bytes": 10737418240,
  "disk_percent": 20,
  "accounts_total": 5,
  "domains_total": 11,
  "applications_total": 3
}
```

## Mail

### `GET /api/v1/admin/mail`

Aggregates mail inventory from active servers.

Response item:

```json
{
  "server": {},
  "available": false,
  "mailboxes": [],
  "groups": [],
  "forwarders": [],
  "domain_forwarders": [],
  "autoresponders": [],
  "error": "mailboxes unavailable: HTTP 404: mailboxes: HTTP 404"
}
```

The endpoint always returns arrays, even when a managed server is missing one
or more mail automation endpoints.

Sounder currently calls these managed Panel endpoints:

- `GET /api/v1/automation/mail/mailboxes`
- `GET /api/v1/automation/mail/groups`
- `GET /api/v1/automation/mail/forwarders`
- `GET /api/v1/automation/mail/domain-forwarders`
- `GET /api/v1/automation/mail/autoresponders`

These endpoints require Panel support. Until they exist on managed servers,
Sounder reports per-server 404s.

## Error Conventions

Malformed JSON returns:

```json
{
  "error": "malformed_json",
  "detail": "..."
}
```

Domain/validation failures generally return HTTP 422.

Missing records return:

```json
{
  "error": "not_found"
}
```
