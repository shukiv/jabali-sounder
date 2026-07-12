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

## M5 — Alerting & incidents ✅

**Goal:** turn point notifications into an alerting system that tells the right
person, once. Compounds the in-app notifications (SND-18).

- ✅ **Configurable thresholds + multi-channel routing** (SND-20). Per-metric
  alert rules (cpu/ram/disk/load1: threshold + duration + severity + enabled),
  edited in the UI, seeded with defaults that preserve the prior CPU behaviour.
  Channels for ntfy (phone push), SMTP email, and PagerDuty alongside the
  Slack/Discord webhook; a dispatcher routes each event to every channel whose
  minimum severity admits it. Channel secrets are AES-GCM sealed and write-only.
- ✅ **Incidents with ack / snooze / mute + escalation** (SND-21). Notifications
  carry severity and an acknowledge / snooze / mute lifecycle; anything left
  unacked past a window escalates once. Mute is per (server, kind).
- ✅ **Maintenance windows** (SND-22). Global / environment / server windows
  suppress alert creation and delivery while active; scheduled from Settings.

**Acceptance:** an operator sets a threshold in the UI, gets notified via a
non-webhook channel, acks the incident, and a scheduled maintenance window
suppresses alerts for a named server — all shipped.

---

## M6 — Observability export & audit ✅

**Goal:** let the fleet's data leave Sounder and make the audit trail readable.

- ✅ **Prometheus `/metrics` export** (SND-23). `/api/v1/metrics/prometheus`
  (token- or session-authed) emits per-server up / cpu / ram / disk / load1 /
  cert-expiry gauges (server/id/environment labels) plus fleet totals in text
  exposition format — existing Grafana/Alertmanager stacks scrape Sounder direct.
- ✅ **Audit log viewer + CSV export** (SND-24). Privileged mutations persist to
  an `audit_logs` table (dual-written with the structured slog event); a
  filterable Audit page (event/actor/time) + CSV export closes the accountability
  loop.
- ✅ **Metric charts with range selection** (SND-25). `/:id/metrics` gains
  6h/24h/7d/30d windows with server-side downsampling; the history drawer renders
  dependency-free multi-series CPU/RAM/disk/load charts.
- ✅ **Uptime / SLA rollup** (SND-26). Uptime computed over a configurable window
  from all stored heartbeats; a 7-day SLA shows in the history drawer and the
  dashboard (fleet + per-server).

**Acceptance:** an external Grafana scrapes Sounder; the audit page answers "who
restarted what, when"; a server's 7-day uptime is visible — all shipped.

---

## M7 — Fleet ops ✅

**Goal:** operational depth that matters as the managed fleet grows.

- ✅ **Backup management view** (SND-27). Backup runs triggered from Sounder are
  recorded and tracked to completion by the poller (panels expose no listing);
  a Backups page shows per-run status/history, and servers with no recent
  successful backup raise a notification. (Panels don't report backup size.)
- ✅ **Token rotation + expiry reminders** (SND-28). One-click rotate a Sounder
  API token (new secret, same id — old stops working immediately); the poller
  reminds before a token expires. Panel credential re-key is the existing server
  update (token_id/token_secret).
- ✅ **Auto-restart remediation** (SND-29). Opt-in, off by default: the poller
  restarts a server's web service after N consecutive failed checks, once per
  outage, skipping maintenance windows, audited as `system:remediation`.
  Config-gated (`[poller] remediation`).

**Acceptance:** stale backups alert; a Sounder token rotates without
re-enrollment; a flapping service auto-restarts under an operator's standing
(config) approval — all shipped.

---

## M8 — Hardening ✅

**Goal:** close the security gaps a fleet-wide control plane accumulates.

- ✅ **Login throttle — per-IP lockout + per-account backoff** (SND-30). The
  per-IP failed-login lockout (SND-3) is joined by a per-account capped backoff
  delay + a one-time brute-force alert (notification + audit). Per-account uses
  backoff, not a hard lock, so an attacker can't lock out the sole admin.
- ✅ **Scoped API tokens + IP allowlist** (SND-31). `snd_` tokens can be minted
  with coarse read scopes (fleet/monitor/inventory/metrics/audit/backups, or
  read:\*) enforced by a scope guard, plus an optional source IP/CIDR allowlist
  enforced at authentication. Rotation preserves scopes/allowlist. (Per-token
  rate-limiting deferred.)
- ✅ **Config / policy drift detection** (SND-32). A policy evaluator flags weak
  TLS (verification disabled), invalid credentials, unreachability, cert expiry,
  and version drift from the fleet majority; surfaced on a Compliance page and a
  dashboard card.

**Acceptance:** repeated bad logins are throttled per-IP and slowed per-account;
a token can be scoped + IP-restricted; the dashboard flags non-compliant panels —
all shipped.

## M9 — Mobile (iOS & Android) 🚧

**Goal:** ship Jabali Sounder as native iOS and Android apps from the *same*
Go backend and React SPA — no second codebase.

Enabled by migrating the desktop app from **Wails v2 to v3**, whose mobile
targets compile the same `main.go` (Go → C shared library, OS WebView renders
the existing frontend, `@wailsio/runtime` bindings unchanged). See
[MOBILE.md](MOBILE.md) for the architecture and build matrix.

- ✅ **P0 — Desktop v2 → v3 migration.** `cmd/desktop` rewritten for
  `application.New` + `AssetOptions{Handler}` (the gin+SPA handler is the asset
  handler, so `/api/v1` and the SPA are served in-process on every platform —
  no open ports on mobile). Bridge is now a v3 Service; the SPA calls it via
  `@wailsio/runtime` `Call.ByName`. Builds on Linux with `-tags gtk3`
  (webkit2gtk-4.1); server build, tests, and UI unaffected.
- 🔭 **P1 — Responsive UI.** Phone-width layouts: bottom-tab nav, safe-area
  insets, mobile-friendly tables/drawers.
- 🔭 **P2 — Android target.** wails3 android build tasks, NDK 26.3 wiring,
  native push (`PostNotification` → the alerting system), Play `.aab`. Needs the
  Android SDK/NDK installed.
- 🔭 **P3 — iOS target.** iOS `//go:build ios` wiring, entitlements, push, App
  Store `.ipa`. Built and tested on macOS (Xcode) — not on the Linux CI box.
- 🔭 **P4 — Store release.** Signing, metadata, CI. Mobile updates ship through
  the App Store / Play Store (binary self-update is desktop-only).

**Acceptance:** the same account, fleet, alerting, and actions work from a phone
app installed from the store; incidents arrive as native push.

---

If you had to pick one to build next: **M1's poller + alerting** — it changes
what the product *is* (passive → active) and reuses the same plumbing that
history, cert-expiry, and drift depend on. With M1–M4 shipped, the highest-
leverage next step is **M5 (alerting v2)** — it compounds SND-18 and turns
"shows problems" into "pages the right person, once" — or **M6's Prometheus
export** as the cheapest big win if you already run Grafana.
