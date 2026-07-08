# Jabali Sounder — V1 Construction Blueprint

**Objective:** Build a central control plane that manages multiple Jabali Panel
servers from one app (Docker Compose or standalone VM), talking to each managed
server through its existing panel API + scoped automation tokens.

**Decisions locked (user, 2026-07-08):**
- **v1 = read-only + one ADR-gated mutating exception.** Enroll + health
  dashboard + global search + unified logs, **plus delegated login /
  jump-to-server** (the plan's acceptance criteria list it as a v1 capability
  and it's core to the multi-server UX). Delegated login mints a short-lived
  panel session on the target server — a mutating auth action — so it ships
  behind its own ADR (threat model, TTL, one-time use, scope gating). Truly
  mutating *server-state* actions (mass update, cross-server migration) are v2
  — separate blueprint.
- **Manager dials out.** Manager holds each server's URL + scoped automation
  token and calls the panel API outbound. No call-home agent, no SSH.
- **Mirror the jabali2 stack.** Go 1.25 / Gin / GORM + MariaDB / go-redis /
  golang-migrate / cobra / slog; React 19 / Vite 6 / Ant Design 6 / TanStack
  Query / axios / react-router 7. Copy patterns verbatim from
  `/home/shuki/projects/jabali2` (see `AGENTS.md`).

**Status:** Pre-implementation. Repo is empty except `AGENTS.md` + this plan.

---

## CRITICAL FINDING — jabali2 automation API surface gap

Verified from `jabali2/panel-api/internal/api/automation.go` +
`middleware/automation_hmac.go` + `models/automation_token.go`. The manager
authenticates to each managed server with **HMAC request signing** (not Bearer):

```
Authorization: Jabali-HMAC kid=<token-id>, ts=<unix>, sig=<hex>
sig = hex(HMAC_SHA256(secret, METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(BODY))))
```

where `RequestURI` is the **full request URI including the raw query string**
(`c.Request.URL.RequestURI()` in jabali2 — path + `?` + rawquery). Signing
only the path will 401 any request carrying query params (log filters,
pagination, search). For GET with no body, the body hash is `sha256("")`.
5-minute clock-skew window; Redis SETNX replay defense (fail-closed if Redis
down). The manager's remote client MUST sign `url.RequestURI()` verbatim. The **only** automation endpoints that exist today (all GET, all under
`/api/v1/automation/`):

| Endpoint | Scope | Returns |
|---|---|---|
| `GET /health` | (none) | `{status:"ok", version}` |
| `GET /api/v1/automation/status` | `read:status` | `{healthy, time}` |
| `GET /api/v1/automation/domains` | `read:domains` | `{data:[{id,name,user_id,is_enabled}], total}` |
| `GET /api/v1/automation/users` | `read:users` | `{data:[{id,email,username,package_id,is_admin}], total}` |
| `GET /api/v1/automation/applications` | `read:applications` | `{data:[{id,app_type,domain_id,status}], total}` |

Allowed scopes (closed set in `AllowedAutomationScopes`): `read:*`,
`read:domains`, `read:users`, `read:applications`, `read:status`. There is no
denylist — `write:*` is simply absent from the allowed set, so
`IsAllowedAutomationScope("write:*")` returns false and the mint endpoint
rejects it.

**Gap:** v1's goals require endpoints that do **not** exist yet on the managed
servers:
- **Mailboxes** (global search lists mailboxes + owning server) — no
  `read:mailboxes` endpoint.
- **Server metrics** (dashboard shows disk/memory/load/update status) —
  `/automation/status` returns only `{healthy, time}`; the rich
  `/admin/server-status` envelope is admin-session-gated, not automation-gated.
- **Logs** (unified log search) — no `read:logs` automation endpoint.
- **Delegated login / jump-to-server** — no automation endpoint mints a
  short-lived panel session.

**Consequence:** this blueprint includes jabali2-side PRs (Steps 6, 9, 11) that
extend the automation API. The manager and jabali2 evolve together. Steps are
ordered so the manager is useful against the *existing* surface as early as
possible, and the jabali2 extensions unlock the rest.

---

## Architecture

```
jabali-sounder/
  manager-api/                 Go + Gin HTTP server (the control plane)
    cmd/server/                cobra entry — serve.go wires Deps into app.NewWithDeps
    internal/
      app/                     route mount (NewWithDeps)
      config/                  Viper + toml, defaults in config.Defaults()
      db/                      migrations/ + GORM open helper (MariaDB)
      api/                     route families, one file per resource
        servers.go             enroll/edit/disable/remove managed servers
        dashboard.go           aggregate health across servers
        search.go              global cross-server search
        logs.go                unified log proxy/search
        audit.go               manager-side audit log
      repository/              one file per aggregate; interface + GORM impl
        server_repository.go
        heartbeat_repository.go
        inventory_index.go
        audit_log_repository.go
      remote/                  outbound client to managed servers
        client.go              HMAC-signed http.RoundTripper
        scopes.go              scope constants matching jabali2
        status.go              /health + /automation/status
        inventory.go           /automation/{domains,users,applications,mailboxes}
        metrics.go             /automation/server-status (new jabali2 endpoint)
        logs.go                /automation/logs (new jabali2 endpoint)
      reconciler/              heartbeat + inventory sync loop
      middleware/              auth, ratelimit, audit
      models/                  GORM structs
      ids/                     NewULID()
  manager-ui/                  React + Vite + AntD SPA
    src/apiClient.ts           axios instance + error envelope
    src/shells/                AdminLayout
    src/components/            SearchableTable, ServerCard, HealthBadge
    src/admin/<resource>/      one folder per feature
  agentwire/                   (none — manager has no agent; placeholder for shared types)
  plans/                       this blueprint + runbooks
  docs/                        ADRs, CONVENTIONS.md (copied + adapted from jabali2)
```

**No agent daemon.** Unlike jabali2 (panel + root agent over Unix socket), the
manager is a single binary + SPA. All privileged work happens on managed
servers via their existing agent; the manager only orchestrates outbound API
calls.

**Auth model (manager-internal):** the manager itself needs its own admin auth.
v1: reuse jabali2's Kratos integration pattern (manager is a Kratos relying
party) OR a simpler local admin + JWT. **Open: decide in Step 1b.** Recommend
Kratos to stay consistent with jabali2 and inherit SSO.

---

## Dependency graph

```
Step 0   (scaffold) ───────────────────────────────────┐
Step 1a  (config + DB + migrations + audit repo + secret key) ─┐
Step 1b  (internal auth: Kratos/JWT decision + impl) ◀── 1a     │
                                                                │
Step 2   (server enrollment: model+repo+API+rotate+credential_status) ◀── 1a,1b
Step 3   (remote HMAC client) ◀── 1a (secret key) ──────────────┘
                                                │
              ┌─────────────────────────────────┤
              ▼                                 ▼
Step 4   (health sync loop + heartbeat)    Step 5 (enrollment UI)
              │                                 │
              ▼                                 │
Step 6   (jabali2: /automation/server-status + read:metrics)  [jabali2 PR]
              │
              ▼
Step 7   (dashboard UI: metrics + update status)
              │
Step 8   (inventory ingest: domains/users/apps) ◀── depends on Step 3 (transitively Step 2)
              │
Step 9   (jabali2: /automation/mailboxes + read:mailboxes)  [jabali2 PR]
              │
              ▼
Step 10  (global search UI + owning-server links)
              │
Step 11a (jabali2: /automation/logs + read:logs)  [jabali2 PR]
              │
              ▼
Step 12a (unified logs UI)
              │
Step 11b (jabali2: /automation/delegated-login + delegate:login + ADR)  [jabali2 PR]
              │
              ▼
Step 12b (jump-to-server UI + manager-side audit)
              │
              ▼
Step 13  (packaging: Docker Compose + VM install + backup/restore)
```

**Parallelism:** Steps 4 and 5 run in parallel after 2+3 (backend vs frontend,
no shared files). Steps 6, 9, 11a, 11b are jabali2-side and can be *drafted* in
parallel (different endpoints) but land serially to keep jabali2 CI stable.
Step 11b is split from 11a because delegated login is a distinct security
surface with its own ADR and reviewer. Steps 12a and 12b are serial (logs UI
before jump UI).

---

## Steps

### Step 0 — Scaffold repo skeleton

**Context brief:** Repo is empty. Mirror jabali2's layout so conventions,
Makefile targets, lint, and CI transfer verbatim. Read `AGENTS.md` (already
written) and `/home/shuki/projects/jabali2/{Makefile,.golangci.yml,.editorconfig,panel-ui/package.json}`.

**Tasks:**
- `git init`, `.gitignore` (copy from jabali2, drop panel-specific entries).
- `go.mod` — `module git.jabali-panel.com/shukivaknin/jabali-sounder`, `go 1.25`.
- `manager-api/cmd/server/main.go` + `serve.go` — cobra `serve` command, wire
  `app.NewWithDeps(Deps{})` (empty for now).
- `manager-api/internal/app/app.go` — `NewWithDeps(g *gin.Engine, deps Deps)`,
  mount `/health` returning `{status:"ok", version}`.
- `manager-api/internal/config/config.go` — Viper + toml; `config.Defaults()`
  with `[server] addr`, `[log]`, `[database] url`, `[redis] url`.
- `manager-api/internal/db/db.go` — GORM open helper (MariaDB driver).
- `manager-api/internal/ids/ids.go` — `NewULID()` (copy from jabali2).
- `config.example.toml`, `.env.example`, `.envrc` (adapt from jabali2).
- `Makefile` — copy jabali2's, change `API_PKG` to `./manager-api/...`, drop
  agent/wire targets, keep all test/lint/ui targets.
- `.golangci.yml`, `.editorconfig` — copy verbatim.
- `manager-ui/` — `npm create vite`, copy jabali2's `panel-ui/package.json`
  deps (rename to `jabali-sounder-ui`), `App.tsx` with `/health` ping, AntD
  ConfigProvider, react-router stub.
- `docs/CONVENTIONS.md` — copy jabali2's, adapt repo layout section.
- Forgejo/CI workflow — copy jabali2's `ci.yml`, adjust paths.

**Verification:**
```
make build && make run &   # /health returns 200
make fmt && make vet && make lint
make test                  # empty suite, green
make ui-install && make ui-build && make test-ui
```

**Exit criteria:** `make test-all` green; `/health` responds; `make lint`
clean; SPA builds.

---

### Step 1a — Config + DB + migrations + audit repo + secret key

**Context brief:** Depends on Step 0. Stand up the manager's own database
(MariaDB) with golang-migrate, the GORM open helper, the manager's secret-key
loader (used to encrypt stored automation-token secrets), and the audit-log
repository. Other repos (servers, heartbeats, inventory) are owned by the
steps that build their API (Steps 2, 4, 8) — this step only owns `audit_log`.

**Tasks:**
- `manager-api/internal/db/migrations/000001_servers.up.sql` —
  `servers` table: `id CHAR(26) PK`, `name`, `base_url`, `token_id CHAR(26)`,
  `token_secret_enc BLOB`, `scopes JSON`, `version`, `capabilities JSON`,
  `health_url`, `status ENUM('active','disabled','unreachable')`,
  `credential_status ENUM('valid','invalid','unknown') NOT NULL DEFAULT 'unknown'`,
  `last_heartbeat_at`, `created_at`, `updated_at`, `disabled_at NULL`.
  (The `credential_status` column tracks whether the stored token still
  authenticates; the health loop flips it on 401.)
- `000002_audit_log.up.sql` — `audit_log`: `id CHAR(26)`, `actor`, `action`,
  `target_server_id`, `detail JSON`, `ip`, `request_id`, `created_at`.
- `000003_heartbeats.up.sql` — `heartbeats`: `id`, `server_id`, `healthy BOOL`,
  `version`, `details JSON`, `checked_at`.
- `000004_inventory_index.up.sql` — `inventory_index`: `id`, `server_id`,
  `kind ENUM('domain','user','mailbox','application')`, `remote_id`,
  `payload JSON`, `indexed_at`.
- Each `*.up.sql` has a matching `*.down.sql` (test `down` on every migration PR).
- GORM models in `internal/models/` for `audit_log` only (this step). Other
  models land with their steps.
- `repository/audit_log_repository.go` — interface + impl + sqlmock `_test.go`.
- **Secret key:** `[secrets] key_file` (default `/etc/jabali-sounder/secrets.key`)
  or `JABALI_SOUNDER_SECRET_KEY` env. Load at startup into a `*secrets.Key`
  (mirror jabali2's `ssokey.Key` Seal/Open). Inject into `app.Deps`. Without
  this key, stored automation-token secrets cannot be encrypted (Step 2) or
  decrypted (Step 3). Document in `docs/adr/0002-manager-secret-key.md`:
  single point of failure, rotation, and **custody** — the key backup MUST be
  stored separately from the DB backup so a DB leak alone can't decrypt tokens.
- `Makefile` — add `migrate-up` / `migrate-down` targets using golang-migrate.
- Config: `[database] url`, `[redis] url`, `[secrets] key_file`, `[health]`,
  `[inventory]`.

**Verification:**
```
make migrate-up    # against a fresh MariaDB (JABALI_SOUNDER_TEST_DATABASE_URL)
make migrate-down  # rolls back cleanly
make test          # audit_log repo sqlmock tests green
make test-coverage
```

**Exit criteria:** migrations up + down both clean against a real MariaDB;
audit repo tested; secret key loads at startup and is wired into `Deps`.

---

### Step 1b — Internal admin auth

**Context brief:** Depends on Step 1a. The manager itself needs admin auth
before any protected route can ship. Split out from Step 1a so the auth
decision is deliberate and reviewable on its own.

**Tasks:**
- **Decision + impl:** recommend Kratos (consistent with jabali2, SSO reuse).
  If Kratos: add `[auth.kratos]` config block, Kratos client,
  `middleware.RequireKratosSession()` (mirror jabali2). If local JWT: admin
  table + JWT mint/verify + `middleware.RequireAdmin()`. Document the choice in
  `docs/adr/0001-internal-auth.md`.
- `middleware/audit.go` — record every mutating manager action to `audit_log`
  (the manager's own audit, separate from each managed server's audit).
- A single protected test route `GET /api/v1/admin/me` returning the verified
  admin identity, to prove the middleware chain works.

**Verification:**
```
make test  # auth middleware tests; /me returns 401 without session, 200 with
```

**Exit criteria:** protected route gated; auth middleware tested both
authenticated and unauthenticated; audit middleware records a mutation; ADR
0001 written.

---

### Step 2 — Server enrollment: model + repository + API

**Context brief:** Depends on Step 1a + 1b. Implement the core "enroll a
managed server" CRUD plus token rotation. This is milestone 1 of the plan.
Follow jabali2's route family pattern: `RegisterServerRoutes(g *gin.RouterGroup, cfg ServerHandlerConfig)`,
panic on nil repo, nil-check optional deps. List envelope is
`{data,total,page,page_size}` — NOT `{items,total}`.

**Tasks:**
- `api/servers.go`:
  - `POST /api/v1/admin/servers` — enroll: validate `base_url` reachable
    (`GET /health` via the remote client), store `token_id` + encrypted
    `token_secret_enc` (via the `secrets.Key` from Step 1a), store requested
    scopes. Return the server record (never the plaintext secret — plaintext
    shown once on enroll, like jabali2's token mint). Set
    `credential_status='unknown'`; the health loop validates it.
  - `GET /api/v1/admin/servers` — list (paginated envelope).
  - `GET /api/v1/admin/servers/:id` — detail (includes last heartbeat +
    `credential_status`).
  - `PATCH /api/v1/admin/servers/:id` — edit name/base_url/scopes; re-probe
    health on URL change.
  - `DELETE /api/v1/admin/servers/:id` — disable (soft: `disabled_at`), do not
    hard-delete (preserve audit references). Confirm with `?confirm=1`.
  - `POST /api/v1/admin/servers/:id/rotate-token` — replace `token_id` +
    `token_secret_enc` on an enrolled server; returns the new plaintext
    secret once. The old token remains valid on the managed server until an
    operator revokes it there (document in ADR 0002 — remote revocation is a
    future jabali2 endpoint; for now rotation is manager-side only).
- Validation: 422 for domain rules (duplicate name, invalid URL), 400 for
  malformed JSON (jabali2 convention).
- Audit every action via `middleware/audit.go`.
- `api/servers_test.go` — table-driven, sqlmock for repo, httptest for
  handlers. Cover enroll success, enroll with unreachable URL (422), duplicate
  name (422), disable, re-probe, rotate-token (new secret returned, old
  overwritten).

**Verification:**
```
make test           # servers_test.go green
make test-coverage  # api/servers.go covered
make lint
```

**Exit criteria:** enroll/edit/disable/remove/rotate all work end-to-end
against a mocked jabali2 `/health`; list returns the `{data,total,...}`
envelope; audit log records each action; `credential_status` starts `unknown`.

---

### Step 3 — Remote HMAC client

**Context brief:** Depends on Step 1a (the secret key). The remote client
module itself only needs crypto + the key; the per-server factory (building a
client from stored creds) additionally needs the server repo from Step 2.
Build the outbound client that signs every request with the exact scheme a
managed jabali2 server expects. **Get the signature string format byte-exact**
— a wrong separator, field order, or signing path-only instead of
path+query means every remote call 401s.

**Tasks:**
- `remote/client.go`:
  - `type Client struct { baseURL *url.URL; tokenID, secret string; http *http.Client }`
  - `func (c *Client) Do(ctx, method, pathAndQuery string, body []byte) (*http.Response, error)`
    signs and sends.
  - Signature: `sig = hex(HMAC_SHA256(secret, METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(body))))`
    where `RequestURI` is path + `?` + raw query (use `u.RequestURI()` on the
    constructed `*url.URL`). For GET with no body, body hash is `sha256("")`.
    `ts` is unix seconds string. Header: `Authorization: Jabali-HMAC kid=<id>, ts=<ts>, sig=<sig>`.
  - 5-min skew: client clock vs server. Use `time.Now().Unix()`. (Replay
    defense is server-side; client just must not reuse a ts within the window.)
  - Timeout: 10s default, per-call override.
- `remote/scopes.go` — constants matching jabali2's `AllowedAutomationScopes`:
  `ReadAll`, `ReadDomains`, `ReadUsers`, `ReadApplications`, `ReadStatus`.
- `remote/status.go` — `Health(ctx) (*HealthResp, error)` → `GET /health`;
  `AutomationStatus(ctx) (*StatusResp, error)` → `GET /api/v1/automation/status`.
- `remote/inventory.go` — `Domains(ctx)`, `Users(ctx)`, `Applications(ctx)` →
  each returns the `{data,total}` envelope, decoded into typed slices. Handle
  503 (feature off) vs 401 (bad token) vs 200.
- `remote/client_test.go` — golden signature tests:
  1. Fixed secret + fixed method/path/ts/body → assert exact hex sig matches a
     precomputed vector. **The single most important test in the manager.**
  2. **Query-param case**: `GET /api/v1/automation/logs?service=nginx&since=…`
     — assert the signed string includes `?service=nginx&since=…`. This catches
     the path-only signing regression that would 401 every filtered request.
  3. GET no-body sig; 4. large-body cap (1 MiB — mirror jabali2's `autoMaxBody`).
- `remote/verifier_mock_test.go` — a tiny in-process HTTP server that runs the
  jabali2 HMAC verification logic (copy the relevant constant-time compare +
  replay-skip-for-test) against a fixed token, so the client's round-trip can
  be tested end-to-end **without a live jabali2 instance**. The golden vector +
  this mock together are the gate.
- Token secret storage: **default to decrypt-per-call** (security over
  functionality — caching plaintext secrets in memory widens the compromise
  blast radius). If load testing later shows per-call decryption is too slow,
  require an ADR (per the loosening-needs-ADR rule) before adding an in-memory
  cache with TTL + mutex.

**Verification:**
```
make test  # golden vectors pass; query-param case passes; mock-verifier round-trip 200
# optional live check: point at a real jabali2 dev instance with a minted token,
# confirm GET /api/v1/automation/status returns 200
```

**Exit criteria:** golden signature test green (incl. query-param case); mock
verifier round-trip 200; live call to a jabali2 dev instance succeeds if
available; 401 path tested.

---

### Step 4 — Health sync loop + heartbeat store (PARALLEL with Step 5)

**Context brief:** Depends on Step 3. Background goroutine that, on a
configurable interval (default 30s), calls `GET /health` +
`GET /api/v1/automation/status` on every active enrolled server, stores a
`heartbeat` row, and updates `servers.status` (`active`/`unreachable`) and
`servers.credential_status` (`valid` on 200, `invalid` on 401, `unknown`
on network error). Use `errgroup` with a concurrency cap (mirror jabali2's
`server_status.go` `maxInFlight=8`, `subCallTimeout=5s`). A slow/unreachable
server must not block the loop.

**Tasks:**
- `reconciler/health_loop.go` — `Run(ctx)` ticks, fans out per-server, records
  heartbeat, updates status + credential_status. Failure-isolated: one
  server's error is logged + recorded, never panics the loop.
- `repository/heartbeat_repository.go` — `Record(ctx, hb)`, `Latest(ctx, serverID)`,
  `Recent(ctx, serverID, n)`.
- `repository/server_repository.go` — `UpdateStatus(ctx, id, status, credentialStatus)`.
- `api/dashboard.go` — `GET /api/v1/admin/dashboard` aggregates latest heartbeat
  per server: `{data:[{server_id, name, status, healthy, credential_status, version, last_heartbeat_at}], total}`.
- Config: `[health] interval = "30s"`, `timeout = "5s"`, `max_in_flight = 8`.
- **Audit policy for background loops:** periodic read-only loops do NOT emit
  a per-call audit row (would flood the log). Instead emit one summary row per
  run (`action=health_sync`, `detail={checked, healthy, unreachable, auth_failed}`)
  and emit a `credential_status_changed` audit row only on a transition
  (`valid`→`invalid` or vice versa). Document in
  `docs/adr/0003-background-loop-audit.md`.
- Tests: mock the remote client, simulate 1 unreachable + 1 valid + 1
  bad-token (401) of 3 servers, assert loop completes, statuses flip, and
  `credential_status` is `invalid` for the 401 server only.

**Verification:**
```
make test  # health_loop_test.go: 3 servers, mixed states, loop completes < 2s
```

**Exit criteria:** loop runs, isolates per-server failures, flips
`credential_status` on 401, dashboard endpoint returns aggregate health +
credential status.

---

### Step 5 — Enrollment UI (PARALLEL with Step 4)

**Context brief:** Depends on Step 2 (the enrollment API). React + AntD page
that lets an admin enroll/list/edit/disable servers. Mirror jabali2's
`panel-ui` patterns: `SearchableTable` for the list, `Drawer` for create+edit,
`@icons` shim, `useListQuery`/`useMutation` hooks reading `.data` from the
list envelope.

**Tasks:**
- `manager-ui/src/admin/servers/ServerList.tsx` — SearchableTable with columns
  name, base_url, status badge, version, last heartbeat. Actions: edit
  (Drawer), disable, remove (confirm modal).
- `manager-ui/src/admin/servers/ServerForm.tsx` — Drawer form: name, base_url,
  token_id, token_secret (one-time entry), scopes (multi-select of the 5
  allowed scopes). On submit → `POST /api/v1/admin/servers`.
- `manager-ui/src/hooks/useServers.ts` — TanStack Query hooks.
- `manager-ui/src/apiClient.ts` — axios instance, base `/api/v1`, error
  envelope handling (mirror jabali2).
- `manager-ui/src/App.tsx` — react-router with `/admin/servers` route + AntD
  layout shell.
- Playwright E2E: enroll a server against a mocked API, see it in the list,
  disable it.

**Verification:**
```
make ui-build && make test-ui && make test-e2e
```

**Exit criteria:** admin can enroll a server through the UI, see it in the
list, disable it; E2E green against `dist/`.

---

### Step 6 — jabali2: extend automation API with server metrics  [jabali2 PR]

**Context brief:** This is a **jabali2** PR, not a jabali-sounder PR. The
existing `/automation/status` returns only `{healthy, time}`. The manager's
health dashboard needs disk/memory/load/update status. jabali2 already has a
rich `/admin/server-status` envelope (gated behind admin session) — expose a
thinned, automation-gated version. Add `read:metrics` to
`AllowedAutomationScopes`.

**Tasks (in `/home/shuki/projects/jabali2`):**
- `panel-api/internal/api/automation.go` — add `GET /api/v1/automation/server-status`
  with scope `read:metrics`. Return a thinned envelope: `{server_name, panel_version,
  agent_version, disk{used,total}, memory{used,total}, load{1,5,15}, update_status,
  security_status, as_of}`. Reuse the fan-out logic from `server_status.go` but
  drop fields that leak topology (listen IPs, doc roots).
- `panel-api/internal/models/automation_token.go` — add `read:metrics` to
  `AllowedAutomationScopes` (one scope only; do not add a redundant
  `read:server-status`).
- `panel-api/internal/middleware/automation_hmac.go` — no change (already
  generic).
- Tests: extend `automation_test.go` with the new endpoint + scope check.
- Update `docs/CONVENTIONS.md` automation API table.

**Verification (jabali2):**
```
make test && make lint   # in jabali2
# mint a token with read:metrics, curl /automation/server-status with HMAC
```

**Exit criteria:** jabali2 CI green; new endpoint returns metrics with a
`read:metrics`-scoped HMAC token; 403 without the scope.

---

### Step 7 — Manager dashboard UI: metrics + update status

**Context brief:** Depends on Steps 4 + 6. Extend the dashboard to pull
metrics from the new jabali2 endpoint and render AntD cards per server with
disk/memory/load gauges, version, update-status badge, security-status badge.

**Tasks:**
- `remote/metrics.go` — `ServerStatus(ctx) (*MetricsResp, error)` → `GET /api/v1/automation/server-status`.
- Extend `reconciler/health_loop.go` to also fetch metrics (best-effort; if a
  server is on an older jabali2 without the endpoint, 404 → omit metrics, keep
  basic health).
- `manager-ui/src/admin/dashboard/Dashboard.tsx` — grid of `ServerCard`s.
- `manager-ui/src/admin/dashboard/ServerCard.tsx` — AntD Card: name, status
  badge, version, disk/memory progress bars, load, update + security badges,
  last-checked time. Click → server detail.
- `api/dashboard.go` — extend the aggregate response with metrics.

**Verification:**
```
make test && make ui-build && make test-e2e
```

**Exit criteria:** dashboard renders live metrics for enrolled servers on a
jabali2 build that has Step 6; gracefully degrades for servers that don't.

---

### Step 8 — Inventory ingest: domains / users / applications

**Context brief:** Depends on Step 3. Periodic (configurable, default 5m) pull
of `GET /api/v1/automation/{domains,users,applications}` per active server,
upsert into `inventory_index`. This is the data behind global search
(milestone 5). Store `server_id` so every indexed object knows its owning
server.

**Tasks:**
- `reconciler/inventory_loop.go` — per-server, per-kind fetch + upsert. Rate
  limited; failure-isolated per server. Store `indexed_at`.
- **Audit policy:** same summary-level approach as Step 4 — one
  `action=inventory_sync` row per run with counts, plus a
  `credential_status_changed` row on 401 transition (per ADR 0003).
- `repository/inventory_index.go` — `Upsert(ctx, serverID, kind, remoteID, payload)`,
  `Search(ctx, query, kind?, serverID?) ([]IndexEntry, total, error)`. Search
  over a JSON payload field (MariaDB JSON functions) or a generated text column.
- `api/search.go` — `GET /api/v1/admin/search?q=&kind=&server_id=` returns
  `{data:[{kind, server_id, server_name, remote_id, payload}], total, page, page_size}`.
- Tests: ingest 3 domains from 2 servers, search "example", assert results
  carry owning server.

**Verification:**
```
make test  # inventory_loop_test.go + search_test.go green
```

**Exit criteria:** search across indexed domains/users/applications returns
results tagged with the owning jabali server.

---

### Step 9 — jabali2: extend automation API with mailboxes  [jabali2 PR]

**Context brief:** jabali2 PR. Global search must list mailboxes + owning
server. No `read:mailboxes` endpoint exists. Add one returning a thinned
mailbox list (address, owner user, domain, enabled) — no secret/auth fields.

**Tasks (in jabali2):**
- `automation.go` — `GET /api/v1/automation/mailboxes` scope `read:mailboxes`.
  Thin fields: `{id, address, local_part, domain_name, user_id, is_enabled, quota_used, quota_limit}`.
- `AllowedAutomationScopes` — add `read:mailboxes`.
- Tests + CONVENTIONS.md update.

**Verification (jabali2):** `make test && make lint`; curl with `read:mailboxes` token.

**Exit criteria:** endpoint returns thinned mailboxes; 403 without scope.

---

### Step 10 — Global search UI + owning-server links

**Context brief:** Depends on Steps 8 + 9. AntD search page: kind filter
(all/domain/user/mailbox/application), server filter, free-text. Results table
shows the object + a link/"jump" affordance to the owning server. For v1 the
"jump" is a link to the server's panel URL (full delegated login is Step 12b).

**Tasks:**
- Extend `remote/inventory.go` — `Mailboxes(ctx)`.
- Extend `reconciler/inventory_loop.go` to ingest mailboxes when the server
  exposes the endpoint (404 → skip, log at debug).
- `manager-ui/src/admin/search/SearchPage.tsx` — AntD Input.Search + Select
  filters + SearchableTable. Columns: kind, name/identifier, owning server
  (link), last synced.
- `api/search.go` — already built in Step 8; add mailbox kind.

**Verification:**
```
make test && make ui-build && make test-e2e
```

**Exit criteria:** admin searches "example" and sees matching domains, users,
mailboxes across all enrolled servers, each tagged with its owning server.

---

### Step 11a — jabali2: automation logs endpoint  [jabali2 PR]

**Context brief:** jabali2 PR. Unified logs need a `read:logs` automation
endpoint. Security-sensitive (log content can leak data) but read-only —
follow jabali2's "security over functionality" rule and thin the fields.

**Tasks (in `/home/shuki/projects/jabali2`):**
- `automation.go` — `GET /api/v1/automation/logs` scope `read:logs`, params
  `service`, `severity`, `since`, `until`, `user_id`, `domain_id`, `q`.
  Proxy to the existing log stream infra (mirror `websocket_logs.go` read
  path, HTTP-paginated not WS for the automation surface). Return the full
  list envelope `{data, total, page, page_size}`.
- `AllowedAutomationScopes` — add `read:logs`.
- Tests: endpoint + scope check (403 without `read:logs`).

**Verification (jabali2):** `make test && make lint`; curl with `read:logs` token.

**Exit criteria:** logs endpoint returns filtered, paginated logs with
`read:logs`; 403 without the scope.

---

### Step 12a — Unified logs UI

**Context brief:** Depends on Steps 10 + 11a. Unified log search across
servers (filters: server, service, severity, time range, user/domain).

**Tasks:**
- `remote/logs.go` — `Logs(ctx, filter)` → `GET /api/v1/automation/logs`.
- `api/logs.go` — `GET /api/v1/admin/logs` queries across servers, merges by
  timestamp, returns the full list envelope `{data, total, page, page_size}`
  (paginated — logs are inherently large; do NOT return an unpaged blob).
- `manager-ui/src/admin/logs/LogsPage.tsx` — AntD Table + filters + server
  column.

**Verification:**
```
make test && make ui-build && make test-e2e
```

**Exit criteria:** admin searches logs across 2+ servers from one UI;
response is paginated.

---

### Step 11b — jabali2: delegated login endpoint  [jabali2 PR]

**Context brief:** jabali2 PR, **separate from 11a** because it's a distinct
mutating security surface with its own ADR and reviewer. Delegated login
mints a short-lived panel session from an automation-scoped request. This is
the one ADR-gated mutating exception in v1 (see decisions block).

**Tasks (in `/home/shuki/projects/jabali2`):**
- `automation.go` — `POST /api/v1/automation/delegated-login` scope
  `delegate:login`. Body `{target_user_id, redirect_path?}` —
  **`target_user_id` is REQUIRED** (no optional; omitting it must 422, never
  mint an elevated/admin session). The minted session assumes exactly that
  user's identity. Returns a short-lived (≤60s) one-time URL that logs the
  target in and redirects. The URL is single-use (redeem marks it consumed).
- Audit on the jabali2 side: actor = the automation token id, target =
  `target_user_id`, action = `delegated_login`, plus the source IP the
  delegated-login request came from.
- `AllowedAutomationScopes` — add `delegate:login`.
- **Write `docs/adr/00XX-automation-delegated-login.md`** covering: threat
  model, why `target_user_id` is required (privilege-escalation prevention),
  token TTL + one-time use, scope gating, audit requirements, and the
  constraint that the manager admin may only request targets they're
  authorized to impersonate (enforced manager-side, see Step 12b).
- Tests: delegated-login mints a one-time URL with `delegate:login`; 422 when
  `target_user_id` omitted; 403 without scope; URL is single-use (second
  redeem 410); both mint + redeem audited.

**Verification (jabali2):** `make test && make lint`; ADR linked from
CONVENTIONS.md.

**Exit criteria:** delegated-login mints a one-time URL for a required
`target_user_id`; 422 if omitted; single-use; audited on jabali2 side.

---

### Step 12b — Jump-to-server UI + manager-side audit

**Context brief:** Depends on Steps 12a + 11b. The "jump to server" button on
every search result mints a delegated-login URL and opens the target panel in
a new tab, authenticated. The manager enforces that the requesting admin is
authorized to impersonate the target user (per the ADR from Step 11b).

**Tasks:**
- `remote/delegated_login.go` — `DelegatedLogin(ctx, targetUserID, path)` →
  `POST /api/v1/automation/delegated-login`. Returns the one-time URL.
- `api/jump.go` — `POST /api/v1/admin/servers/:server_id/jump` body
  `{target_user_id, redirect_path?}`. Manager-side authorization check: is
  this admin allowed to impersonate `target_user_id` on that server? (v1
  policy: any manager admin may jump to any user on any enrolled server —
  document this as the v1 policy in ADR 0004; tighten in v2 if needed.) Then
  calls the remote delegated-login, returns the URL.
- **Manager-side audit:** every jump is recorded in `audit_log` with actor,
  target server, target user, timestamp, request_id, and the redirect path.
  This is in addition to the jabali2-side audit from Step 11b — both sides
  record it.
- `manager-ui/src/admin/search/*` — "Open in panel" button per result → calls
  the jump endpoint → `window.open(url)`. Confirm modal showing the target
  server + user before minting.
- Tests: jump endpoint records audit row; 422 if `target_user_id` missing;
  remote 403 propagated.

**Verification:**
```
make test && make ui-build && make test-e2e
```

**Exit criteria:** admin clicks "Open in panel" on a user, confirms, and
lands authenticated in the target jabali panel as that user; both the manager
and the jabali2 server record an audit entry.

---

### Step 13 — Packaging: Docker Compose + VM install + backup/restore

**Context brief:** Depends on all prior. Ship the manager as Docker Compose
(manager-api + manager-ui static + MariaDB + Redis) and a standalone VM
install script (mirror jabali2's `install.sh` shape, vastly smaller). Document
backup/restore of the manager's own DB (the manager is stateful — its
inventory index + audit log + enrolled-server creds are the crown jewels).

**Tasks:**
- `docker-compose.yml` — services: `manager-api`, `manager-ui` (nginx serving
  `dist/`), `mariadb`, `redis`. Volumes for DB + Redis. `.env` for secrets.
- `Dockerfile` — multi-stage: Go build + Vite build → final image with binary
  + embedded SPA (mirror jabali2's single-binary-serves-SPA pattern).
- `install/install.sh` — standalone VM install: install MariaDB + Redis, run
  migrations, create systemd unit, issue self-signed cert / LE. Much thinner
  than jabali2's (no tenant isolation, no agent, no mail).
- `docs/runbooks/backup-restore.md` — `mariadb-dump` of the manager DB +
  `redis-cli SAVE` + the `ssokey` key file (without it, stored server tokens
  are unrecoverable). Restore procedure.
- `docs/runbooks/enroll-first-server.md` — operator runbook: mint a token on a
  jabali2 server with the right scopes, enroll it in the manager.
- `docs/adr/0002-manager-secret-key.md` — the manager's token-encryption key
  is a single point of failure; document rotation.

**Verification:**
```
docker compose up -d && curl localhost:8080/health
# enroll a server via the runbook, see it on the dashboard
```

**Exit criteria:** `docker compose up` yields a working manager; VM install
runbook is followable cold; backup/restore tested on a dev box.

---

## Rollback strategy

- Every step lands on a feature branch (per `AGENTS.md` git workflow); the
  dispatcher merges after independent verification. A bad step is reverted at
  the merge, not in the tree.
- DB migrations: every `up.sql` has a `down.sql`. Test down on every migration
  PR. The manager's schema is its own — rolling back a manager migration does
  not touch managed servers.
- jabali2 PRs (Steps 6, 9, 11a, 11b): additive endpoints + new scopes. Rolling
  back = remove the endpoint; existing automation tokens keep working. The
  manager must gracefully handle a 404 from a server that hasn't adopted the
  endpoint (degrade, don't crash) — this is tested in Steps 7, 10, 12a, 12b.
- The remote client (Step 3) is the highest-risk component: a signature bug
  401s every remote call. The golden-vector test (incl. the query-param case)
  is the rollback gate — if it breaks, do not merge.

## v2 (deferred — separate blueprint)

- Mass update orchestration (milestone 7): needs `write:update` scope on
  jabali2 (no write scopes exist in `AllowedAutomationScopes` today), resumable
  batch runner, preflight, per-server result tracking, rollback guidance.
- Cross-server migration (milestone 9): DNS/mail/SSL/package-mismatch
  handling — large, deserves its own ADR series + blueprint.
- Remote token revocation (jabali2 endpoint to revoke a compromised automation
  token from the manager — today rotation is manager-side only; the old token
  stays valid on the managed server until an operator revokes it there).
- Call-home hybrid (if NAT'd servers appear later).

## Open questions to resolve during build

- Step 1b: Kratos vs local JWT for the manager's own admin auth (recommend
  Kratos).
- Step 3: confirmed default = decrypt-per-call. Revisit caching only if load
  testing proves it's necessary, and then behind an ADR.
- Step 11b: delegated-login token TTL + one-time enforcement details (ADR).
- Step 12b: v1 jump policy = any manager admin may jump to any user on any
  enrolled server (ADR 0004). Tighten in v2 if multi-tenant manager admins
  appear.
- Step 13: single-binary-embeds-SPA (jabali2 pattern) vs separate nginx
  container for the SPA.
