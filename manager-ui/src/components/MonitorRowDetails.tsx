import { Descriptions, Space, Tag, Tooltip, Typography } from "antd";
import type { MonitorLiveEntry } from "../types";
import { fmtAbs, fmtAge } from "./monitorFormat";

const { Text } = Typography;

// The workloads we surface service health for (SND-80). The managed Panel API
// does not yet report per-service health, so each is shown capability-aware:
// "unknown" when the enrolled server advertises the capability, "unsupported"
// otherwise. Nothing here is ever reported as healthy without real probe data.
const SERVICES: { key: string; label: string; caps: string[] }[] = [
  { key: "web", label: "Web server", caps: ["web", "nginx", "apache"] },
  { key: "php", label: "PHP-FPM", caps: ["php", "php-fpm"] },
  { key: "db", label: "Database", caps: ["db", "database", "mysql", "mariadb"] },
  { key: "mail", label: "Mail", caps: ["mail", "smtp"] },
  { key: "dns", label: "DNS", caps: ["dns"] },
  { key: "crowdsec", label: "CrowdSec", caps: ["crowdsec"] },
  { key: "docker", label: "Docker", caps: ["docker"] },
  { key: "backup", label: "Backup agent", caps: ["backup"] },
];

type ServiceState = "healthy" | "degraded" | "failed" | "unknown" | "unsupported";

const STATE_COLOR: Record<ServiceState, string> = {
  healthy: "green",
  degraded: "gold",
  failed: "red",
  unknown: "default",
  unsupported: "default",
};

function serviceState(caps: string[], serverCaps?: string[]): ServiceState {
  if (serverCaps && caps.some((c) => serverCaps.includes(c))) return "unknown";
  return "unsupported";
}

interface Props {
  entry: MonitorLiveEntry;
}

// MonitorRowDetails is the expandable detail panel for a Monitor row: it shows
// per-service health (SND-80) and connection/network telemetry (SND-81).
export default function MonitorRowDetails({ entry }: Props) {
  const serverCaps = entry.server.capabilities;
  const latency = entry.api_latency_ms;
  const lastHb = entry.server.last_heartbeat_at;

  const unsupportedTag = <Tag>unsupported</Tag>;

  return (
    <Space direction="vertical" size={12} style={{ width: "100%" }}>
      <div>
        <Text strong>Service health</Text>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 8 }}>
          {SERVICES.map((svc) => {
            const st = serviceState(svc.caps, serverCaps);
            return (
              <Tooltip
                key={svc.key}
                title={
                  st === "unsupported"
                    ? "Not reported by this server's capabilities."
                    : "Supported, but the Panel API does not yet report a live status."
                }
              >
                <Tag color={STATE_COLOR[st]}>
                  {svc.label}: {st}
                </Tag>
              </Tooltip>
            );
          })}
        </div>
        <Text type="secondary" style={{ fontSize: 12 }}>
          Per-service health is capability-aware. Live service status arrives once the managed Panel API exposes it.
        </Text>
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
            { key: "down", label: "Download rate", children: unsupportedTag },
            { key: "up", label: "Upload rate", children: unsupportedTag },
            { key: "loss", label: "Packet loss", children: unsupportedTag },
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
        <Text type="secondary" style={{ fontSize: 12 }}>
          Throughput and packet loss are unsupported until the managed Panel API reports network telemetry.
        </Text>
      </div>
    </Space>
  );
}
