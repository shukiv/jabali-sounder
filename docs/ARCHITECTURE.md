# Architecture

Jabali Sounder is a two-part application:

- `manager-api`: Go API server, CLI, MariaDB migrations, repositories, and
  remote Jabali Panel automation clients.
- `manager-ui`: React/Vite SPA served separately by nginx or any static file
  server and backed by `/api/v1`.

The system is intentionally a control plane, not an agent. Managed Jabali Panel
servers remain the source of truth for their own users, domains, applications,
metrics, and mail stack.

## High-Level Flow

```text
Browser
  |
  | JWT bearer token
  v
Jabali Sounder API
  |
  | HMAC-signed automation requests
  v
Managed Jabali Panel servers
```

Sounder persists only manager-side state:

- Enrolled servers and their automation credential metadata.
- Encrypted automation token secrets.
- Server status fields and heartbeat rows.
- Sounder admin users and password hashes.

Inventory data is fetched from managed servers on demand. It is not currently
cached in the Sounder database.

## Backend Packages

```text
manager-api/cmd/server
  Cobra CLI entrypoint, config loading, serve/migrate/admin commands.

manager-api/internal/app
  Gin engine construction and route registration.

manager-api/internal/api
  Route-family handlers. One file per resource family.

manager-api/internal/db
  GORM/MariaDB setup and migrations.

manager-api/internal/models
  GORM models.

manager-api/internal/repository
  Repository interfaces and GORM implementations.

manager-api/internal/remote
  HMAC-signed clients for managed Jabali Panel automation APIs.

manager-api/internal/middleware
  JWT authentication middleware.

manager-api/internal/secrets
  Local encryption key handling for automation token secrets.
```

## Frontend Structure

```text
manager-ui/src/App.tsx
  Router and auth gate.

manager-ui/src/shells/AdminLayout.tsx
  Main authenticated shell and navigation.

manager-ui/src/admin
  Page components: Dashboard, Servers, Monitor, Mail, Domains, Users, Login.

manager-ui/src/hooks
  TanStack Query hooks and auth helpers.

manager-ui/src/types.ts
  Shared TypeScript response types.
```

## Route Registration Pattern

The API follows the Jabali Panel route-family convention:

```go
func RegisterXxxRoutes(g *gin.RouterGroup, cfg XxxHandlerConfig)
```

Required dependencies are checked at registration time. If a repository is nil
because the database is disabled, that route family is not mounted.

Mounted route families:

- `RegisterHealthRoutes`
- `RegisterAuthRoutes`
- `RegisterServerRoutes`
- `RegisterDashboardRoutes`
- `RegisterInventoryRoutes`
- `RegisterMonitorRoutes`
- `RegisterMailRoutes`

## Persistence Model

Tables:

- `servers`
- `heartbeats`
- `admins`

Server enrollment stores:

- Server display name and base URL.
- Automation token ID.
- Encrypted automation token secret.
- Requested scopes.
- Version, health URL, status, credential status.
- Timestamps.

`heartbeats` records health check details, but the current UI primarily uses
the latest status fields on `servers`.

## Remote Authentication

Remote calls to managed Panels use:

```text
Authorization: Jabali-HMAC kid=<token-id>, ts=<unix>, sig=<hex>
```

The signed string is:

```text
METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(body))
```

`RequestURI` includes the query string. This is covered by
`manager-api/internal/remote/client_test.go` because signing the path without
the query string would break authenticated calls.

## Cross-Server Aggregation

Inventory, monitor, and mail endpoints fan out across active enrolled servers
with an errgroup concurrency limit of 8. Per-server failures are isolated:

- Inventory endpoints log and skip failed servers.
- Monitor endpoints return per-server `available` and `error` fields.
- Mail endpoint returns per-server empty arrays plus `error` when a managed
  server does not expose the expected automation endpoints.

## Current Deployment Shape

The current test deployment on `10.0.3.14` uses:

- API on `127.0.0.1:8484`
- UI served by nginx on `:8485`
- systemd service running `/opt/jabali-manager/bin/jabali-sounder serve`
- compatibility config path `/etc/jabali-manager/config.toml`

The install path still contains `jabali-manager` for continuity, but the binary
and app branding are Jabali Sounder.
