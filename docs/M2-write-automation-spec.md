# M2 — Write-automation spec (for the Jabali Panel / jabali2 team)

Sounder is a **read-only** control plane today (`read:*` automation scopes). M2
("Act / remediation") lets an operator perform scoped write actions against a
managed panel from Sounder. That requires **jabali2 to expose write-automation
endpoints and write scopes**. This document is the request; Sounder builds the
client side once these land.

Nothing here changes existing read endpoints. All of it is additive and
gated by new, separately-granted scopes.

## 1. Write scopes (least privilege)

Add write scopes to the automation token model, independent of read scopes so an
operator grants only what they need:

| Scope | Grants |
| --- | --- |
| `write:services` | restart / reload managed services |
| `write:users` | disable / enable / suspend / unsuspend users |
| `write:domains` | suspend / unsuspend domains |
| `write:cache` | purge caches |
| `write:backups` | trigger backups |

Requirements:
- A token holds any subset of read and write scopes.
- `read:*` must **not** imply any write scope.
- Each endpoint enforces its scope and returns `403 { "error": "scope_denied" }`
  when absent.

## 2. Endpoints (HMAC-signed, same scheme as read)

Same signing as today:
`Authorization: Jabali-HMAC kid=<id>, ts=<unix>, sig=<hex>` where
`sig = HMAC_SHA256(secret, METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(body)))`.

Reversible actions only for M2 (no delete via automation):

| Method / Path | Scope | Body |
| --- | --- | --- |
| `POST /api/v1/automation/services/{name}/restart` | `write:services` | — |
| `POST /api/v1/automation/users/{id}/disable` | `write:users` | — |
| `POST /api/v1/automation/users/{id}/enable` | `write:users` | — |
| `POST /api/v1/automation/domains/{id}/suspend` | `write:domains` | — |
| `POST /api/v1/automation/domains/{id}/unsuspend` | `write:domains` | — |
| `POST /api/v1/automation/cache/purge` | `write:cache` | `{ "scope": "all"｜"domain", "domain"?: string }` |
| `POST /api/v1/automation/backups` | `write:backups` | `{ "targets": [...] }` |

- Prefer idempotent semantics (disabling an already-disabled user → `200`, not an
  error).
- Short actions (restart, disable, suspend, purge) respond synchronously.

## 3. Async operations

Long actions (backups, possibly restarts) return an operation handle and expose a
status endpoint Sounder can poll:

- Action returns `202 { "operation_id": "...", "status": "pending" }`.
- `GET /api/v1/automation/operations/{id}` → `{ "status": "pending｜running｜done｜failed", "message": "...", "started_at": "...", "finished_at"?: "..." }`.
- Scope: the same scope as the action that created it, or a read-only
  `read:operations`.

## 4. Response envelope

Consistent across all write endpoints:

```json
{ "ok": true, "operation_id": "...optional...", "status": "done", "message": "..." }
```

On error: `{ "ok": false, "error": "<code>", "message": "..." }` with codes:
`scope_denied`, `not_found`, `conflict`, `unsupported`, `rate_limited`,
`internal`.

## 5. Auth & safety (write is higher-stakes than read)

- **Enforce scope per endpoint** (see table). Deny by default.
- **Replay protection** on the `ts`/nonce window as today. (Sounder sends one
  signed request per action, so no same-second-duplicate concern.)
- **Server-side rate limiting / throttling** on write endpoints.
- **Panel-side audit**: record `{ token kid, action, target, result, at }` for
  every write, independent of Sounder's own audit.
- **No irreversible actions in M2**: no account/domain/data deletion via
  automation. Destructive actions, if ever added, must require an explicit
  `"confirm": true` body field and their own scope.
- Optional: a per-token "writes enabled" master switch so an operator can mint a
  read+write-scoped token but keep writes off until deliberately enabled.

## 6. Capability discovery

So Sounder shows only actions a given panel/version supports (and stays
forward-compatible as jabali2 adds actions):

- `GET /api/v1/automation/capabilities` →
  `{ "version": "...", "actions": ["services.restart", "users.disable", "cache.purge", ...], "scopes": ["write:services", ...] }`
- Sounder already stores a `capabilities` array per enrolled server; it will
  populate it from this endpoint and render available actions accordingly.

## 7. What Sounder builds once this lands

- Enrollment UI exposes the new write scopes (opt-in per token).
- Per-action **confirm-before-act** dialog in the UI.
- Records each action in Sounder's audit log (already implemented:
  `auditServerMutation`).
- Polls `operations/{id}` for async actions and reflects status in the UI.
- Bulk actions (act on N servers) reuse the same per-server endpoints.

## Open questions for the jabali2 team

1. Do user/domain ids in automation match the ids Sounder already reads from
   `/automation/users` and `/automation/domains`?
2. Which service names are restartable, and is there an allowlist?
3. Is there an existing operations/jobs framework to reuse for async status?
4. Preferred scope granularity — the five above, or finer/coarser?
