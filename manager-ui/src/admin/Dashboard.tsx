import { useTranslation } from "react-i18next";
import { useMemo } from "react";
import { Card, Col, Row, Space, Table, Tag, Typography, Button } from "antd";
import { Link, useNavigate } from "react-router";
import {
  CloudServerOutlined,
  GlobalOutlined,
  TeamOutlined,
  ReloadOutlined,
  DashboardOutlined,
} from "@ant-design/icons";
import { useDashboard, useFleetSLA } from "../hooks/useDashboard";
import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";
import { useDomains, useUsers } from "../hooks/useInventory";
import type { DomainRow, UserRow } from "../hooks/useInventory";
import { StatCard } from "../components/StatCard";
import type { DashboardEntry } from "../types";
import { sortable } from "../lib/tableSort";

const { Title, Text } = Typography;
const RECENT = 5;
const fmt = (n?: number) => (n == null ? "—" : n.toLocaleString());

function statusTag(status: string) {
  const color = status === "active" ? "green" : status === "unreachable" ? "red" : "default";
  return <Tag color={color}>{status}</Tag>;
}

function credTag(cred: string) {
  const color = cred === "valid" ? "green" : cred === "invalid" ? "red" : "orange";
  return <Tag color={color}>{cred}</Tag>;
}

export default function Dashboard() {
  const { t } = useTranslation();
  const { data: servers, isLoading, refetch, isFetching } = useDashboard();
  const { data: sla } = useFleetSLA();
  const { data: policy } = useQuery({
    queryKey: ["policy-summary"],
    queryFn: async () => (await apiClient.get<{ servers_at_risk: number; total: number }>("/admin/policy")).data,
    refetchInterval: 60000,
  });
  const domains = useDomains();
  const users = useUsers();
  const navigate = useNavigate();

  const serverRows = servers || [];
  const envBreakdown = useMemo(() => {
    const map = new Map<string, { total: number; healthy: number }>();
    for (const srv of serverRows) {
      const env = srv.environment || "unassigned";
      const e = map.get(env) || { total: 0, healthy: 0 };
      e.total++;
      if (srv.healthy) e.healthy++;
      map.set(env, e);
    }
    return Array.from(map.entries())
      .map(([env, v]) => ({ env, ...v }))
      .sort((a, b) => b.total - a.total);
  }, [serverRows]);

  const versionDrift = useMemo(() => {
    const map = new Map<string, number>();
    for (const srv of serverRows) {
      const v = srv.version || "unknown";
      map.set(v, (map.get(v) || 0) + 1);
    }
    const rows = Array.from(map.entries())
      .map(([version, count]) => ({ version, count }))
      .sort((a, b) => b.count - a.count);
    const majority = rows[0]?.version;
    const drifted = serverRows.filter((s) => (s.version || "unknown") !== majority).length;
    return { rows, majority, drifted, total: serverRows.length };
  }, [serverRows]);
  const total = serverRows.length;
  const healthy = serverRows.filter((s) => s.healthy && s.status === "active").length;

  let healthTag = <Tag color="green">All healthy</Tag>;
  if (total === 0) healthTag = <Tag>No servers</Tag>;
  else if (healthy < total) healthTag = <Tag color="orange">{`${healthy}/${total} healthy`}</Tag>;

  return (
    <div>
      <Space wrap style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Title level={3} style={{ margin: 0 }}>
          Dashboard
        </Title>
        <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => refetch()}>
          Refresh
        </Button>
      </Space>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))",
          gap: 16,
          marginBottom: 16,
        }}
      >
        <StatCard
          label={t("dashboard.active_servers")}
          value={`${healthy} / ${total}`}
          icon={<CloudServerOutlined />}
          iconColor="#1677ff"
          to="/servers"
        />
        <StatCard
          label={t("dashboard.domains")}
          value={fmt(domains.data?.length)}
          icon={<GlobalOutlined />}
          iconColor="#9254de"
          to="/domains"
        />
        <StatCard
          label={t("dashboard.users")}
          value={fmt(users.data?.length)}
          icon={<TeamOutlined />}
          iconColor="#fa8c16"
          to="/users"
        />
        <StatCard
          label={`Fleet SLA (${sla?.window_days ?? 7}d)`}
          value={sla?.fleet_ratio != null ? `${(sla.fleet_ratio * 100).toFixed(1)}%` : "—"}
          icon={<DashboardOutlined />}
          iconColor={sla?.fleet_ratio != null && sla.fleet_ratio >= 0.99 ? "#3f8600" : "#d48806"}
          to="/monitor"
        />
        <StatCard
          label={t("dashboard.policy_issues")}
          value={policy ? `${policy.servers_at_risk} server${policy.servers_at_risk === 1 ? "" : "s"}` : "—"}
          icon={<DashboardOutlined />}
          iconColor={policy && policy.servers_at_risk > 0 ? "#cf1322" : "#3f8600"}
          to="/policy"
        />
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card>
            <Space direction="vertical" size={12} style={{ width: "100%" }}>
              <Space size={12} wrap>
                <Title level={4} style={{ margin: 0 }}>
                  Fleet
                </Title>
                {healthTag}
              </Space>
              <Text type="secondary">
                Aggregate health across enrolled Jabali Panel servers. For live CPU, RAM,
                and load, see Monitor.
              </Text>
              <Link to="/monitor">
                <Button type="primary" icon={<DashboardOutlined />}>
                  View monitor &rarr;
                </Button>
              </Link>
            </Space>
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card
            title={t("dashboard.servers")}
            size="small"
            extra={
              <Link to="/servers">
                <Button type="primary" size="small">
                  View all
                </Button>
              </Link>
            }
          >
            <Table<DashboardEntry>
              size="small"
              rowKey="id"
              loading={isLoading}
              pagination={false}
              dataSource={serverRows.slice(0, RECENT)}
              columns={sortable([
                { title: t("dashboard.name"), dataIndex: "name", key: "name" },
                { title: t("dashboard.status"), dataIndex: "status", key: "status", render: (s: string) => statusTag(s) },
                {
                  title: t("dashboard.credentials"),
                  dataIndex: "credential_status",
                  key: "credential_status",
                  render: (c: string) => credTag(c),
                },
              ])}
              onRow={() => ({
                onClick: () => navigate("/servers"),
                style: { cursor: "pointer" },
              })}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title={t("dashboard.environments")} size="small">
            <Table<{ env: string; total: number; healthy: number }>
              size="small"
              rowKey="env"
              pagination={false}
              dataSource={envBreakdown}
              columns={sortable([
                { title: t("dashboard.environment"), dataIndex: "env", key: "env", render: (e: string) => <Tag color="geekblue">{e}</Tag> },
                { title: t("dashboard.servers"), dataIndex: "total", key: "total" },
                { title: t("dashboard.healthy"), key: "healthy", render: (_: unknown, r: { total: number; healthy: number }) => `${r.healthy} / ${r.total}` },
              ])}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card
            title={t("dashboard.version_drift")}
            size="small"
            extra={
              versionDrift.drifted > 0 ? (
                <Tag color="warning">{versionDrift.drifted} off majority</Tag>
              ) : (
                <Tag color="success">aligned</Tag>
              )
            }
          >
            <Table
              size="small"
              rowKey="version"
              pagination={false}
              dataSource={versionDrift.rows}
              columns={sortable([
                {
                  title: t("dashboard.version"),
                  dataIndex: "version",
                  key: "version",
                  render: (v: string) => (
                    <span>
                      {v}{" "}
                      {v === versionDrift.majority ? <Tag color="blue">majority</Tag> : null}
                    </span>
                  ),
                },
                { title: t("dashboard.servers"), dataIndex: "count", key: "count" },
              ])}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card
            title={t("dashboard.recent_domains")}
            size="small"
            extra={
              <Link to="/domains">
                <Button type="primary" size="small">
                  View all
                </Button>
              </Link>
            }
          >
            <Table<DomainRow>
              size="small"
              rowKey="id"
              loading={domains.isLoading}
              pagination={false}
              dataSource={(domains.data || []).slice(0, RECENT)}
              columns={sortable([
                { title: t("dashboard.domain"), dataIndex: "name", key: "name" },
                { title: t("dashboard.server"), dataIndex: "server_name", key: "server_name" },
              ])}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card
            title={t("dashboard.recent_users")}
            size="small"
            extra={
              <Link to="/users">
                <Button type="primary" size="small">
                  View all
                </Button>
              </Link>
            }
          >
            <Table<UserRow>
              size="small"
              rowKey="id"
              loading={users.isLoading}
              pagination={false}
              dataSource={(users.data || []).slice(0, RECENT)}
              columns={sortable([
                {
                  title: t("dashboard.user"),
                  dataIndex: "username",
                  key: "username",
                  render: (u: string, r: UserRow) => u || r.email,
                },
                { title: t("dashboard.server"), dataIndex: "server_name", key: "server_name" },
              ])}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
