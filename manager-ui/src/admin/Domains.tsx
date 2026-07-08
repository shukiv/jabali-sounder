import { useState } from "react";
import { Card, Table, Tag, Input, Space, Typography, Statistic, Row, Col } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { useDomains } from "../hooks/useInventory";
import type { DomainRow } from "../hooks/useInventory";

const { Title } = Typography;

export default function Domains() {
  const { data: domains, isLoading } = useDomains();
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
          columns={columns}
          rowKey={(r) => r.server_id + ":" + r.id}
          loading={isLoading}
          pagination={{ pageSize: 50, showSizeChanger: true }}
        />
      </Card>
    </div>
  );
}
