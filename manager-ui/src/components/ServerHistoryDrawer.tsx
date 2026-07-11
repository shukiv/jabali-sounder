import { useState } from "react";
import { Drawer, Statistic, List, Badge, Tag, Empty, Spin, Row, Col, Segmented } from "antd";
import { useServerHeartbeats, useServerMetrics } from "../hooks/useServers";
import MetricChart from "./MetricChart";
import type { Server } from "../types";

interface Props {
  server: Server | null;
  onClose: () => void;
}

// ServerHistoryDrawer shows a server's recent health-check history (recorded by
// the background poller), an uptime/SLA summary, and range-selectable resource
// trend charts (M1 + M6/SND-25/26).
function certInfo(iso?: string): { text: string; color: string } | null {
  if (!iso) return null;
  const days = Math.floor((new Date(iso).getTime() - Date.now()) / 86400000);
  const date = new Date(iso).toLocaleDateString();
  if (days < 0) return { text: `expired (${date})`, color: "#cf1322" };
  const color = days < 7 ? "#cf1322" : days < 14 ? "#d48806" : "#3f8600";
  return { text: `${date} · ${days}d left`, color };
}

const RANGE_OPTIONS = [
  { label: "Live", value: "live" },
  { label: "6h", value: "6h" },
  { label: "24h", value: "24h" },
  { label: "7d", value: "7d" },
  { label: "30d", value: "30d" },
];

function uptimeColor(pct: number): string {
  return pct >= 99 ? "#3f8600" : pct >= 90 ? "#d48806" : "#cf1322";
}

export default function ServerHistoryDrawer({ server, onClose }: Props) {
  const [range, setRange] = useState("live");
  const { data, isLoading } = useServerHeartbeats(server?.id ?? null);
  const { data: metrics } = useServerMetrics(server?.id ?? null, range === "live" ? undefined : range);

  const uptimePct = data ? Math.round(data.uptime.ratio * 1000) / 10 : 0;
  const slaPct = data?.uptime_window ? Math.round(data.uptime_window.ratio * 1000) / 10 : null;
  const cert = certInfo(server?.cert_expires_at);

  // Ensure chronological order regardless of endpoint (Recent is DESC).
  const samples = [...(metrics?.data ?? [])].sort(
    (a, b) => new Date(a.sampled_at).getTime() - new Date(b.sampled_at).getTime(),
  );
  const timestamps = samples.map((s) => s.sampled_at);
  const percentSeries = [
    { label: "CPU %", color: "#1677ff", values: samples.map((s) => s.cpu_percent ?? 0) },
    { label: "RAM %", color: "#722ed1", values: samples.map((s) => s.ram_percent ?? 0) },
    { label: "Disk %", color: "#d48806", values: samples.map((s) => s.disk_percent ?? 0) },
  ];
  const loadSeries = [{ label: "Load 1m", color: "#3f8600", values: samples.map((s) => s.load1 ?? 0) }];

  return (
    <Drawer
      title={server ? `Health history — ${server.name}` : "Health history"}
      open={!!server}
      onClose={onClose}
      width={720}
      destroyOnClose
    >
      {isLoading ? (
        <Spin />
      ) : !data || data.total === 0 ? (
        <Empty description="No health checks recorded yet" />
      ) : (
        <>
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={8}>
              <Statistic
                title={`Uptime (last ${data.uptime.total})`}
                value={uptimePct}
                precision={1}
                suffix="%"
                valueStyle={{ color: uptimeColor(uptimePct) }}
              />
            </Col>
            {slaPct !== null ? (
              <Col span={8}>
                <Statistic
                  title={`SLA (${data.uptime_window?.window_days}d)`}
                  value={slaPct}
                  precision={1}
                  suffix="%"
                  valueStyle={{ color: uptimeColor(slaPct) }}
                />
              </Col>
            ) : null}
            <Col span={8}>
              <Statistic title="Healthy checks" value={data.uptime.healthy} suffix={`/ ${data.uptime.total}`} />
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

          <div style={{ marginBottom: 8 }}>
            <Segmented options={RANGE_OPTIONS} value={range} onChange={(v) => setRange(v as string)} size="small" />
          </div>
          {samples.length > 1 ? (
            <div style={{ marginBottom: 20 }}>
              <div style={{ color: "#888", fontSize: 12, marginBottom: 4 }}>Resource utilisation</div>
              <MetricChart series={percentSeries} timestamps={timestamps} yMax={100} />
              <div style={{ color: "#888", fontSize: 12, margin: "12px 0 4px" }}>Load average</div>
              <MetricChart series={loadSeries} timestamps={timestamps} yMax={1} height={110} />
            </div>
          ) : (
            <Empty description="No samples in this range" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          )}

          <List
            size="small"
            dataSource={data.data}
            renderItem={(hb) => (
              <List.Item>
                <Badge status={hb.healthy ? "success" : "error"} text={hb.healthy ? "healthy" : "unhealthy"} />
                <span style={{ color: "#888", fontSize: 12 }}>{new Date(hb.checked_at).toLocaleString()}</span>
                {hb.version ? <Tag>{hb.version}</Tag> : null}
              </List.Item>
            )}
          />
        </>
      )}
    </Drawer>
  );
}
