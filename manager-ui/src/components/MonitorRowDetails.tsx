import { Descriptions, Space, Tag, Tooltip, Typography } from "antd";
import type { MonitorLiveEntry } from "../types";
import { fmtAbs, fmtAge } from "./monitorFormat";

const { Text } = Typography;

// The workloads we surface service health for (SND-80). Each `id` matches the
// name the managed Panel reports in `services[]` (JAB-150). Until the Panel
// exposes real status, each is shown capability-aware: "unknown" when the
// enrolled server advertises the capability, "unsupported" otherwise. Nothing is
// ever reported healthy without a real probe.
const SERVICES: { id: string; label: string; caps: string[] }[] = [
  { id: "web", label: "Web server", caps: ["web", "nginx", "apache"] },
  { id: "php-fpm", label: "PHP-FPM", caps: ["php", "php-fpm"] },
  { id: "database", label: "Database", caps: ["db", "database", "mysql", "mariadb"] },
  { id: "mail", label: "Mail", caps: ["mail", "smtp"] },
  { id: "dns", label: "DNS", caps: ["dns"] },
  { id: "crowdsec", label: "CrowdSec", caps: ["crowdsec"] },
  { id: "docker", label: "Docker", caps: ["docker"] },
  { id: "backup", label: "Backup agent", caps: ["backup"] },
];

type ServiceState = "healthy" | "degraded" | "failed" | "unknown" | "unsupported";

const STATE_COLOR: Record<ServiceState, string> = {
  healthy: "green",
  degraded: "gold",
  failed: "red",
  unknown: "default",
  unsupported: "default",
};

function normStatus(s: string): ServiceState {
  const v = s.toLowerCase();
  if (v === "healthy" || v === "ok" || v === "running") return "healthy";
  if (v === "degraded" || v === "warning") return "degraded";
  if (v === "failed" || v === "critical" || v === "stopped" || v === "down") return "failed";
  return "unknown";
}

function fmtRate(bps?: number): string {
  if (typeof bps !== "number" || Number.isNaN(bps) || bps < 0) return "n/a";
  const units = ["bit/s", "kbit/s", "Mbit/s", "Gbit/s"];
  let v = bps;
  let u = 0;
  while (v >= 1000 && u < units.length - 1) {
    v /= 1000;
    u += 1;
  }
  return `${v >= 10 || u === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[u]}`;
}

interface Props {
  entry: MonitorLiveEntry;
}

// MonitorRowDetails is the expandable detail panel for a Monitor row: it shows
// per-service health (SND-80) and connection/network telemetry (SND-81). Real
// values are rendered when the managed Panel reports them (JAB-150); otherwise
// fields stay explicitly unknown/unsupported.
export default function MonitorRowDetails({ entry }: Props) {
  const serverCaps = entry.server.capabilities;
  const latency = entry.api_latency_ms;
  const lastHb = entry.server.last_heartbeat_at;

  const svcByName = new Map((entry.services || []).map((s) => [s.name.toLowerCase(), s]));
  const net = entry.net;
  const unsupportedTag = <Tag>unsupported</Tag>;

  return (
    <Space direction="vertical" size={12} style={{ width: "100%" }}>
      <div>
        <Text strong>Service health</Text>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 8 }}>
          {SERVICES.map((svc) => {
            const real = svcByName.get(svc.id);
            let st: ServiceState;
            let tip: string;
            if (real) {
              st = normStatus(real.status);
              tip = [real.reason, real.last_checked ? `checked ${fmtAge(real.last_checked)}` : ""]
                .filter(Boolean)
                .join(" · ") || "Reported by the managed Panel.";
            } else if (serverCaps && svc.caps.some((c) => serverCaps.includes(c))) {
              st = "unknown";
              tip = "Supported, but the Panel API does not yet report a live status.";
            } else {
              st = "unsupported";
              tip = "Not reported by this server's capabilities.";
            }
            return (
              <Tooltip key={svc.id} title={tip}>
                <Tag color={STATE_COLOR[st]}>
                  {svc.label}: {st}
                </Tag>
              </Tooltip>
            );
          })}
        </div>
        {entry.services && entry.services.length > 0 ? null : (
          <Text type="secondary" style={{ fontSize: 12 }}>
            Per-service health is capability-aware. Live service status arrives once the managed Panel API exposes it.
          </Text>
        )}
      </div>

      <div>
        <Text strong>Connection</Text>
        <Descriptions
          size="small"
          column={{ xs: 1, sm: 2, lg: 3 }}
          style={{ marginTop: 8 }}
          items={[
            {
              key: "latency",
              label: "Panel API latency",
              children:
                typeof latency === "number" ? (
                  <Text>{latency} ms <Text type="secondary" style={{ fontSize: 12 }}>(GET /automation/status)</Text></Text>
                ) : (
                  <Text type="secondary">unknown</Text>
                ),
            },
            {
              key: "heartbeat",
              label: "Last successful heartbeat",
              children: lastHb ? (
                <Text>{fmtAbs(lastHb)} <Text type="secondary" style={{ fontSize: 12 }}>({fmtAge(lastHb)})</Text></Text>
              ) : (
                <Text type="secondary">never</Text>
              ),
            },
            {
              key: "down",
              label: "Download rate",
              children: net ? <Text>{fmtRate(net.download_bps)}</Text> : unsupportedTag,
            },
            {
              key: "up",
              label: "Upload rate",
              children: net ? <Text>{fmtRate(net.upload_bps)}</Text> : unsupportedTag,
            },
            {
              key: "loss",
              label: "Packet loss",
              children: net ? (
                <Text>{net.packet_loss_pct.toFixed(1)}%{net.window_seconds ? <Text type="secondary" style={{ fontSize: 12 }}> ({net.window_seconds}s)</Text> : null}</Text>
              ) : (
                unsupportedTag
              ),
            },
            {
              key: "ntp",
              label: "Clock (NTP)",
              children:
                entry.ntp_synced === undefined ? (
                  <Text type="secondary">unknown</Text>
                ) : entry.ntp_synced ? (
                  <Tag color="green">synced</Tag>
                ) : (
                  <Tag color="gold">not synced</Tag>
                ),
            },
          ]}
        />
        {net ? null : (
          <Text type="secondary" style={{ fontSize: 12 }}>
            Throughput and packet loss are unsupported until the managed Panel API reports network telemetry.
          </Text>
        )}
      </div>
    </Space>
  );
}
