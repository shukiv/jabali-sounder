// Notification bodies are composed by the Go poller and stored in the DB, so the
// stored `message` is frozen in the language it was written in. Every row also
// carries the structured fields the message was built from (kind, metric, value,
// threshold, server_name), so we re-render it here in the active language instead
// of showing the stored English. Anything we don't recognise falls back to the
// stored text, so a new server-side notification is never blank.
import i18n from "../i18n";

export interface NotificationLike {
  kind: string;
  metric?: string;
  value?: number;
  threshold?: number;
  server_name?: string;
  message: string;
}

/** Metric labels as the poller writes them ("CPU at 95%", "load1 at 3.20"). */
function metricLabel(metric: string): string {
  return metric === "load1" ? "load1" : metric.toUpperCase();
}

function fmt(metric: string, n?: number): string {
  if (typeof n !== "number" || Number.isNaN(n)) return "";
  return metric === "load1" ? n.toFixed(2) : `${n.toFixed(0)}%`;
}

/**
 * Render a notification in the active language. `kind` drives the shape:
 *   <metric>_high        threshold breach / recovery
 *   service_down:<name>  a managed-server service is not healthy
 *   backup_stale | token_expiring | auto_restart | cert_expiring | down | recovered
 */
export function notificationMessage(n: NotificationLike): string {
  const kind = n.kind || "";
  const raw = n.message || "";
  // Escalations wrap the original message; translate the prefix, recurse on the rest.
  const ESC = "ESCALATION (unacknowledged): ";
  if (raw.startsWith(ESC)) {
    return i18n.t("notifications.escalation", {
      message: notificationMessage({ ...n, message: raw.slice(ESC.length) }),
    });
  }

  if (kind.startsWith("service_down:")) {
    const service = kind.slice("service_down:".length);
    // "service dns is stopped (dead)" -> keep the status/reason payload.
    const m = /^service\s+\S+\s+is\s+(.+)$/i.exec(raw);
    return i18n.t("notifications.service_down", { service, status: m ? m[1] : raw });
  }

  if (kind.endsWith("_high") && n.metric) {
    const label = metricLabel(n.metric);
    // A resolved breach is stored as "<METRIC> back to <value>".
    if (/back to/i.test(raw)) {
      return i18n.t("notifications.metric_recovered", { metric: label, value: fmt(n.metric, n.value) });
    }
    return i18n.t("notifications.metric_high", {
      metric: label,
      value: fmt(n.metric, n.value),
      threshold: fmt(n.metric, n.threshold),
    });
  }

  switch (kind) {
    case "backup_stale": {
      const days = /(\d+)\s*days?/.exec(raw)?.[1];
      return days
        ? i18n.t("notifications.backup_stale", { days })
        : i18n.t("notifications.backup_stale_generic");
    }
    case "token_expiring": {
      const name = /"([^"]+)"|“([^”]+)”/.exec(raw);
      const days = /(\d+)\s*days?/.exec(raw)?.[1];
      if (name && days) {
        return i18n.t("notifications.token_expiring", { name: name[1] ?? name[2], days });
      }
      break;
    }
    case "auto_restart": {
      const name = /"([^"]+)"|“([^”]+)”/.exec(raw);
      const count = /(\d+)\s*failed/.exec(raw)?.[1];
      if (name && count) {
        return i18n.t("notifications.auto_restart", { service: name[1] ?? name[2], count });
      }
      break;
    }
    case "cert_expiring": {
      if (/has expired/i.test(raw)) return i18n.t("notifications.cert_expired");
      const days = /(-?\d+)\s*days?/.exec(raw)?.[1];
      if (days) return i18n.t("notifications.cert_expiring", { days });
      break;
    }
    default:
      break;
  }

  // Health transitions the poller writes verbatim.
  if (/^server unreachable$/i.test(raw)) return i18n.t("notifications.server_unreachable");
  if (/^automation credential invalid$/i.test(raw)) return i18n.t("notifications.credential_invalid");

  return raw;
}
