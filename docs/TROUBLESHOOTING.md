# Troubleshooting

## UI Loads but API Calls Return 401

Clear browser storage and log in again.

Sounder uses:

```text
jabali-sounder-auth
```

The client also clears the old `jabali-manager-auth` key on 401 for rename
compatibility.

Check JWT config:

```bash
grep -n "jwt" -A3 /etc/jabali-sounder/config.toml
```

## Service Will Not Start

Check logs:

```bash
journalctl -u jabali-sounder.service -n 100 --no-pager
```

Current test server:

```bash
journalctl -u jabali-manager.service -n 100 --no-pager
```

Common causes:

- MariaDB DSN is missing or invalid.
- Secret key path is wrong.
- Port `127.0.0.1:8484` is already in use.
- Config path environment variable points to a missing file.

## `database.url not set`

Set either TOML:

```toml
[database]
url = "jabali_sounder:CHANGE_ME@tcp(127.0.0.1:3306)/jabali_sounder?parseTime=true&charset=utf8mb4&loc=UTC"
```

or environment:

```bash
export JABALI_SOUNDER_DATABASE_URL='...'
```

## Admin Login Fails

Reset the admin password:

```bash
JABALI_SOUNDER_CONFIG=/etc/jabali-sounder/config.toml \
  /opt/jabali-sounder/bin/jabali-sounder admin set-password -u admin
```

Verify the `admins` table exists:

```sql
SHOW TABLES LIKE 'admins';
```

## Server Enrollment Fails with `server_unreachable`

Sounder probes:

```text
GET <base_url>/health
```

Check from the Sounder host:

```bash
curl -k -i https://panel.example.com/health
```

Confirm:

- URL scheme is `http` or `https`.
- DNS or IP is reachable from the Sounder host.
- Firewall allows the connection.
- The Panel service is running.

## Credential Check Fails

Sounder calls:

```text
GET /api/v1/automation/status
```

with HMAC auth.

Likely causes:

- Wrong token ID.
- Wrong token secret.
- Token was revoked.
- Token lacks `read:status` or `read:*`.
- Managed Panel clock skew breaks timestamp validation.
- Request signing middleware on Panel changed.

## Monitor Shows Metrics Unavailable

Confirm the managed Panel supports enriched `/api/v1/automation/status`.

Required for full Monitor data:

- `cpu.usage_percent`
- `cpu.iowait_percent`
- `io.read_bps`
- `io.write_bps`
- `host` or `system` memory fields
- `host.partitions`

If only `{healthy,time}` is returned, Sounder will show partial or unavailable
metrics.

## Mail Tab Shows HTTP 404

This means the managed Panel does not yet expose the mail automation endpoints.

Expected endpoints:

- `/api/v1/automation/mail/mailboxes`
- `/api/v1/automation/mail/groups`
- `/api/v1/automation/mail/forwarders`
- `/api/v1/automation/mail/domain-forwarders`
- `/api/v1/automation/mail/autoresponders`

This is a Panel-side feature gap, not a Sounder UI failure.

## Empty Domains or Users

Check that the server is active and credentials are valid.

Then confirm the automation token has:

```text
read:domains
read:users
```

or:

```text
read:*
```

## Build Fails in UI

Clean generated TypeScript build info and rebuild:

```bash
rm -f manager-ui/tsconfig*.tsbuildinfo
cd manager-ui
npm run build
```

If dependencies are stale:

```bash
rm -rf manager-ui/node_modules
make ui-install
```

## Go Tests Include a Node Module Package

The current `go test ./...` may include:

```text
manager-ui/node_modules/flatted/golang/pkg/flatted
```

This is harmless in the current tree but noisy. Use the Makefile package
patterns for normal verification:

```bash
make test
```
