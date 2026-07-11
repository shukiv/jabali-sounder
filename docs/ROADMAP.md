# Roadmap

Direction for Jabali Sounder, the central control plane for a fleet of Jabali
Panel servers. Today Sounder is **read-only** (`read:*` scopes), checks health
**on demand**, shows **live-only** metrics, and has a **single admin**. The
milestones below are ordered by leverage — each unlocks the next.

Status legend: 🔭 planned · 🚧 in progress · ✅ done

---

## M1 — Observe → Monitor 🚧

**Goal:** turn Sounder from a passive viewer into an active monitor. Best
value-to-effort, and the foundation everything else builds on.

- ✅ **Background health poller + status history.** Probes every non-disabled
  server on a configurable interval, updates status/credential state, and records
  a heartbeat per poll (with retention). Runs on both the server and desktop
  builds. Status is now current without a manual *Check*. Per-server history
  (uptime + recent checks) is viewable from the Servers table.
- ✅ **Alerting.** The poller fires a notification when a server crosses the
  healthy boundary (down / recovered) via a configured webhook (Slack/Discord/
  Mattermost-compatible). Config: `[alert] webhook_url` / env
  `JABALI_SOUNDER_ALERT_WEBHOOK_URL`. Follow-ups: threshold alerts (disk/cert),
  more channels, and UI configuration.
- **Historical metrics + trends.** Persist the live monitor time series and show
  sparklines / growth (disk, load) instead of only a snapshot.
- **TLS cert-expiry tracking.** Surface each panel's certificate expiry and flag
  soon-to-expire. (Directly relevant to the self-signed / `insecure_skip_verify`
  issues seen in the field.)

**Acceptance:** dashboard reflects current fleet state without a manual Check;
at least one alert channel fires on a threshold breach; disk/load history is
viewable; cert expiry is shown per server.

---

## M2 — Act (remediation) 🔭

**Goal:** the capability jump from read-only to acting on panels.

- **Write / remediation actions** — restart a service, disable a user, suspend a
  domain, clear cache, trigger a backup — via panel write-automation endpoints.
- **Bulk operations** — select N servers → check / disable / tag / act at once.

**Dependencies:** jabali2 must expose write-automation endpoints (tracked as
jabali2-side issues), plus new **write scopes**, a **confirm-before-act** UX, and
audit coverage (audit logging already landed). Cross-repo effort.

**Acceptance:** an operator can perform a scoped write action against a managed
panel from Sounder, gated by an explicit confirmation and recorded in the audit
log.

---

## M3 — Multi-operator 🔭

**Goal:** real accountability behind the audit trail — there is a single shared
`admin` today.

- **Multiple admins + RBAC** (viewer / operator / owner).
- **2FA / TOTP** on login.
- **Session management** — list and revoke active sessions.

**Acceptance:** more than one operator with distinct roles; a viewer cannot
mutate; login supports TOTP; sessions can be revoked; audit events attribute the
acting user.

---

## M4 — Scale & polish 🔭

**Goal:** features that matter as the managed fleet grows.

- **Server groups / environments** (prod / staging) beyond flat tags, with
  per-group dashboards.
- **Fleet version-drift overview** — one glance at "N panels behind latest,"
  highlighting stragglers.
- **Global search** across servers / domains / users / mail.
- **Sounder read-only API tokens** — let external tooling query Sounder, not just
  the SPA.
- **Scheduled reports** — CSV / PDF fleet summaries.

**Acceptance:** servers can be grouped and filtered; out-of-date panels are
obvious; a single search spans all inventories; a token-authenticated read API
exists; a report can be scheduled.

---

If you had to pick one to build next: **M1's poller + alerting** — it changes
what the product *is* (passive → active) and reuses the same plumbing that
history, cert-expiry, and drift depend on.
