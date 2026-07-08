# Operations

This document covers recurring operator workflows for Jabali Sounder.

## Service Health

API health:

```bash
curl -fsS http://127.0.0.1:8484/health
```

Current test server:

```bash
ssh root@10.0.3.14 'systemctl is-active jabali-manager.service; curl -fsS http://127.0.0.1:8484/health'
```

Expected response:

```json
{
  "status": "ok",
  "version": "dev"
}
```

## Logs

On systemd deployments:

```bash
journalctl -u jabali-sounder.service -f
```

Current test server service name:

```bash
journalctl -u jabali-manager.service -f
```

## Admin Password

Create or update an admin password:

```bash
JABALI_SOUNDER_CONFIG=/etc/jabali-sounder/config.toml \
  /opt/jabali-sounder/bin/jabali-sounder admin set-password -u admin
```

Current test server:

```bash
ssh root@10.0.3.14 '/opt/jabali-manager/bin/jabali-sounder admin set-password -u admin'
```

Use `--password` only in controlled automation. It can expose the password in
shell history or process lists.

## Enrolling a Managed Server

1. Create an automation token on the managed Jabali Panel server.
2. Give it only the scopes required by the desired Sounder tabs.
3. In Sounder, open `Servers`.
4. Add server name, base URL, token ID, token secret, and scopes.
5. Run the health check action after enrollment.

Recommended scopes:

```text
read:status
read:metrics
read:domains
read:users
read:applications
read:mail
```

`read:*` is accepted but broader than necessary.

## Checking Managed Server Credentials

From the UI:

- Open `Servers`.
- Click the refresh/check action for the target row.

From the API:

```bash
curl -X POST \
  -H "Authorization: Bearer $JWT" \
  http://127.0.0.1:8484/api/v1/admin/servers/<server-id>/check
```

The check updates:

- `status`: `active` or `unreachable`
- `credential_status`: `valid`, `invalid`, or `unknown`
- `version` when returned by `/health`

## Monitor Tab

The Monitor tab polls:

- live data every 5 seconds
- summary data every 60 seconds

Live data comes from managed Panel `/api/v1/automation/status`.

Summary data also uses `/domains`, `/users`, and `/applications` automation
endpoints.

## Mail Tab

The Mail tab is read-only and aggregates five resources:

- Mailboxes
- Mailbox forwarders
- Domain forwarders
- Groups
- Autoresponders

If a managed server does not support the mail automation endpoints, Sounder
shows a warning for that server and keeps the tables empty.

## Backup Before Deploy

For standalone deployments, back up:

- binary directory
- UI `dist`
- config
- secrets key
- MariaDB database

Example artifact backup:

```bash
stamp=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p /opt/jabali-sounder/backups/$stamp
cp -a /opt/jabali-sounder/bin /opt/jabali-sounder/backups/$stamp/bin
cp -a /opt/jabali-sounder/manager-ui/dist /opt/jabali-sounder/backups/$stamp/dist
```

Database backup depends on local MariaDB configuration.

## Rollback

Stop the service, restore backed-up binary and UI dist, then start the service:

```bash
systemctl stop jabali-sounder.service
cp -a /opt/jabali-sounder/backups/<stamp>/bin /opt/jabali-sounder/bin
cp -a /opt/jabali-sounder/backups/<stamp>/dist /opt/jabali-sounder/manager-ui/dist
systemctl start jabali-sounder.service
```

If migrations changed the database schema, rollback requires a database backup
or explicit migration rollback plan.
