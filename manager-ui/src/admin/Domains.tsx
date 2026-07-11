import { useState } from "react";
import { Card, Table, Tag, Input, Space, Typography, Statistic, Row, Col, Button, Popconfirm, App } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { useDomains } from "../hooks/useInventory";
import { useServerAction } from "../hooks/useServers";
import { roleAtLeast } from "../hooks/useAuth";
import type { DomainRow } from "../hooks/useInventory";

const { Title } = Typography;

export default function Domains() {
  const { data: domains, isLoading } = useDomains();
  const { message } = App.useApp();
  const canWrite = roleAtLeast("operator");
  const actionMut = useServerAction();
  const domainAction = async (r: DomainRow, suspended: boolean) => {
    try {
      await actionMut.mutateAsync({ id: r.server_id, action: "domain", body: { domain_id: r.id, suspended } });
      message.success(suspended ? "Domain suspended" : "Domain unsuspended");
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };
  const [search, setSearch] = useState("");

  const filtered = (domains || []).filter((d) =>
    d.name.toLowerCase().includes(search.toLowerCase()),
  );

  const servers = new Set((domains || []).map((d) => d.server_name));
  const enabled = (domains || []).filter((d) => d.is_enabled).length;

  const columns = [
    { title: "Domain", dataIndex: "name", key: "name", sorter: (a: DomainRow, b: DomainRow) => a.name.localeCompare(b.name) },
    {
      title: "Status",
      dataIndex: "is_enabled",
      key: "is_enabled",
      render: (enabled: boolean) =>
        enabled ? <Tag color="green">enabled</Tag> : <Tag color="default">disabled</Tag>,
    },
    { title: "Owner", dataIndex: "user_id", key: "user_id", render: (id: string) => id ? id.slice(0, 10) + "…" : "—" },
    {
      title: "Server",
      dataIndex: "server_name",
      key: "server_name",
      render: (name: string) => <Tag color="blue">{name}</Tag>,
    },
  ];

  const domainActionsCol = {
    title: "Actions",
    key: "actions",
    render: (_: unknown, r: DomainRow) =>
      r.is_enabled ? (
        <Popconfirm
          title={`Suspend ${r.name}?`}
          okText="Suspend"
          okButtonProps={{ danger: true }}
          onConfirm={() => domainAction(r, true)}
        >
          <Button danger size="small">Suspend</Button>
        </Popconfirm>
      ) : (
        <Button size="small" onClick={() => domainAction(r, false)}>Unsuspend</Button>
      ),
  };

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>Domains</Title>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card size="small">
            <Statistic title="Total Domains" value={domains?.length || 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="Enabled" value={enabled} valueStyle={{ color: "#3f8600" }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="Servers" value={servers.size} />
          </Card>
        </Col>
      </Row>
      <Card>
        <Space style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
          <Input
            placeholder="Search domains…"
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: 400 }}
            allowClear
          />
          <span style={{ color: "#888" }}>{filtered.length} of {domains?.length || 0}</span>
        </Space>
        <Table<DomainRow>
          dataSource={filtered}
          columns={canWrite ? [...columns, domainActionsCol] : columns}
          rowKey={(r) => r.server_id + ":" + r.id}
          loading={isLoading}
          pagination={{ pageSize: 50, showSizeChanger: true }}
        />
      </Card>
    </div>
  );
}
