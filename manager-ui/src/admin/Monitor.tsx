import {
  Alert,
  Button,
  Card,
  Col,
  Progress,
  Row,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
} from "antd";
import {
  DashboardOutlined,
  HddOutlined,
  ReloadOutlined,
  TeamOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import { useMemo } from "react";
import { useMonitorLive, useMonitorSummary } from "../hooks/useMonitor";
import type { MonitorLiveEntry, MonitorSummaryEntry } from "../types";

const { Text, Title } = Typography;

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

function statusTag(entry: MonitorLiveEntry) {
  if (entry.available) return <Tag color="green">live</Tag>;
  if (entry.server.status !== "active") return <Tag>{entry.server.status}</Tag>;
  return <Tag color="orange">unavailable</Tag>;
}

export default function Monitor() {
  const live = useMonitorLive();
  const summary = useMonitorSummary();

  const summaryByID = useMemo(() => {
    const map = new Map<string, MonitorSummaryEntry>();
    for (const row of summary.data || []) map.set(row.server.id, row);
    return map;
  }, [summary.data]);

  const liveRows = live.data || [];
  const unavailable = liveRows.filter((row) => !row.available && row.error);

  const columns = [
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
    {
      title: "State",
      key: "state",
      render: (_: unknown, row: MonitorLiveEntry) => statusTag(row),
    },
    {
      title: "CPU",
      key: "cpu",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={4} style={{ minWidth: 160 }}>
          <Progress percent={pct(row.cpu_percent)} size="small" status={row.cpu_percent && row.cpu_percent > 90 ? "exception" : "normal"} />
          <Text type="secondary">{pctText(row.cpu_percent)} · load {row.load1?.toFixed(2) ?? "n/a"}</Text>
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
        </Space>
      ),
    },
    {
      title: "IO",
      key: "io",
      render: (_: unknown, row: MonitorLiveEntry) => (
        <Space direction="vertical" size={0}>
          <Text>{pctText(row.io_wait_percent)} iowait</Text>
          <Text type="secondary">{bytes(row.io_read_bps)}/s read · {bytes(row.io_write_bps)}/s write</Text>
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
    {
      title: "Accounts",
      key: "accounts",
      render: (_: unknown, row: MonitorLiveEntry) => num(summaryByID.get(row.server.id)?.accounts_total),
    },
    {
      title: "Domains",
      key: "domains",
      render: (_: unknown, row: MonitorLiveEntry) => num(summaryByID.get(row.server.id)?.domains_total),
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

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <Space style={{ width: "100%", justifyContent: "space-between" }}>
        <Title level={3} style={{ margin: 0 }}>Monitor</Title>
        <Space>
          <Button icon={<ReloadOutlined />} loading={summary.isFetching} onClick={() => summary.refetch()}>
            Refresh Summary
          </Button>
          <Button type="primary" icon={<ReloadOutlined />} loading={live.isFetching} onClick={() => live.refetch()}>
            Refresh Live
          </Button>
        </Space>
      </Space>

      {unavailable.length > 0 && (
        <Alert
          type="warning"
          showIcon
          title="Some server metrics are unavailable"
          description={unavailable.map((row) => `${row.server.name}: ${row.error}`).join(" · ")}
        />
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}>
          <Card>
            <Statistic title="Live servers" value={totals.live} suffix={`/ ${liveRows.length}`} prefix={<DashboardOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <Card>
            <Statistic title="Avg CPU" value={totals.cpu / liveCount} precision={1} suffix="%" prefix={<ThunderboltOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <Card>
            <Statistic title="Avg RAM" value={totals.ram / liveCount} precision={1} suffix="%" prefix={<HddOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <Card>
            <Statistic title="Accounts / Domains" value={`${totals.accounts.toLocaleString()} / ${totals.domains.toLocaleString()}`} prefix={<TeamOutlined />} />
          </Card>
        </Col>
      </Row>

      <Card>
        <Table<MonitorLiveEntry>
          dataSource={liveRows}
          columns={columns}
          rowKey={(row) => row.server.id}
          loading={live.isLoading || summary.isLoading}
          pagination={false}
          scroll={{ x: 1100 }}
        />
      </Card>
    </Space>
  );
}
