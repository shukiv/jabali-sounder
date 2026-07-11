import { Drawer, Statistic, List, Badge, Tag, Empty, Spin, Row, Col } from "antd";
import { useServerHeartbeats, useServerMetrics } from "../hooks/useServers";
import Sparkline from "./Sparkline";
import type { Server } from "../types";

interface Props {
  server: Server | null;
  onClose: () => void;
}

// ServerHistoryDrawer shows a server's recent health-check history (recorded by
// the background poller) plus an uptime summary over the returned window (M1).
function certInfo(iso?: string): { text: string; color: string } | null {
  if (!iso) return null;
  const days = Math.floor((new Date(iso).getTime() - Date.now()) / 86400000);
  const date = new Date(iso).toLocaleDateString();
  if (days < 0) return { text: `expired (${date})`, color: "#cf1322" };
  const color = days < 7 ? "#cf1322" : days < 14 ? "#d48806" : "#3f8600";
  return { text: `${date} · ${days}d left`, color };
}

export default function ServerHistoryDrawer({ server, onClose }: Props) {
  const { data, isLoading } = useServerHeartbeats(server?.id ?? null);
  const { data: metrics } = useServerMetrics(server?.id ?? null);
  const uptimePct = data ? Math.round(data.uptime.ratio * 1000) / 10 : 0;
  const cert = certInfo(server?.cert_expires_at);
  const samples = [...(metrics?.data ?? [])].reverse();
  const series = (pick: (s: (typeof samples)[number]) => number | undefined) =>
    samples.map(pick).filter((v): v is number => v != null);

  return (
    <Drawer
      title={server ? `Health history — ${server.name}` : "Health history"}
      open={!!server}
      onClose={onClose}
      width={480}
      destroyOnClose
    >
      {isLoading ? (
        <Spin />
      ) : !data || data.total === 0 ? (
        <Empty description="No health checks recorded yet" />
      ) : (
        <>
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={12}>
              <Statistic
                title={`Uptime (last ${data.uptime.total})`}
                value={uptimePct}
                precision={1}
                suffix="%"
                valueStyle={{
                  color: uptimePct >= 99 ? "#3f8600" : uptimePct >= 90 ? "#d48806" : "#cf1322",
                }}
              />
            </Col>
            <Col span={12}>
              <Statistic
                title="Healthy checks"
                value={data.uptime.healthy}
                suffix={`/ ${data.uptime.total}`}
              />
            </Col>
          </Row>
          {cert ? (
            <div style={{ marginBottom: 12 }}>
              <span style={{ color: "#888", fontSize: 12 }}>TLS certificate: </span>
              <Tag color={cert.color === "#cf1322" ? "error" : cert.color === "#d48806" ? "warning" : "success"}>
                {cert.text}
              </Tag>
            </div>
          ) : null}
          {samples.length > 1 ? (
            <div style={{ marginBottom: 16 }}>
              <div style={{ color: "#888", fontSize: 12, marginBottom: 4 }}>
                Trends (last {samples.length} samples)
              </div>
              {[
                { label: "CPU %", color: "#1677ff", vals: series((s) => s.cpu_percent) },
                { label: "RAM %", color: "#722ed1", vals: series((s) => s.ram_percent) },
                { label: "Disk %", color: "#d48806", vals: series((s) => s.disk_percent) },
                { label: "Load 1m", color: "#3f8600", vals: series((s) => s.load1) },
              ].map((m) => (
                <Row key={m.label} align="middle" style={{ marginBottom: 4 }}>
                  <Col span={8} style={{ fontSize: 12 }}>
                    {m.label}
                    {m.vals.length ? (
                      <strong> {m.vals[m.vals.length - 1].toFixed(1)}</strong>
                    ) : null}
                  </Col>
                  <Col span={16}>
                    <Sparkline values={m.vals} color={m.color} />
                  </Col>
                </Row>
              ))}
            </div>
          ) : null}
          <List
            size="small"
            dataSource={data.data}
            renderItem={(hb) => (
              <List.Item>
                <Badge
                  status={hb.healthy ? "success" : "error"}
                  text={hb.healthy ? "healthy" : "unhealthy"}
                />
                <span style={{ color: "#888", fontSize: 12 }}>
                  {new Date(hb.checked_at).toLocaleString()}
                </span>
                {hb.version ? <Tag>{hb.version}</Tag> : null}
              </List.Item>
            )}
          />
        </>
      )}
    </Drawer>
  );
}
