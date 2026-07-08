# AGENTS.md — Jabali Sounder

Central control plane that manages multiple Jabali Panel servers from one app.
Ships as Docker Compose or standalone VM. Talks to each managed server through
that server's existing panel API + scoped automation tokens.

## Status

Implemented early control-plane app. The current product name is Jabali
Sounder, while some component directories still use `manager-api` and
`manager-ui`.

The repo currently includes:

- Go API/CLI in `manager-api`
- React/Vite/Ant Design UI in `manager-ui`
- MariaDB migrations for servers, heartbeats, and admins
- Dashboard, servers, monitor, domains, users, and mail tabs
- Documentation in `README.md` and `docs/`

Mirror Jabali Panel patterns below for new work; do not invent a second
architecture.

## Reference repo

`/home/shuki/projects/jabali2` is the canonical Jabali Panel implementation.
Before scaffolding, read these files there and copy their patterns verbatim:

- `docs/CONVENTIONS.md` — route family pattern, repository pattern, list
  envelope, error conventions, the security-over-functionality rule
- `Makefile` — exact build/test/lint/coverage targets
- `panel-ui/package.json` — frontend toolchain + pinned versions
- `.golangci.yml`, `.editorconfig` — lint + formatting config

The sounder is a sibling product, not a greenfield experiment.

## Stack (verified from jabali2)

- **Backend:** Go 1.25, Gin, GORM + MariaDB driver, go-redis, golang-migrate,
  cobra CLI, `slog` logging
- **Frontend:** React 19, Vite 6, Ant Design 6, TanStack Query, axios,
  react-router 7
- **Tests:** testify + sqlmock (DB) + miniredis (Redis); no real services in
  unit tests. `go test -race ./...`
- **IDs:** ULID (26-char `CHAR(26)`)
- **Formatting:** Go = tabs/4; everything else = spaces/2; Makefile = tabs
  (enforce via `.editorconfig`)

## Sounder-specific design constraints

These shape the architecture and are easy to default wrong:

- **No SSH to managed nodes.** Use each server's existing panel API + scoped
  automation tokens. Do not assume root SSH access.
- **Explicit server enrollment.** Store server identity, URL, version,
  capabilities, health endpoint, credential status. Support token rotation +
  revocation.
- **Cross-server actions (mass update, migration) must be resumable,
  observable, and failure-isolated per target server.** Preflight checks,
  concurrency limits, rolling batches, per-server result tracking, rollback
  guidance.
- **Delegated login / jump-to-server** uses short-lived tokens + audit records.
- **Every remote action** carries authorization scopes + audit logs + safe
  failure handling.

## Conventions to carry over (non-obvious, regression-prone)

- **List envelope:** `{ "data": [...], "total": N, "page": N, "page_size": N }`
  — NOT `{items,total}`. The frontend reads `.data`.
- **Route family:** one file per resource,
  `RegisterXxxRoutes(g *gin.RouterGroup, cfg)`; panic on nil required deps,
  nil-check optional deps.
- **Repository pattern:** one file per aggregate; interface + GORM impl; wrap
  GORM `ErrRecordNotFound` → `repository.ErrNotFound`.
- **Errors:** `fmt.Errorf("...: %w", err)` always; `slog` not `log.Println`;
  sentinel `var ErrXxx`.
- **SQL:** GORM parameters only — never string-concat.
- **Validation:** at system boundaries only; 422 for domain rules, 400 for
  malformed JSON.
- **Security over functionality is non-negotiable.** Never loosen
  sandbox/hardening/isolation to unblock a feature — change the workflow
  instead. Loosening requires an ADR + edit-site comment + sign-off.

## Commands to replicate (from jabali2 Makefile)

```
make build            # compile binaries
make run              # dev server
make test             # go test -race across all packages
make test-short       # skip integration
make test-coverage    # unit coverage (internal packages, no gate)
make test-integration # requires JABALI_TEST_DATABASE_URL + real MariaDB
make coverage-check   # fails below 80% (needs the integration suite)
make lint             # golangci-lint + install.sh phantom-function lint
make fmt && make vet && make tidy
make ui-install       # npm ci --no-audit --no-fund
make ui-build         # tsc -b && vite build (required before E2E)
make test-ui          # vitest
make test-e2e         # playwright (builds SPA first)
make test-all         # Go + vitest + playwright
```

Pre-commit verification order: `make fmt && make vet && make lint && make test`
(Go) and `npm run lint && npm test` (UI).

## Test gotchas

- `coverage-check` (the 80% gate) requires `JABALI_TEST_DATABASE_URL` pointing
  at a real MariaDB — unit-only `test-coverage` has no gate.
- Integration tests use sqlmock + miniredis; no real Redis/MariaDB in unit tests.
- `TMPDIR` on this box is small — set `TMPDIR=/home/shuki/tmp-go` for large
  test matrices.
- Playwright E2E runs against `dist/` — run `make ui-build` first.

## Git workflow

- Commit to feature branches only, never `main`: `git checkout -b <slug>`.
- Never `git push` — the dispatcher pushes after independent verification.
- Before your final report: `git fetch origin main && git rebase origin/main`,
  then re-run tests.
