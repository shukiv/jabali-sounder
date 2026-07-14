import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Progress,
  Row,
  Segmented,
  Space,
  Spin,
  Table,
  Tag,
  Tooltip,
  Typography,
} from "antd";
import { StatCard } from "../components/StatCard";
import MetricChart from "../components/MetricChart";
import MonitorRowDetails from "../components/MonitorRowDetails";
import ServerHistoryDrawer from "../components/ServerHistoryDrawer";
import { RowActions } from "../components/RowActions";
import { fmtAbs, fmtAge, fmtUptime } from "../components/monitorFormat";
import {
  CloudServerOutlined,
  DashboardOutlined,
  HddOutlined,
  LinkOutlined,
  PoweroffOutlined,
  ProfileOutlined,
  ReloadOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { useMemo, useState } from "react";
import { useMonitorHistory, useMonitorLive, useMonitorSummary } from "../hooks/useMonitor";
import { useCheckHealth, useDisableServer, useServers } from "../hooks/useServers";
import { roleAtLeast } from "../hooks/useAuth";
import { desktopBridge } from "../lib/desktop";
import type {
  MetricSample,
  MonitorLiveEntry,
  MonitorServerRef,
  MonitorSummaryEntry,
  Server,
} from "../types";

const { Text, Title } = Typography;

const RANGE_OPTIONS = [
  { label: "Live", value: "live" },
  { label: "1h", value: "1h" },
  { label: "6h", value: "6h" },
  { label: "24h", value: "24h" },
  { label: "7d", value: "7d" },
];

const RANGE_LABEL: Record<string, string> = {
  "1h": "last hour",
  "6h": "last 6 hours",
  "24h": "last 24 hours",
  "7d": "last 7 days",
};

const AUTO_OPTIONS = [
  { label: "10s", value: 10000 },
  { label: "30s", value: 30000 },
  { label: "1m", value: 60000 },
  { label: "Off", value: 0 },
];

function pct(v?: number) {
  if (typeof v !== "number" || Number.isNaN(v)) return undefined;
  return Math.max(0, Math.min(100, Math.round(v)));
}

function pctText(v?: number) {
  if (typeof v !== "number" || Number.isNaN(v)) return "n/a";
  return `${v.toFixed(1)}%`;
}

function bytes(v?: number) {
  if (typeof v !== "number" || Number.isNaN(v)) return "n/a";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = v;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`;
}

function mean(nums: (number | undefined)[]): number | undefined {
  const v = nums.filter((n): n is number => typeof n === "number" && !Number.isNaN(n));
  return v.length ? v.reduce((a, b) => a + b, 0) / v.length : undefined;
}

function peak(nums: (number | undefined)[]): number | undefined {
  const v = nums.filter((n): n is number => typeof n === "number" && !Number.isNaN(n));
  return v.length ? Math.max(...v) : undefined;
}

// issues lists the reasons a server needs attention, using the same rules the
// fleet health card counts (SND-82/84). Empty means healthy.
function issues(e: MonitorLiveEntry): string[] {
  const reasons: string[] = [];
  if (e.server.status !== "active") reasons.push(`status: ${e.server.status}`);
  else if (!e.available) reasons.push(e.error || "unreachable");
  if (e.server.credential_status === "invalid") reasons.push("credential invalid");
  for (const a of e.alerts || []) reasons.push(`${a.level}: ${a.kind}${a.detail ? ` — ${a.detail}` : ""}`);
  return reasons;
}

function statusTag(entry: MonitorLiveEntry) {
  if (entry.available) return <Tag color="green">live</Tag>;
  if (entry.server.status !== "active") return <Tag>{entry.server.status}</Tag>;
  return <Tag color="orange">unavailable</Tag>;
}

interface HistoryRow {
  server: MonitorServerRef;
  samples: MetricSample[];
  isLoading: boolean;
  isError: boolean;
}

export default function Monitor() {
  const { message } = App.useApp();
  const [range, setRange] = useState("live");
  const [autoMs, setAutoMs] = useState(10000);
  const [detailsServer, setDetailsServer] = useState<Server | null>(null);
  const isLive = range === "live";
  const canWrite = roleAtLeast("operator");

  const live = useMonitorLive(isLive, isLive && autoMs > 0 ? autoMs : false);
  const summary = useMonitorSummary(isLive ? (autoMs > 0 ? autoMs : false) : 60000);
  const servers = useServers();
  const checkMut = useCheckHealth();
  const disableMut = useDisableServer();

  const serversByID = useMemo(() => {
    const map = new Map<string, Server>();
    for (const s of servers.data || []) map.set(s.id, s);
    return map;
  }, [servers.data]);

  const summaryByID = useMemo(() => {
    const map = new Map<string, MonitorSummaryEntry>();
    for (const row of summary.data || []) map.set(row.server.id, row);
    return map;
  }, [summary.data]);

  const roster = useMemo(() => summary.data || [], [summary.data]);
  const serverIds = useMemo(() => roster.map((r) => r.server.id), [roster]);
  const history = useMonitorHistory(serverIds, range, !isLive);

  const historyRows: HistoryRow[] = roster.map((r, i) => {
    const q = history[i];
    return { server: r.server, samples: q?.data?.data ?? [], isLoading: !!q?.isLoading, isError: !!q?.isError };
  });
  const historyFetching = history.some((q) => q.isFetching);

  const liveRows = live.data || [];
  const unavailable = liveRows.filter((row) => !row.available && row.error);

  const lastUpdated = Math.max(live.dataUpdatedAt || 0, summary.dataUpdatedAt || 0);
  const stale = isLive && (live.isError || (autoMs > 0 && lastUpdated > 0 && Date.now() - lastUpdated > autoMs * 2.5));

  const openDetails = (ref: MonitorServerRef) => {
    const full = serversByID.get(ref.id);
    setDetailsServer(full ?? ({ id: ref.id, name: ref.name } as Server));
  };
  const openPanel = (url: string) => {
    const bridge = desktopBridge();
    if (bridge?.OpenExternal) bridge.OpenExternal(url);
    else window.open(url, "_blank", "noopener,noreferrer");
  };
  const runCheck = async (ref: MonitorServerRef) => {
    try {
      const res = await checkMut.mutateAsync(ref.id);
      if (res?.healthy === false) message.warning(`${ref.name}: health check reported unhealthy`);
      else message.success(`${ref.name}: health check complete`);
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Health check failed");
    }
  };
  const disableMonitoring = async (ref: MonitorServerRef) => {
    try {
      await disableMut.mutateAsync(ref.id);
      message.success(`Monitoring disabled for ${ref.name}`);
    } catch (err) {
      message.error(err instanceof Error ? err.message : "Failed to disable monitoring");
    }
  };

  const refreshLiveView = () => {
    live.refetch();
    summary.refetch();
  };

  const liveColumns = [
    {
      title: "Server",
      key: "server",
      render: (_: unknown, row: MonitorLiveEntry) => {
        const reasons = issues(row);
        return (
          <Space direction="vertical" size={2} style={{ minWidth: 200 }}>
            <Space size={6} align="center">
              <Button
                type="link"
                style={{ padding: 0, height: "auto", fontWeight: 600 }}
                onClick={() => openDetails(row.server)}
              >
                {row.server.name}
              </Button>
              {reasons.length > 0 ? (
                <Tooltip title={reasons.join("; ")}>
                  <WarningOutlined aria-label="Needs attention" style={{ color: "#faad14" }} />
                </Tooltip>
              ) : null}
            </Space>
            <Text type="secondary" style={{ fontSize: 12 }}>{row.server.base_url}</Text>
            <Space size={4} wrap>
              {row.os ? <Tooltip title={row.kernel}><Tag style={{ margin: 0 }}>{row.os}</Tag></Tooltip> : null}
              {row.server.environment ? <Tag color="blue" style={{ margin: 0 }}>{row.server.environment}</Tag> : null}
              {(row.server.tags || []).slice(0, 2).map((t) => <Tag key={t} style={{ margin: 0 }}>{t}</Tag>)}
            </Space>
          </Space>
        );
      },
    },
    {
      title: "State",
      key: "state",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={2}>
          {statusTag(row)}
          {!row.available ? (
            <Tooltip title={fmtAbs(row.server.last_heartbeat_at)}>
              <Text type="secondary" style={{ fontSize: 12 }}>seen {fmtAge(row.server.last_heartbeat_at)}</Text>
            </Tooltip>
          ) : null}
        </Space>
      ),
    },
    {
      title: "CPU",
      key: "cpu",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={4} style={{ minWidth: 150 }}>
          <Progress percent={pct(row.cpu_percent)} size="small" status={row.cpu_percent && row.cpu_percent > 90 ? "exception" : "normal"} />
          <Text type="secondary">{pctText(row.cpu_percent)} · load {row.load1?.toFixed(2) ?? "n/a"} · {pctText(row.io_wait_percent)} iowait</Text>
        </Space>
      ),
    },
    {
      title: "RAM",
      key: "ram",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={4} style={{ minWidth: 150 }}>
          <Progress percent={pct(row.ram_percent)} size="small" status={row.ram_percent && row.ram_percent > 90 ? "exception" : "normal"} />
          <Text type="secondary">{bytes(row.ram_used_bytes)} / {bytes(row.ram_total_bytes)}</Text>
          {row.swap_total_bytes ? (
            <Text type="secondary" style={{ fontSize: 12 }}>swap {bytes(row.swap_used_bytes)} / {bytes(row.swap_total_bytes)}</Text>
          ) : null}
        </Space>
      ),
    },
    {
      title: "Disk",
      key: "disk",
      render: (_: unknown, row: MonitorLiveEntry) => {
        const s = summaryByID.get(row.server.id);
        return (
          <Space direction="vertical" size={4} style={{ minWidth: 150 }}>
            <Progress percent={pct(s?.disk_percent)} size="small" status={s?.disk_percent && s.disk_percent > 90 ? "exception" : "normal"} />
            <Text type="secondary">{bytes(s?.disk_used_bytes)} / {bytes(s?.disk_total_bytes)}</Text>
          </Space>
        );
      },
    },
    {
      title: "Uptime",
      key: "uptime",
      render: (_: unknown, row: MonitorLiveEntry) =>
        row.available ? <Text>{fmtUptime(row.uptime_seconds)}</Text> : <Text type="secondary">—</Text>,
    },
    {
      title: "Connection",
      key: "connection",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={2} style={{ minWidth: 120 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            API {typeof row.api_latency_ms === "number" ? `${row.api_latency_ms} ms` : "n/a"}
          </Text>
          <Tooltip title={fmtAbs(row.server.last_heartbeat_at)}>
            <Text type="secondary" style={{ fontSize: 12 }}>hb {fmtAge(row.server.last_heartbeat_at)}</Text>
          </Tooltip>
        </Space>
      ),
    },
    {
      title: "Actions",
      key: "actions",
      fixed: "right" as const,
      width: 130,
      render: (_: unknown, row: MonitorLiveEntry) => (
        <RowActions
          actions={[
            { key: "details", label: "View details", icon: <ProfileOutlined />, onClick: () => openDetails(row.server) },
            { key: "panel", label: "Open panel", icon: <LinkOutlined />, onClick: () => openPanel(row.server.base_url) },
            {
              key: "check",
              label: "Run health check",
              icon: <ReloadOutlined />,
              loading: checkMut.isPending && checkMut.variables === row.server.id,
              onClick: () => runCheck(row.server),
            },
            ...(canWrite && row.server.status === "active"
              ? [
                  {
                    key: "disable",
                    label: "Disable monitoring",
                    icon: <PoweroffOutlined />,
                    danger: true,
                    onClick: () => disableMonitoring(row.server),
                    confirm: {
                      title: `Disable monitoring for "${row.server.name}"?`,
                      description:
                        "Stops health checks and metric polling for this server. This does NOT delete the server or its history — you can re-enable it anytime.",
                      okText: "Disable monitoring",
                    },
                  },
                ]
              : []),
          ]}
        />
      ),
    },
  ];

  const rangeText = (row: HistoryRow, values: (number | undefined)[]) => {
    if (row.isLoading) return <Spin size="small" />;
    if (row.isError) return <Text type="danger">failed</Text>;
    if (row.samples.length === 0) return <Text type="secondary">no data</Text>;
    return <Text type="secondary">{pctText(mean(values))} / {pctText(peak(values))}</Text>;
  };

  const historyColumns = [
    {
      title: "Server",
      key: "server",
      render: (_: unknown, row: HistoryRow) => (
        <Space direction="vertical" size={0}>
          <Text strong>{row.server.name}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{row.server.base_url}</Text>
        </Space>
      ),
    },
    {
      title: "Trend (CPU / RAM)",
      key: "trend",
      render: (_: unknown, row: HistoryRow) => {
        if (row.isLoading) return <Spin size="small" />;
        if (row.isError) return <Tag color="red">failed</Tag>;
        if (row.samples.length < 2) return <Text type="secondary">no samples</Text>;
        const ts = row.samples.map((s) => s.sampled_at);
        return (
          <div style={{ width: 220, maxWidth: "100%" }}>
            <MetricChart
              height={52}
              timestamps={ts}
              series={[
                { label: "CPU", color: "#1677ff", values: row.samples.map((s) => s.cpu_percent ?? 0) },
                { label: "RAM", color: "#722ed1", values: row.samples.map((s) => s.ram_percent ?? 0) },
              ]}
            />
          </div>
        );
      },
    },
    { title: "CPU avg / peak", key: "cpu", render: (_: unknown, row: HistoryRow) => rangeText(row, row.samples.map((s) => s.cpu_percent)) },
    { title: "RAM avg / peak", key: "ram", render: (_: unknown, row: HistoryRow) => rangeText(row, row.samples.map((s) => s.ram_percent)) },
    { title: "Disk avg / peak", key: "disk", render: (_: unknown, row: HistoryRow) => rangeText(row, row.samples.map((s) => s.disk_percent)) },
    {
      title: "Load avg",
      key: "load",
      render: (_: unknown, row: HistoryRow) => {
        if (row.isLoading) return <Spin size="small" />;
        if (row.isError) return <Text type="danger">failed</Text>;
        const m = mean(row.samples.map((s) => s.load1));
        return <Text type="secondary">{typeof m === "number" ? m.toFixed(2) : "n/a"}</Text>;
      },
    },
    { title: "Samples", key: "samples", render: (_: unknown, row: HistoryRow) => row.isError ? <Text type="secondary">—</Text> : <Text type="secondary">{row.samples.length}</Text> },
  ];

  // ---- Fleet summary cards (SND-84, live mode: six focused values) ----
  const activeRows = liveRows.filter((r) => r.server.status === "active");
  const healthyRows = liveRows.filter((r) => r.available && issues(r).length === 0);
  const issueCount = activeRows.length - healthyRows.length;
  const cpuSamples = liveRows.filter((r) => r.available && typeof r.cpu_percent === "number");
  const ramSamples = liveRows.filter((r) => r.available && typeof r.ram_percent === "number");
  const avgCpu = mean(cpuSamples.map((r) => r.cpu_percent));
  const avgRam = mean(ramSamples.map((r) => r.ram_percent));
  const storageUsed = roster.reduce((a, r) => a + (r.disk_used_bytes || 0), 0);
  const storageTotal = roster.reduce((a, r) => a + (r.disk_total_bytes || 0), 0);
  const accountsTotal = roster.reduce((a, r) => a + (r.accounts_total || 0), 0);
  const domainsTotal = roster.reduce((a, r) => a + (r.domains_total || 0), 0);

  // ---- Historical fleet aggregates (SND-78) ----
  const histServerCpu = historyRows.map((r) => mean(r.samples.map((s) => s.cpu_percent)));
  const histServerRam = historyRows.map((r) => mean(r.samples.map((s) => s.ram_percent)));
  const fleetCpu = mean(histServerCpu);
  const fleetRam = mean(histServerRam);
  const withData = historyRows.filter((r) => r.samples.length > 0).length;

  const cardGrid: React.CSSProperties = {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
    gap: 16,
  };

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <Space wrap style={{ width: "100%", justifyContent: "space-between" }}>
        <Space wrap>
          <Title level={3} style={{ margin: 0 }}>Monitor</Title>
          <Segmented options={RANGE_OPTIONS} value={range} onChange={(v) => setRange(v as string)} aria-label="Monitor time range" />
        </Space>
        {isLive ? (
          <Space wrap>
            <Space size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>Auto refresh:</Text>
              <Segmented size="small" options={AUTO_OPTIONS} value={autoMs} onChange={(v) => setAutoMs(Number(v))} aria-label="Auto refresh interval" />
            </Space>
            <Text type={stale ? "warning" : "secondary"} style={{ fontSize: 12 }}>
              {lastUpdated ? `Updated ${new Date(lastUpdated).toLocaleTimeString()}${stale ? " · stale" : ""}` : "—"}
            </Text>
            <Button icon={<ReloadOutlined />} loading={live.isFetching || summary.isFetching} onClick={refreshLiveView}>
              Refresh
            </Button>
          </Space>
        ) : (
          <Button
            type="primary"
            icon={<ReloadOutlined />}
            loading={summary.isFetching || historyFetching}
            onClick={() => {
              summary.refetch();
              history.forEach((q) => q.refetch());
            }}
          >
            Refresh
          </Button>
        )}
      </Space>

      {!isLive && (
        <Text type="secondary">
          Showing stored trends and averages for the <b>{RANGE_LABEL[range] ?? range}</b>. Live polling is paused.
        </Text>
      )}

      {isLive && unavailable.length > 0 && (
        <Alert
          type="warning"
          showIcon
          message="Some server metrics are unavailable"
          description={unavailable.map((row) => `${row.server.name}: ${row.error}`).join(" · ")}
        />
      )}

      {isLive ? (
        <div style={cardGrid}>
          <StatCard label="Server health" value={`${healthyRows.length} healthy · ${Math.max(issueCount, 0)} issues`} Icon={DashboardOutlined} iconColor={issueCount > 0 ? "#faad14" : "#52c41a"} />
          <StatCard label={`Avg CPU (${cpuSamples.length}/${activeRows.length})`} value={pctText(avgCpu)} Icon={ThunderboltOutlined} iconColor="#fa8c16" />
          <StatCard label={`Avg RAM (${ramSamples.length}/${activeRows.length})`} value={pctText(avgRam)} Icon={HddOutlined} iconColor="#9254de" />
          <StatCard label="Storage used" value={`${bytes(storageUsed)} / ${bytes(storageTotal)}`} Icon={CloudServerOutlined} iconColor="#1677ff" />
          <StatCard label="Accounts" value={accountsTotal.toLocaleString()} Icon={TeamOutlined} iconColor="#13c2c2" />
          <StatCard label="Domains" value={domainsTotal.toLocaleString()} Icon={CloudServerOutlined} iconColor="#eb2f96" />
        </div>
      ) : (
        <Row gutter={[16, 16]}>
          <Col xs={24} sm={12} xl={6}>
            <StatCard label="Servers with data" value={`${withData} / ${roster.length}`} Icon={DashboardOutlined} iconColor="#1677ff" />
          </Col>
          <Col xs={24} sm={12} xl={6}>
            <StatCard label={`Avg CPU (${range})`} value={pctText(fleetCpu)} Icon={ThunderboltOutlined} iconColor="#fa8c16" />
          </Col>
          <Col xs={24} sm={12} xl={6}>
            <StatCard label={`Avg RAM (${range})`} value={pctText(fleetRam)} Icon={HddOutlined} iconColor="#9254de" />
          </Col>
          <Col xs={24} sm={12} xl={6}>
            <StatCard label="Accounts / Domains" value={`${accountsTotal.toLocaleString()} / ${domainsTotal.toLocaleString()}`} Icon={TeamOutlined} iconColor="#13c2c2" />
          </Col>
        </Row>
      )}

      <Card>
        {isLive ? (
          <Table<MonitorLiveEntry>
            dataSource={liveRows}
            columns={liveColumns}
            rowKey={(row) => row.server.id}
            loading={live.isLoading || summary.isLoading}
            pagination={false}
            scroll={{ x: 1300 }}
            expandable={{
              expandedRowRender: (row) => <MonitorRowDetails entry={row} />,
              rowExpandable: (row) => row.server.status === "active",
            }}
          />
        ) : (
          <Table<HistoryRow>
            dataSource={historyRows}
            columns={historyColumns}
            rowKey={(row) => row.server.id}
            loading={summary.isLoading}
            pagination={false}
            scroll={{ x: 1000 }}
          />
        )}
      </Card>

      <ServerHistoryDrawer server={detailsServer} onClose={() => setDetailsServer(null)} />
    </Space>
  );
}
