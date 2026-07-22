import { useTranslation } from "react-i18next";
import { useState } from "react";
import { Card, Table, Tag, Input, Space, Typography, Row, Col, Button, Popconfirm, App } from "antd";
import { SearchOutlined, GlobalOutlined, CheckCircleOutlined, CloudServerOutlined } from "@ant-design/icons";
import { StatCard } from "../components/StatCard";
import { useDomains } from "../hooks/useInventory";
import { useServerAction } from "../hooks/useServers";
import { roleAtLeast } from "../hooks/useAuth";
import type { DomainRow } from "../hooks/useInventory";
import { sortable } from "../lib/tableSort";

const { Title } = Typography;

export default function Domains() {
  const { t } = useTranslation();
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
    { title: t("domains.domain"), dataIndex: "name", key: "name", sorter: (a: DomainRow, b: DomainRow) => a.name.localeCompare(b.name) },
    {
      title: t("domains.status"),
      dataIndex: "is_enabled",
      key: "is_enabled",
      render: (enabled: boolean) =>
        enabled ? <Tag color="green">enabled</Tag> : <Tag color="default">disabled</Tag>,
    },
    { title: t("domains.owner"), dataIndex: "user_id", key: "user_id", render: (id: string) => id ? id.slice(0, 10) + "…" : "—" },
    {
      title: t("domains.server"),
      dataIndex: "server_name",
      key: "server_name",
      render: (name: string) => <Tag color="blue">{name}</Tag>,
    },
  ];

  const domainActionsCol = {
    title: t("domains.actions"),
    key: "actions",
    render: (_: unknown, r: DomainRow) =>
      r.is_enabled ? (
        <Popconfirm
          title={`Suspend ${r.name}?`}
          okText={t("domains.suspend")}
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
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label={t("domains.total_domains")} value={domains?.length || 0} Icon={GlobalOutlined} iconColor="#1677ff" />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label={t("domains.enabled")} value={enabled} Icon={CheckCircleOutlined} iconColor="#3f8600" />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label={t("domains.servers")} value={servers.size} Icon={CloudServerOutlined} iconColor="#9254de" />
        </Col>
      </Row>
      <Card>
        <Space wrap style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
          <Input
            placeholder={t("domains.search_domains")}
            aria-label={t("domains.search_domains_2")}
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: "100%", maxWidth: 400 }}
            allowClear
          />
          <span style={{ color: "#888" }}>{filtered.length} of {domains?.length || 0}</span>
        </Space>
        <Table<DomainRow>
          dataSource={filtered}
          columns={sortable(canWrite ? [...columns, domainActionsCol] : columns)}
          rowKey={(r) => r.server_id + ":" + r.id}
          loading={isLoading}
          pagination={{ pageSize: 50, showSizeChanger: true }}
          scroll={{ x: "max-content" }}
        />
      </Card>
    </div>
  );
}
