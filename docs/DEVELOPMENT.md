# Development

This project mirrors Jabali Panel's conventions where possible. Keep changes
small, explicit, and aligned with existing route-family and repository patterns.

## Prerequisites

- Go 1.25
- Node.js compatible with the checked-in Vite/React toolchain
- npm
- MariaDB for persistent local testing
- `golangci-lint` for `make lint`

Install UI dependencies:

```bash
make ui-install
```

## Local Configuration

Create a local config:

```bash
cp config.example.toml config.local.toml
```

Set a database URL in the config or environment:

```bash
export JABALI_SOUNDER_DATABASE_URL='jabali_sounder:CHANGE_ME@tcp(127.0.0.1:3306)/jabali_sounder?parseTime=true&charset=utf8mb4&loc=UTC'
```

Generate a local secret key:

```bash
openssl rand -out ./secrets.local.key 32
```

Set `[secrets].key_file` to `./secrets.local.key`.

## Database

Run migrations:

```bash
JABALI_SOUNDER_CONFIG=./config.local.toml make migrate-up
```

Rollback all migrations:

```bash
JABALI_SOUNDER_CONFIG=./config.local.toml make migrate-down
```

## Admin User

Create or update an admin:

```bash
JABALI_SOUNDER_CONFIG=./config.local.toml go run ./manager-api/cmd/server admin set-password -u admin
```

The current CLI reads the password from `/dev/tty`; in non-interactive
automation, pass `--password`.

## Running Locally

API:

```bash
JABALI_SOUNDER_CONFIG=./config.local.toml make run
```

UI build:

```bash
make ui-build
```

The repository does not currently include a local reverse proxy config. In
production, nginx serves `manager-ui/dist` and proxies `/api/v1` to the API.

## Tests and Checks

Go:

```bash
make fmt
make vet
make test
make lint
```

UI:

```bash
cd manager-ui
npm run lint
npm test
npm run build
```

Known current lint warning:

```text
manager-ui/src/theme/ThemeModeContext.tsx
Fast refresh only works when a file only exports components.
```

That warning is non-blocking and pre-existing.

## Code Conventions

Backend:

- Use `slog`.
- Wrap errors with `%w`.
- Keep one route family per file.
- Keep one repository per aggregate.
- Use GORM parameters; never build SQL with string concatenation.
- Return 400 for malformed JSON and 422 for valid JSON that violates domain
  rules.

Frontend:

- Use TanStack Query hooks in `manager-ui/src/hooks`.
- Keep API response types in `manager-ui/src/types.ts`.
- Use Ant Design components consistently with existing pages.
- Keep data tables dense and operational.

## Adding a New Managed-Server Resource

1. Add or confirm the Jabali Panel automation endpoint.
2. Add a typed remote client in `manager-api/internal/remote`.
3. Add an admin aggregation route in `manager-api/internal/api`.
4. Mount it from `manager-api/internal/app/app.go`.
5. Add TypeScript types and a query hook.
6. Add a page or extend an existing page.
7. Verify per-server failures do not break the whole response.
8. Add tests where the risk warrants it.
