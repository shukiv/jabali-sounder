# Database

Jabali Sounder uses GORM with two database modes:

- `mysql`: server deployments, backed by MariaDB and SQL migrations.
- `sqlite`: local standalone desktop mode, backed by a local SQLite file and
  GORM AutoMigrate.

MySQL schema changes are managed by `golang-migrate` SQL files in:

```text
manager-api/internal/db/migrations/
```

## Migration Commands

Apply migrations:

```bash
make migrate-up
```

Roll back all migrations:

```bash
make migrate-down
```

Both commands require `database.url` in config or `JABALI_SOUNDER_DATABASE_URL`.
Use `database.driver = "mysql"` for normal server migrations.

SQLite desktop mode creates/updates its schema automatically at startup.

## Tables

### `servers`

Stores enrolled managed Jabali Panel servers.

| Column | Type | Purpose |
| --- | --- | --- |
| `id` | `CHAR(26)` | ULID primary key. |
| `name` | `VARCHAR(200)` | Unique display name. |
| `base_url` | `VARCHAR(500)` | Managed Panel base URL. |
| `token_id` | `CHAR(26)` | Automation token ID. |
| `token_secret_enc` | `BLOB` | Encrypted automation token secret. |
| `scopes` | `TEXT` | JSON array of requested scopes. |
| `version` | `VARCHAR(50)` | Version returned by managed Panel `/health`. |
| `capabilities` | `TEXT` | Reserved JSON array for future capability discovery. |
| `health_url` | `VARCHAR(500)` | Cached health endpoint URL. |
| `status` | `ENUM` | `active`, `disabled`, or `unreachable`. |
| `credential_status` | `ENUM` | `valid`, `invalid`, or `unknown`. |
| `last_heartbeat_at` | `DATETIME(3)` | Reserved/latest heartbeat timestamp. |
| `last_checked_at` | `DATETIME(3)` | Reserved/latest check timestamp. |
| `created_at` | `DATETIME(3)` | Creation timestamp. |
| `updated_at` | `DATETIME(3)` | Update timestamp. |
| `disabled_at` | `DATETIME(3)` | Reserved disabled timestamp. |

Indexes:

- unique `name`
- unique `token_id`

### `heartbeats`

Stores health check history.

| Column | Type | Purpose |
| --- | --- | --- |
| `id` | `CHAR(26)` | ULID primary key. |
| `server_id` | `CHAR(26)` | Foreign key to `servers.id`. |
| `healthy` | `TINYINT(1)` | Health result. |
| `version` | `VARCHAR(50)` | Version observed at check time. |
| `details` | `TEXT` | JSON details. |
| `checked_at` | `DATETIME(3)` | Check timestamp. |

Indexes:

- `(server_id, checked_at)`

Foreign keys:

- `server_id` references `servers(id)` with cascade delete.

### `admins`

Stores Sounder administrators.

| Column | Type | Purpose |
| --- | --- | --- |
| `id` | `CHAR(26)` | ULID primary key. |
| `username` | `VARCHAR(100)` | Unique login name. |
| `password_hash` | `VARCHAR(255)` | bcrypt password hash. |
| `created_at` | `DATETIME(3)` | Creation timestamp. |
| `updated_at` | `DATETIME(3)` | Update timestamp. |

Indexes:

- unique `username`

## Repository Pattern

Repositories live in `manager-api/internal/repository`.

Existing repositories:

- `ServerRepository`
- `HeartbeatRepository`
- `AdminRepository`

The repository layer translates `gorm.ErrRecordNotFound` into
`repository.ErrNotFound`.

## Adding a Migration

1. Add paired files:

   ```text
   00000N_name.up.sql
   00000N_name.down.sql
   ```

2. Keep DDL idempotent when practical.
3. Use explicit indexes and foreign keys.
4. Run:

   ```bash
   make migrate-up
   make test
   ```

5. If the migration is destructive, document the rollback and backup plan in
   the PR.
