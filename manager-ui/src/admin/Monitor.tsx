import {
  Alert,
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
  Typography,
} from "antd";
import { StatCard } from "../components/StatCard";
import MetricChart from "../components/MetricChart";
import {
  DashboardOutlined,
  HddOutlined,
  ReloadOutlined,
  TeamOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import { useMemo, useState } from "react";
import { useMonitorHistory, useMonitorLive, useMonitorSummary } from "../hooks/useMonitor";
import type {
  MetricSample,
  MonitorLiveEntry,
  MonitorServerRef,
  MonitorSummaryEntry,
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

function num(v?: number) {
  return typeof v === "number" ? v.toLocaleString() : "n/a";
}

function mean(nums: (number | undefined)[]): number | undefined {
  const v = nums.filter((n): n is number => typeof n === "number" && !Number.isNaN(n));
  return v.length ? v.reduce((a, b) => a + b, 0) / v.length : undefined;
}

function peak(nums: (number | undefined)[]): number | undefined {
  const v = nums.filter((n): n is number => typeof n === "number" && !Number.isNaN(n));
  return v.length ? Math.max(...v) : undefined;
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
  const [range, setRange] = useState("live");
  const isLive = range === "live";

  const live = useMonitorLive(isLive);
  const summary = useMonitorSummary();

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
    return {
      server: r.server,
      samples: q?.data?.data ?? [],
      isLoading: !!q?.isLoading,
      isError: !!q?.isError,
    };
  });
  const historyFetching = history.some((q) => q.isFetching);

  const liveRows = live.data || [];
  const unavailable = liveRows.filter((row) => !row.available && row.error);

  const liveColumns = [
    {
      title: "Server",
      key: "server",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={0}>
          <Text strong>{row.server.name}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{row.server.base_url}</Text>
        </Space>
      ),
    },
    { title: "State", key: "state", render: (_: unknown, row: MonitorLiveEntry) => statusTag(row) },
    {
      title: "CPU",
      key: "cpu",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={4} style={{ minWidth: 160 }}>
          <Progress percent={pct(row.cpu_percent)} size="small" status={row.cpu_percent && row.cpu_percent > 90 ? "exception" : "normal"} />
          <Text type="secondary">{pctText(row.cpu_percent)} · load {row.load1?.toFixed(2) ?? "n/a"} · {pctText(row.io_wait_percent)} iowait</Text>
        </Space>
      ),
    },
    {
      title: "RAM",
      key: "ram",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={4} style={{ minWidth: 160 }}>
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
          <Space direction="vertical" size={4} style={{ minWidth: 160 }}>
            <Progress percent={pct(s?.disk_percent)} size="small" status={s?.disk_percent && s.disk_percent > 90 ? "exception" : "normal"} />
            <Text type="secondary">{bytes(s?.disk_used_bytes)} / {bytes(s?.disk_total_bytes)}</Text>
          </Space>
        );
      },
    },
    { title: "Accounts", key: "accounts", render: (_: unknown, row: MonitorLiveEntry) => num(summaryByID.get(row.server.id)?.accounts_total) },
    { title: "Domains", key: "domains", render: (_: unknown, row: MonitorLiveEntry) => num(summaryByID.get(row.server.id)?.domains_total) },
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
    {
      title: "Samples",
      key: "samples",
      render: (_: unknown, row: HistoryRow) =>
        row.isError ? <Text type="secondary">—</Text> : <Text type="secondary">{row.samples.length}</Text>,
    },
  ];

  const totals = liveRows.reduce(
    (acc, row) => {
      const s = summaryByID.get(row.server.id);
      acc.cpu += row.cpu_percent || 0;
      acc.ram += row.ram_percent || 0;
      acc.live += row.available ? 1 : 0;
      acc.accounts += s?.accounts_total || 0;
      acc.domains += s?.domains_total || 0;
      return acc;
    },
    { cpu: 0, ram: 0, live: 0, accounts: 0, domains: 0 },
  );
  const liveCount = Math.max(totals.live, 1);

  // Historical fleet aggregates: mean of each server's range-average.
  const histServerCpu = historyRows.map((r) => mean(r.samples.map((s) => s.cpu_percent)));
  const histServerRam = historyRows.map((r) => mean(r.samples.map((s) => s.ram_percent)));
  const fleetCpu = mean(histServerCpu);
  const fleetRam = mean(histServerRam);
  const withData = historyRows.filter((r) => r.samples.length > 0).length;
  const histAccounts = roster.reduce((a, r) => a + (r.accounts_total || 0), 0);
  const histDomains = roster.reduce((a, r) => a + (r.domains_total || 0), 0);

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <Space wrap style={{ width: "100%", justifyContent: "space-between" }}>
        <Space wrap>
          <Title level={3} style={{ margin: 0 }}>Monitor</Title>
          <Segmented
            options={RANGE_OPTIONS}
            value={range}
            onChange={(v) => setRange(v as string)}
            aria-label="Monitor time range"
          />
        </Space>
        {isLive ? (
          <Space>
            <Button icon={<ReloadOutlined />} loading={summary.isFetching} onClick={() => summary.refetch()}>
              Refresh Summary
            </Button>
            <Button type="primary" icon={<ReloadOutlined />} loading={live.isFetching} onClick={() => live.refetch()}>
              Refresh Live
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
          title="Some server metrics are unavailable"
          description={unavailable.map((row) => `${row.server.name}: ${row.error}`).join(" · ")}
        />
      )}

      <Row gutter={[16, 16]}>
        {isLive ? (
          <>
            <Col xs={24} sm={12} xl={6}>
              <StatCard label="Live servers" value={`${totals.live} / ${liveRows.length}`} Icon={DashboardOutlined} iconColor="#1677ff" />
            </Col>
            <Col xs={24} sm={12} xl={6}>
              <StatCard label="Avg CPU" value={`${liveCount ? (totals.cpu / liveCount).toFixed(1) : "0.0"}%`} Icon={ThunderboltOutlined} iconColor="#fa8c16" />
            </Col>
            <Col xs={24} sm={12} xl={6}>
              <StatCard label="Avg RAM" value={`${liveCount ? (totals.ram / liveCount).toFixed(1) : "0.0"}%`} Icon={HddOutlined} iconColor="#9254de" />
            </Col>
            <Col xs={24} sm={12} xl={6}>
              <StatCard label="Accounts / Domains" value={`${totals.accounts.toLocaleString()} / ${totals.domains.toLocaleString()}`} Icon={TeamOutlined} iconColor="#13c2c2" />
            </Col>
          </>
        ) : (
          <>
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
              <StatCard label="Accounts / Domains" value={`${histAccounts.toLocaleString()} / ${histDomains.toLocaleString()}`} Icon={TeamOutlined} iconColor="#13c2c2" />
            </Col>
          </>
        )}
      </Row>

      <Card>
        {isLive ? (
          <Table<MonitorLiveEntry>
            dataSource={liveRows}
            columns={liveColumns}
            rowKey={(row) => row.server.id}
            loading={live.isLoading || summary.isLoading}
            pagination={false}
            scroll={{ x: 1100 }}
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
    </Space>
  );
}
