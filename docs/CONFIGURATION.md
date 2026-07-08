# Configuration

Jabali Sounder reads TOML configuration, applies defaults, then applies
environment variable overrides.

Default config path:

```text
/etc/jabali-sounder/config.toml
```

Override the config path with:

```bash
JABALI_SOUNDER_CONFIG=/path/to/config.toml
```

Existing installs may still use `JABALI_MANAGER_CONFIG`; Sounder accepts it as
a fallback.

## TOML Example

```toml
[server]
addr = "127.0.0.1:8484"
env  = "production"

[log]
level  = "info"
format = "json"

[database]
driver = "mysql"
url = "jabali_sounder:CHANGE_ME@tcp(127.0.0.1:3306)/jabali_sounder?parseTime=true&charset=utf8mb4&loc=UTC"

[secrets]
key_file = "/etc/jabali-sounder/secrets.key"

[jwt]
secret = "replace-with-a-long-random-secret"
```

`config.example.toml` contains a minimal template.

## Sections

### `[server]`

| Key | Default | Description |
| --- | --- | --- |
| `addr` | `127.0.0.1:8484` | API listen address. |
| `env` | `development` | Environment label used in logs. |

### `[log]`

| Key | Default | Description |
| --- | --- | --- |
| `level` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `format` | `text` | `text` or `json`. |

### `[database]`

| Key | Default | Description |
| --- | --- | --- |
| `driver` | `mysql` | `mysql` for server deployments, `sqlite` for desktop/local standalone mode. |
| `url` | empty | MariaDB DSN. Required for persistent operation. |

For MySQL/MariaDB, use `parseTime=true`, `charset=utf8mb4`, and `loc=UTC` in
the DSN. For SQLite, set `url` to the database file path.

### `[secrets]`

| Key | Default | Description |
| --- | --- | --- |
| `key_file` | `/etc/jabali-sounder/secrets.key` | 32-byte AES key used to encrypt automation token secrets. |

Generate the key:

```bash
install -d -m 750 /etc/jabali-sounder
openssl rand -out /etc/jabali-sounder/secrets.key 32
chmod 600 /etc/jabali-sounder/secrets.key
```

### `[jwt]`

| Key | Default | Description |
| --- | --- | --- |
| `secret` | empty | JWT signing secret for admin sessions. |

If no JWT secret is configured, Sounder logs a warning and uses a development
fallback. Do not rely on the fallback in production.

## Environment Variables

Primary variables:

| Variable | TOML equivalent |
| --- | --- |
| `JABALI_SOUNDER_CONFIG` | config file path |
| `JABALI_SOUNDER_ADDR` | `server.addr` |
| `JABALI_SOUNDER_ENV` | `server.env` |
| `JABALI_SOUNDER_DATABASE_URL` | `database.url` |
| `JABALI_SOUNDER_DATABASE_DRIVER` | `database.driver` |
| `JABALI_SOUNDER_SECRET_KEY_FILE` | `secrets.key_file` |
| `JABALI_SOUNDER_JWT_SECRET` | `jwt.secret` |
| `JABALI_SOUNDER_MIGRATIONS_DIR` | migration directory override |
| `LOG_LEVEL` | `log.level` |
| `LOG_FORMAT` | `log.format` |

Compatibility variables still accepted:

- `JABALI_MANAGER_CONFIG`
- `JABALI_MANAGER_ADDR`
- `JABALI_MANAGER_ENV`
- `JABALI_MANAGER_DATABASE_URL`
- `JABALI_MANAGER_DATABASE_DRIVER`
- `JABALI_MANAGER_SECRET_KEY_FILE`
- `JABALI_MANAGER_JWT_SECRET`
- `JABALI_MANAGER_MIGRATIONS_DIR`

When both names are set, the `JABALI_SOUNDER_*` value wins.

## Current Test Deployment Note

The `10.0.3.14` test host still stores config under:

```text
/etc/jabali-manager/config.toml
```

The systemd unit points `JABALI_SOUNDER_CONFIG` at that file for continuity.
New installs should prefer `/etc/jabali-sounder/config.toml`.
