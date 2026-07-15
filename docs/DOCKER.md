# Docker

Jabali Sounder ships as a single container that serves the API and the SPA on
one port, backed by SQLite on a persistent volume — no external database
required. This is the same headless server binary the `install.sh` one-liner
installs, packaged for containers.

## Quick start (prebuilt image — no source checkout)

Each release publishes an image to GitHub Container Registry. Just run it:

```bash
docker run -d --name jabali-sounder -p 8484:8484 -v sounder-data:/data \
  -e JABALI_SOUNDER_ADMIN_PASSWORD=change-me \
  ghcr.io/shukiv/jabali-sounder:latest
```

Or with Docker Compose — download the compose file, then start it:

```bash
curl -fsSL -O https://raw.githubusercontent.com/shukiv/jabali-sounder/main/docker-compose.yml
JABALI_SOUNDER_ADMIN_PASSWORD=change-me docker compose up -d
```

Open `http://localhost:8484` and log in as `admin` with that password.

Image tags: `ghcr.io/shukiv/jabali-sounder:latest` (newest release) and
`:X.Y.Z` (pinned to a release, e.g. `:0.5.16`).

## Build from source (optional)

To build the image yourself from a checkout instead of pulling it:

```bash
git clone https://github.com/shukiv/jabali-sounder.git
cd jabali-sounder
docker build -t jabali-sounder .
docker run -d --name jabali-sounder -p 8484:8484 -v sounder-data:/data \
  -e JABALI_SOUNDER_ADMIN_PASSWORD=change-me jabali-sounder
```

`make docker-build` / `make docker-run` wrap this with git-derived version
stamping. (The committed `docker-compose.yml` pulls the published image by
default; uncomment its `build:` block to build from source instead.)

## Image layout

Multi-stage build (`Dockerfile`):

1. **`ui`** (`node:22-alpine`) — `npm ci` + `npm run build` → the production SPA.
2. **`build`** (`golang:1.25-alpine`) — stages the SPA into
   `manager-api/cmd/server/dist`, then `CGO_ENABLED=0 go build -tags
   embedui,nomsgpack` → a **statically linked** server (pure-Go SQLite, so it
   needs no libc and runs on musl).
3. **runtime** (`alpine:3.20`) — `ca-certificates` (outbound TLS for update
   checks, webhooks, managed panels) + `openssl` (first-run secret generation) +
   the binary + the entrypoint, running as a non-root user (`uid 10001`).

Final image is ~33 MB.

## First-run provisioning (entrypoint)

`docker-entrypoint.sh` runs on every start and is idempotent. It mirrors what
`install.sh` does on a bare host:

1. **Encryption key** — generates `${DATA}/secrets.key` (32 random bytes) if
   absent. Sealed AES-GCM panel-token secrets depend on it; it must stay stable.
2. **JWT secret** — if `JABALI_SOUNDER_JWT_SECRET` is unset, generates and
   persists `${DATA}/jwt.secret` so existing sessions survive restarts.
3. **Migrations** — `jabali-sounder-server migrate up`.
4. **Admin bootstrap** — if `JABALI_SOUNDER_ADMIN_PASSWORD` is set, creates or
   updates the admin (`JABALI_SOUNDER_ADMIN`, default `admin`). Re-running with a
   new value is the password-reset path.
5. **`exec serve`** — the server becomes PID 1 and receives stop signals.

## Configuration (environment)

The image presets container-friendly defaults; override any of these:

| Variable | Default (image) | Purpose |
|----------|-----------------|---------|
| `JABALI_SOUNDER_ADDR` | `0.0.0.0:8484` | Bind address |
| `JABALI_SOUNDER_ENV` | `production` | `production` requires a JWT secret |
| `JABALI_SOUNDER_DATABASE_DRIVER` | `sqlite` | `sqlite` or `mysql` |
| `JABALI_SOUNDER_DATABASE_URL` | `/data/sounder.db` | SQLite path or MySQL DSN |
| `JABALI_SOUNDER_SECRET_KEY_FILE` | `/data/secrets.key` | Token-encryption key file |
| `JABALI_SOUNDER_JWT_SECRET` | (auto → `/data/jwt.secret`) | Session signing secret |
| `JABALI_SOUNDER_ADMIN` | `admin` | Bootstrap admin username |
| `JABALI_SOUNDER_ADMIN_PASSWORD` | (unset) | Bootstrap admin password |
| `JABALI_SOUNDER_ALLOW_PRIVATE_TARGETS` | (unset) | Allow enrolling LAN/private panels |

Legacy `JABALI_MANAGER_*` names are still accepted. See
[CONFIGURATION.md](CONFIGURATION.md) for the full list.

## Data & backups

Everything stateful lives in the **`/data`** volume:

- `sounder.db` — the SQLite database.
- `secrets.key` — token-encryption key. **Back this up.** Losing it makes sealed
  panel tokens unrecoverable.
- `jwt.secret` — session signing secret.

Back up the volume (e.g. `docker run --rm -v sounder-data:/data -v "$PWD":/out
alpine tar czf /out/sounder-data.tgz -C /data .`).

## MariaDB instead of SQLite

Uncomment the `db` service in `docker-compose.yml` and set on the app service:

```yaml
JABALI_SOUNDER_DATABASE_DRIVER: "mysql"
JABALI_SOUNDER_DATABASE_URL: "sounder:sounder@tcp(db:3306)/sounder?parseTime=true&charset=utf8mb4"
```

## TLS

The server speaks plain HTTP. Front it with a TLS-terminating reverse proxy
(Caddy, nginx, Traefik) for anything public.

## Health check

The `Dockerfile` declares a `HEALTHCHECK` hitting `/health`. **Docker** honours
it natively. **Podman** builds OCI-format images by default and ignores it —
build with `podman build --format docker` for parity, or rely on an external
probe / compose healthcheck.

## Version stamping

The build accepts `--build-arg VERSION=… COMMIT=… DATE=…`, which land in
`/health` and `GET /api/v1/version`:

```bash
docker build -t jabali-sounder \
  --build-arg VERSION="$(git describe --tags --always)" \
  --build-arg COMMIT="$(git rev-parse --short HEAD)" \
  --build-arg DATE="$(date -u +%Y-%m-%d)" .
```

`make docker-build` fills these in from git automatically.

## Rootless Podman notes

Podman is a drop-in for the commands above. In restricted/nested environments
(building inside another container, some CI runners) rootless container creation
can trip on namespace limits. Workarounds that have been needed:

- **Base images unqualified** → set
  `unqualified-search-registries = ["docker.io"]` in
  `~/.config/containers/registries.conf`.
- **`crun: mount proc to proc: Operation not permitted`** at a `RUN` step →
  build with `--isolation=chroot` (or `BUILDAH_ISOLATION=chroot`).
- **`crun: ... ping_group_range: Read-only file system`** → clear default
  sysctls: `~/.config/containers/containers.conf` → `[containers]
  default_sysctls = []`.
- **`crun: create keyring … Operation not permitted`** → `[containers] keyring =
  false` in the same file.

These are host-sandbox restrictions, not image issues — on a normal Docker or
Podman host none are needed.

## Verified

The image was built and exercised end-to-end (server binary + entrypoint flow):
the entrypoint provisions `secrets.key` / `jwt.secret` / `sounder.db`, migrations
apply, the admin bootstraps from the environment, `GET /health` returns the
stamped version, `POST /api/v1/auth/login` issues a JWT (bad credentials →
`401`), and `/` serves the embedded SPA.

## See also

- [Deployment & reconciliation runbook](DEPLOYMENT.md)
- [Configuration](CONFIGURATION.md)
- [Security](SECURITY.md)
