<p align="center">
  <img src="manager-ui/src/assets/jabali-sounder.svg" alt="Jabali Sounder" width="320" />
</p>

<h1 align="center">Jabali Sounder</h1>

<p align="center">
  Central control plane for a sounder of Jabali Panel servers.
</p>

<p align="center">
  <a href="https://github.com/shukiv/jabali-sounder/releases"><img alt="Release" src="https://img.shields.io/github/v/release/shukiv/jabali-sounder?sort=semver" /></a>
  <img alt="Status" src="https://img.shields.io/badge/status-beta-yellow" />
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" />
  <img alt="React" src="https://img.shields.io/badge/React-18-20232A?logo=react&logoColor=61DAFB" />
  <img alt="Ant Design" src="https://img.shields.io/badge/Ant%20Design-6-0170FE?logo=antdesign&logoColor=white" />
  <img alt="Vite" src="https://img.shields.io/badge/Vite-6-646CFF?logo=vite&logoColor=white" />
  <img alt="SQLite" src="https://img.shields.io/badge/SQLite-003B57?logo=sqlite&logoColor=white" />
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/License-AGPL--3.0-blue" /></a>
</p>

---

Jabali Sounder is a central control plane for managing multiple Jabali Panel
servers from one admin UI. It talks to each managed server through the existing
Jabali Panel HTTP API and scoped automation tokens. It does not SSH into managed
nodes.

> A *sounder* is a group of wild boar (*jabalí*) — the mark above, and the
> reason this control plane herds many panels from one place.

## Features

**Fleet monitoring**
- Background health poller: status history + heartbeats without a manual check.
- Resource trends (CPU / RAM / disk / load) with range-selectable charts
  (6h / 24h / 7d / 30d) and per-server + fleet uptime / SLA.
- TLS certificate-expiry tracking and version-drift overview.

**Alerting & incidents**
- Configurable per-metric alert rules (threshold + duration + severity).
- Delivery channels: webhook (Slack/Discord/Mattermost), ntfy push, SMTP email,
  and PagerDuty, with severity-based routing.
- In-app incidents with acknowledge / snooze / mute and one-shot escalation.
- Maintenance windows that suppress alerts during planned work.

**Remediation & ops**
- Scoped write actions — restart service, enable/disable user, suspend/unsuspend
  domain, purge cache, create backup — gated by confirm + role and audited.
- Bulk operations across selected servers.
- Backup tracking (runs polled to completion) with stale-backup alerts.
- Opt-in auto-restart remediation after repeated failed checks.

**Multi-operator & auth**
- Multiple admins with RBAC (viewer / operator / owner).
- TOTP two-factor authentication and server-side session management.
- Login throttling: per-IP lockout plus per-account backoff + brute-force alert.

**Security & compliance**
- Per-server automation token IDs with encrypted secrets; HMAC-signed requests.
- Read-only API tokens with coarse scopes, source-IP/CIDR allowlist, per-token
  rate limit, and one-click rotation.
- Persisted audit log with filtering and CSV export.
- Compliance/policy drift detection (weak TLS, invalid credentials, cert expiry,
  version drift).

**Observability export & inventory**
- Prometheus `/metrics` endpoint for external Grafana/Alertmanager.
- Fleet CSV export and scheduled fleet-summary reports.
- Cross-server domain and user inventory, plus a global search palette (⌘/Ctrl+K).
- Mail tab (mailboxes, forwarders, groups, autoresponders) — ready on the Sounder
  side; requires the panel automation mail endpoints (see below).

**Updates & deployment**
- Version endpoint + in-app "update available" notice; checksum-verified desktop
  self-update and a server update script. See [Updating](docs/UPDATING.md).
- Two deploy targets: a headless Linux server (single binary, embedded UI) and a
  standalone Wails desktop app for Windows / macOS / Linux with local SQLite.

## Downloads

Grab the standalone desktop app for your platform from the
[latest release](https://github.com/shukiv/jabali-sounder/releases/latest) — each
file is version-stamped:

| Platform | File on the release page |
|----------|--------------------------|
| Linux (x86-64) | `jabali-sounder-linux-amd64-<version>` |
| Windows (x86-64) | `jabali-sounder-windows-amd64-<version>.exe` |
| macOS (Apple Silicon) | `jabali-sounder-macos-arm64-<version>.dmg` |

All versions: [Releases](https://github.com/shukiv/jabali-sounder/releases).

Notes:

- Binaries are **unsigned**. macOS: right-click the app → **Open** the first
  time (Gatekeeper). Windows: **More info → Run anyway** (SmartScreen).
- macOS build is Apple Silicon (arm64) only. On Linux, `chmod +x` the binary
  before running.

## Install (Linux server)

One-liner for a Debian/Ubuntu server. Installs a single static binary that
serves both the API and the UI on one port (no Go, Node, or MariaDB required),
with a hardened systemd service and a SQLite database:

```bash
curl -fsSL https://raw.githubusercontent.com/shukiv/jabali-sounder/main/install.sh | sudo bash
```

The installer prints the URL and a generated admin password when it finishes.
Change the password in **Settings** after logging in.

- Listens on `0.0.0.0:8484` over plain HTTP — front it with a TLS reverse proxy
  for anything public. Override the bind address with `JABALI_SOUNDER_ADDR`.
- Files: config + encryption key in `/etc/jabali-sounder/`, database in
  `/var/lib/jabali-sounder/`, service `jabali-sounder` (`systemctl status
  jabali-sounder`, `journalctl -u jabali-sounder -f`).
- Preset the admin: `JABALI_SOUNDER_ADMIN` / `JABALI_SOUNDER_ADMIN_PASSWORD`.

For desktop use instead, grab a build from [Downloads](#downloads).

## Repository Layout

```text
manager-api/       Go API server, CLI, migrations, repositories, remote clients
manager-ui/        React/Vite/Ant Design admin SPA
docs/              Project documentation
plans/             Historical implementation blueprint and planning notes
config.example.toml
Makefile
```

The project still uses `manager-api` and `manager-ui` as component directory
names. The product, binary, module, UI branding, and package names are
`jabali-sounder`.

## Quick Start

Install frontend dependencies once:

```bash
make ui-install
```

Create a local configuration or use environment variables:

```bash
cp config.example.toml config.local.toml
```

Run database migrations:

```bash
JABALI_SOUNDER_CONFIG=./config.local.toml make migrate-up
```

Create or update the admin user:

```bash
JABALI_SOUNDER_CONFIG=./config.local.toml go run ./manager-api/cmd/server admin set-password -u admin
```

Run the API:

```bash
JABALI_SOUNDER_CONFIG=./config.local.toml make run
```

Build the UI:

```bash
make ui-build
```

For the deployed test server, the UI is currently served at:

```text
http://10.0.3.14:8485/
```

## Core Commands

```bash
make build          # build ./bin/jabali-sounder
make run            # run the API server
make test           # go test -race
make ui-build       # TypeScript + Vite production build
make test-ui        # Vitest
make lint           # golangci-lint
make fmt            # go fmt
make vet            # go vet
make tidy           # go mod tidy
```

UI-specific commands live in `manager-ui/package.json`:

```bash
npm run lint
npm run build
npm test
```

## Configuration

Default config path:

```text
/etc/jabali-sounder/config.toml
```

Important environment variables:

- `JABALI_SOUNDER_CONFIG`
- `JABALI_SOUNDER_ADDR`
- `JABALI_SOUNDER_ENV`
- `JABALI_SOUNDER_DATABASE_DRIVER`
- `JABALI_SOUNDER_DATABASE_URL`
- `JABALI_SOUNDER_SECRET_KEY_FILE`
- `JABALI_SOUNDER_JWT_SECRET` (required outside `development` env)

Legacy `JABALI_MANAGER_*` names are still accepted as compatibility fallbacks
for existing installs.

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Configuration](docs/CONFIGURATION.md)
- [Development](docs/DEVELOPMENT.md)
- [Deployment](docs/DEPLOYMENT.md)
- [Database](docs/DATABASE.md)
- [Desktop Standalone App](docs/DESKTOP.md)
- [Frontend](docs/FRONTEND.md)
- [Operations](docs/OPERATIONS.md)
- [Managed Panel Requirements](docs/MANAGED-PANEL-REQUIREMENTS.md)
- [Security](docs/SECURITY.md)
- [Updating](docs/UPDATING.md)
- [Roadmap](docs/ROADMAP.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

## Security Model

Sounder stores per-server automation token IDs and encrypted token secrets.
Remote requests are HMAC-signed and scoped by the token permissions on the
managed Jabali Panel server. Sounder does not assume shell access to managed
servers.

Token secret encryption uses a local 32-byte key file configured by
`[secrets].key_file`. If the key cannot be loaded, the code has a development
fallback that stores hex-encoded plaintext; do not use that fallback in
production.

Access is gated by RBAC (viewer / operator / owner) with optional TOTP 2FA and
server-side sessions. Failed logins are throttled per-IP and per-account.
Privileged mutations are recorded in an audit log, and read-only API tokens can
be scoped, IP-restricted, and rate-limited. See [Security](docs/SECURITY.md).

## Known External Dependency

The Mail tab calls proposed read-only Jabali Panel automation endpoints:

- `GET /api/v1/automation/mail/mailboxes`
- `GET /api/v1/automation/mail/forwarders`
- `GET /api/v1/automation/mail/domain-forwarders`
- `GET /api/v1/automation/mail/groups`
- `GET /api/v1/automation/mail/autoresponders`

Until managed Panel servers ship these endpoints, Sounder shows per-server
HTTP 404 errors and empty mail tables.
