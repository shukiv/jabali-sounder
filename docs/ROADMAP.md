# Roadmap

Direction for Jabali Sounder, the central control plane for a fleet of Jabali
Panel servers. Today Sounder is **read-only** (`read:*` scopes), checks health
**on demand**, shows **live-only** metrics, and has a **single admin**. The
milestones below are ordered by leverage — each unlocks the next.

Status legend: 🔭 planned · 🚧 in progress · ✅ done

---

## M1 — Observe → Monitor ✅

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
- ✅ **Historical metrics + trends.** The poller samples CPU/RAM/disk/load per
  interval into a metric_samples table (with retention); the History drawer
  shows inline sparklines. One /automation/status fetch per poll powers both
  health and metrics (no replay collision).
- ✅ **TLS cert-expiry tracking.** The poller samples each panel's TLS cert
  expiry (best-effort, works with self-signed), stores it, shows it in the
  History drawer, and alerts once when it crosses the warning window
  (`[poller] cert_warn_days`, default 14).

**Acceptance:** dashboard reflects current fleet state without a manual Check;
at least one alert channel fires on a threshold breach; disk/load history is
viewable; cert expiry is shown per server.

---

## M2 — Act (remediation) ✅

**Goal:** the capability jump from read-only to acting on panels.

- ✅ **Write / remediation actions** (jabali2 JAB-140 shipped the endpoints).
  Remote write client (restart-service / user disable-enable / domain
  suspend-unsuspend / cache purge / backup + operations + capabilities), write
  scopes in enrollment, Sounder action endpoints (operator+, audited), and
  server-row actions (restart service, purge cache, backup) with confirm.
  User disable/enable on the Users page, domain suspend/unsuspend on the Domains
  page, and backup operation-status polling are wired. (Capability-driven action
  hiding is optional polish — the panel enforces scopes and Sounder surfaces the
  error.)
- ✅ **Bulk operations** — select N servers in the table → check / disable /
  enable / purge-cache / backup in one action.

**Dependencies:** jabali2 must expose write-automation endpoints (tracked as
jabali2-side issues), plus new **write scopes**, a **confirm-before-act** UX, and
audit coverage (audit logging already landed). Cross-repo effort.

**Acceptance:** an operator can perform a scoped write action against a managed
panel from Sounder, gated by an explicit confirmation and recorded in the audit
log.

---

## M3 — Multi-operator ✅

**Goal:** real accountability behind the audit trail — there is a single shared
`admin` today.

- ✅ **Multiple admins + RBAC** (viewer / operator / owner). Role in the JWT,
  RequireRole gate on mutating routes, owner-only Team page + admin CRUD with
  last-owner/self guards.
- ✅ **2FA / TOTP** on login. Hand-rolled RFC-6238; enroll (with QR + manual
  secret) / activate / disable in Settings, TOTP-sealed with the manager key,
  two-step login prompt.
- ✅ **Session management** — server-side session records (JWT carries a session
  id); list active sessions, revoke any (revoked/expired rejected by
  AuthMiddleware), server-side logout, expired-session pruning.

**Acceptance:** more than one operator with distinct roles; a viewer cannot
mutate; login supports TOTP; sessions can be revoked; audit events attribute the
acting user.

---

## M4 — Scale & polish ✅

**Goal:** features that matter as the managed fleet grows.

- ✅ **Server groups / environments** — a single-value environment field per
  server (distinct from multi-tags), an environment filter, and a dashboard
  breakdown (servers + healthy per environment).
- ✅ **Fleet version-drift overview** — dashboard card showing the version
  distribution, the majority version, and how many servers are off it (panel
  versions are git SHAs, so this is drift-from-majority, not "N behind latest").
- ✅ **Global search** (Ctrl/Cmd+K command palette) across enrolled servers and
  the cross-server domain + user inventories, jumping to the relevant page.
- ✅ **Sounder read-only API tokens** — external tooling authenticates with
  `Authorization: Bearer snd_…` for viewer (read-only) access; mint/list/revoke
  in Settings (operator+), sha256-hashed, optional expiry, shown once.
- ✅ **Scheduled reports** — periodic fleet-summary to a webhook (`[report]
  webhook_url` / `interval_hours`), plus an on-demand fleet CSV export from
  Settings.

**Acceptance:** servers can be grouped and filtered; out-of-date panels are
obvious; a single search spans all inventories; a token-authenticated read API
exists; a report can be scheduled.

---

## M5 — Alerting & incidents 🔭

**Goal:** turn point notifications into an alerting system that tells the right
person, once. Compounds the in-app notifications (SND-18) just landed.

- 🔭 **Configurable thresholds + multi-channel routing** (SND-20). Per-metric
  thresholds (disk/RAM/load/cert) with configurable duration, edited in the UI
  not TOML; route to ntfy (phone push), SMTP email, PagerDuty/Opsgenie in
  addition to the existing Slack/Discord webhook; per-channel severity filtering.
  Today alerting is a single webhook with only up/down + one hardcoded CPU rule.
- 🔭 **Incidents with ack / snooze / mute + escalation** (SND-21). Group
  notifications into incidents (open → ack → resolved) with a timeline; ack /
  snooze / mute per incident and per server; escalate if unacked past a window.
- 🔭 **Maintenance windows** (SND-22). Suppress alerts for a server/environment
  during planned work so intentional restarts don't page; scheduled + audited.

**Acceptance:** an operator sets a disk threshold in the UI, gets paged via a
non-webhook channel, acks the incident, and a scheduled maintenance window
suppresses alerts for a named server.

---

## M6 — Observability export & audit 🔭

**Goal:** let the fleet's data leave Sounder and make the audit trail readable.

- 🔭 **Prometheus `/metrics` export** (SND-23). Expose fleet health +
  metric_samples in Prometheus text format behind an API token, so existing
  Grafana/Alertmanager stacks scrape Sounder directly. Low effort, high leverage.
- 🔭 **Audit log viewer + CSV export** (SND-24). Audit events are recorded but
  not viewable; add a filterable/searchable audit page (actor/action/server/time)
  + CSV export. Closes the M3 accountability loop.
- 🔭 **Full metric charts with range selection** (SND-25). Sparklines → real
  time-range charts (zoom, compare servers, fleet-aggregate). Data already exists.
- 🔭 **Uptime / SLA rollup** (SND-26). Uptime % per server from heartbeat
  history; availability card + monthly SLA rollup in reports.

**Acceptance:** an external Grafana scrapes Sounder; the audit page answers "who
restarted what, when"; a server's 30-day uptime is visible.

---

## M7 — Fleet ops 🔭

**Goal:** operational depth that matters as the managed fleet grows.

- 🔭 **Backup management view** (SND-27). Track backups across panels (age, size,
  last-success), alert on stale/missing, trigger + poll status on top of the M2
  backup action.
- 🔭 **Token rotation + credential re-key** (SND-28). One-click rotate a panel's
  automation token, rotate Sounder API tokens, expiry reminders.
- 🔭 **Scheduled remediation / runbooks** (SND-29). Gated + audited automation:
  scheduled backups, auto-restart after N failed checks, respecting maintenance
  windows.

**Acceptance:** stale backups alert; a panel token can be rotated without
re-enrollment; a runbook auto-restarts a flapping service under an operator's
standing approval.

---

## M8 — Hardening 🔭

**Goal:** close the security gaps a fleet-wide control plane accumulates.

- 🔭 **Login rate-limit + brute-force lockout** (SND-30). `/auth/login` is
  unbounded today; add per-IP + per-account backoff/lockout. Pairs with 2FA.
- 🔭 **Scoped API tokens + per-token IP allowlist / rate-limit** (SND-31). `snd_`
  tokens are viewer-all; add read-subset scopes, optional IP allowlist, per-token
  rate limiting.
- 🔭 **Config / policy drift detection** (SND-32). Beyond version drift: flag
  panels with weak TLS, disabled security features, or out-of-policy settings.

**Acceptance:** repeated bad logins are throttled; a token can be scoped +
IP-restricted; the dashboard flags a non-compliant panel.

---

If you had to pick one to build next: **M1's poller + alerting** — it changes
what the product *is* (passive → active) and reuses the same plumbing that
history, cert-expiry, and drift depend on. With M1–M4 shipped, the highest-
leverage next step is **M5 (alerting v2)** — it compounds SND-18 and turns
"shows problems" into "pages the right person, once" — or **M6's Prometheus
export** as the cheapest big win if you already run Grafana.
