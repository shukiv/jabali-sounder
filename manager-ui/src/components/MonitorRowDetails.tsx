import { Descriptions, Space, Tag, Tooltip, Typography } from "antd";
import {
  CheckCircleFilled,
  CloseCircleFilled,
  ExclamationCircleFilled,
  MinusCircleFilled,
  QuestionCircleFilled,
} from "@ant-design/icons";
import type { ComponentType } from "react";
import type { MonitorLiveEntry } from "../types";
import { fmtAbs, fmtAge } from "./monitorFormat";

const { Text } = Typography;

// Workloads we surface service health for (SND-80). Each `id` matches the name
// the managed Panel reports in service_health (JAB-150). Until real status is
// available a service is shown capability-aware: "unknown" when the enrolled
// server advertises the capability, "unsupported" otherwise — never healthy.
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

type ServiceState = "healthy" | "degraded" | "failed" | "stopped" | "unknown" | "unsupported";

const STATE_STYLE: Record<ServiceState, { color: string; Icon: ComponentType<{ style?: React.CSSProperties }> }> = {
  healthy: { color: "#52c41a", Icon: CheckCircleFilled },
  degraded: { color: "#faad14", Icon: ExclamationCircleFilled },
  failed: { color: "#ff4d4f", Icon: CloseCircleFilled },
  stopped: { color: "#8c8c8c", Icon: MinusCircleFilled },
  unknown: { color: "#8c8c8c", Icon: QuestionCircleFilled },
  unsupported: { color: "#bfbfbf", Icon: MinusCircleFilled },
};

function normStatus(s: string): ServiceState {
  const v = s.toLowerCase();
  if (v === "healthy" || v === "ok" || v === "running") return "healthy";
  if (v === "degraded" || v === "warning") return "degraded";
  if (v === "failed" || v === "critical" || v === "down") return "failed";
  if (v === "stopped" || v === "inactive") return "stopped";
  return "unknown";
}

function prettyName(name: string): string {
  const up = new Set(["dns", "ssh", "api", "php"]);
  return up.has(name.toLowerCase()) ? name.toUpperCase() : name.charAt(0).toUpperCase() + name.slice(1);
}

function label(st: ServiceState): string {
  return st.charAt(0).toUpperCase() + st.slice(1);
}

// ServiceChip renders one service as a status card: a coloured status icon, the
// service name, and its status word (matches the Monitor service-health design).
function ServiceChip({ name, state, tip }: { name: string; state: ServiceState; tip: string }) {
  const { color, Icon } = STATE_STYLE[state];
  return (
    <Tooltip title={tip}>
      <div
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 8,
          padding: "5px 12px",
          borderRadius: 8,
          background: `${color}1f`,
          border: `1px solid ${color}55`,
        }}
      >
        <Icon style={{ color, fontSize: 16 }} />
        <span style={{ display: "flex", flexDirection: "column", lineHeight: 1.15 }}>
          <Text strong style={{ fontSize: 13 }}>{name}</Text>
          <Text style={{ fontSize: 11, color }}>{label(state)}</Text>
        </span>
      </div>
    </Tooltip>
  );
}

interface Props {
  entry: MonitorLiveEntry;
}

// MonitorRowDetails is the expandable detail panel for a Monitor row: per-service
// health (SND-80) and connection/network telemetry (SND-81). Real values render
// when the managed Panel reports them (JAB-150); otherwise fields stay explicitly
// unknown/unsupported.
export default function MonitorRowDetails({ entry }: Props) {
  const serverCaps = entry.server.capabilities;
  const latency = entry.api_latency_ms;
  const lastHb = entry.server.last_heartbeat_at;
  const reported = entry.services || [];
  const net = entry.net;
  const unsupportedTag = <Tag>unsupported</Tag>;

  return (
    <Space direction="vertical" size={12} style={{ width: "100%" }}>
      <div>
        <Text strong>Service health</Text>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 8 }}>
          {reported.length > 0
            ? reported.map((sv) => (
                <ServiceChip
                  key={sv.name}
                  name={prettyName(sv.name)}
                  state={normStatus(sv.status)}
                  tip={
                    [sv.reason, sv.last_checked ? `checked ${fmtAge(sv.last_checked)}` : ""]
                      .filter(Boolean)
                      .join(" · ") || "Reported by the managed Panel."
                  }
                />
              ))
            : SERVICES.map((svc) => {
                const state: ServiceState =
                  serverCaps && svc.caps.some((c) => serverCaps.includes(c)) ? "unknown" : "unsupported";
                return (
                  <ServiceChip
                    key={svc.id}
                    name={svc.label}
                    state={state}
                    tip={
                      state === "unknown"
                        ? "Supported, but the Panel API does not yet report a live status."
                        : "Not reported by this server's capabilities."
                    }
                  />
                );
              })}
        </div>
        {reported.length > 0 ? null : (
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
            { key: "down", label: "Download rate", children: net ? <Text>{fmtRate(net.download_bps)}</Text> : unsupportedTag },
            { key: "up", label: "Upload rate", children: net ? <Text>{fmtRate(net.upload_bps)}</Text> : unsupportedTag },
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
