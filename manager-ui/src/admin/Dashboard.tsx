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
import { useDashboard } from "../hooks/useDashboard";
import { useDomains, useUsers } from "../hooks/useInventory";
import type { DomainRow, UserRow } from "../hooks/useInventory";
import { StatCard } from "../components/StatCard";
import type { DashboardEntry } from "../types";

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
  const { data: servers, isLoading, refetch, isFetching } = useDashboard();
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
  const total = serverRows.length;
  const healthy = serverRows.filter((s) => s.healthy && s.status === "active").length;

  let healthTag = <Tag color="green">All healthy</Tag>;
  if (total === 0) healthTag = <Tag>No servers</Tag>;
  else if (healthy < total) healthTag = <Tag color="orange">{`${healthy}/${total} healthy`}</Tag>;

  return (
    <div>
      <Space style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Title level={3} style={{ margin: 0 }}>
          Dashboard
        </Title>
        <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => refetch()}>
          Refresh
        </Button>
      </Space>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={8}>
          <StatCard
            label="Active Servers"
            value={`${healthy} / ${total}`}
            icon={<CloudServerOutlined />}
            iconColor="#1677ff"
            to="/servers"
          />
        </Col>
        <Col xs={24} sm={8}>
          <StatCard
            label="Domains"
            value={fmt(domains.data?.length)}
            icon={<GlobalOutlined />}
            iconColor="#9254de"
            to="/domains"
          />
        </Col>
        <Col xs={24} sm={8}>
          <StatCard
            label="Users"
            value={fmt(users.data?.length)}
            icon={<TeamOutlined />}
            iconColor="#fa8c16"
            to="/users"
          />
        </Col>
      </Row>

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
            title="Servers"
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
              columns={[
                { title: "Name", dataIndex: "name", key: "name" },
                { title: "Status", dataIndex: "status", key: "status", render: (s: string) => statusTag(s) },
                {
                  title: "Credentials",
                  dataIndex: "credential_status",
                  key: "credential_status",
                  render: (c: string) => credTag(c),
                },
              ]}
              onRow={() => ({
                onClick: () => navigate("/servers"),
                style: { cursor: "pointer" },
              })}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title="Environments" size="small">
            <Table
              size="small"
              rowKey="env"
              pagination={false}
              dataSource={envBreakdown}
              columns={[
                { title: "Environment", dataIndex: "env", key: "env", render: (e: string) => <Tag color="geekblue">{e}</Tag> },
                { title: "Servers", dataIndex: "total", key: "total" },
                { title: "Healthy", key: "healthy", render: (_: unknown, r: { total: number; healthy: number }) => `${r.healthy} / ${r.total}` },
              ]}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card
            title="Recent Domains"
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
              columns={[
                { title: "Domain", dataIndex: "name", key: "name" },
                { title: "Server", dataIndex: "server_name", key: "server_name" },
              ]}
            />
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card
            title="Recent Users"
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
              columns={[
                {
                  title: "User",
                  dataIndex: "username",
                  key: "username",
                  render: (u: string, r: UserRow) => u || r.email,
                },
                { title: "Server", dataIndex: "server_name", key: "server_name" },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
}
