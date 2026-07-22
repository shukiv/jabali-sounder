import { useTranslation } from "react-i18next";
import { useMemo, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Col,
  Input,
  Row,
  Space,
  Table,
  Tabs,
  Select,
  Grid,
  Tag,
  Typography,
} from "antd";
import { StatCard } from "../components/StatCard";
import type { TableColumnsType } from "antd";
import {
  CheckCircleOutlined,
  MailOutlined,
  ReloadOutlined,
  SearchOutlined,
  TeamOutlined,
  SwapOutlined,
} from "@ant-design/icons";
import { useMail } from "../hooks/useMail";
import type {
  DomainForwarder,
  MailAutoresponder,
  MailForwarder,
  MailGroup,
  MailSnapshotEntry,
  Mailbox,
} from "../types";
import { sortable } from "../lib/tableSort";

const { Text, Title } = Typography;

type ServerStamped<T> = T & {
  row_key: string;
  server_id: string;
  server_name: string;
};

function bytes(v?: number) {
  if (typeof v !== "number" || Number.isNaN(v)) return "n/a";
  if (v === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = v;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`;
}

function dateText(value?: string | null) {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function enabledTag(enabled: boolean) {
  return enabled ? <Tag color="green">enabled</Tag> : <Tag>disabled</Tag>;
}

function serverTag(name: string) {
  return <Tag color="blue">{name}</Tag>;
}

function includes(haystack: Array<string | undefined | null>, needle: string) {
  if (!needle) return true;
  const q = needle.toLowerCase();
  return haystack.some((value) => (value || "").toLowerCase().includes(q));
}

function stampRows<T extends { id?: string }>(
  snapshots: MailSnapshotEntry[],
  key: keyof Pick<MailSnapshotEntry, "mailboxes" | "groups" | "forwarders" | "domain_forwarders" | "autoresponders">,
) {
  return snapshots.flatMap((snapshot) =>
    (snapshot[key] as unknown as T[]).map((row, index) => ({
      ...row,
      row_key: `${snapshot.server.id}:${row.id || index}`,
      server_id: snapshot.server.id,
      server_name: snapshot.server.name,
    })),
  );
}

export default function Mail() {
  const { t } = useTranslation();
  const mail = useMail();
  const [search, setSearch] = useState("");
  const [activeTab, setActiveTab] = useState("mailboxes");
  const screens = Grid.useBreakpoint();

  const snapshots = useMemo(() => mail.data || [], [mail.data]);
  const unavailable = snapshots.filter((row) => !row.available && row.error);

  const rows = useMemo(() => {
    const mailboxes = stampRows<Mailbox>(snapshots, "mailboxes");
    const groups = stampRows<MailGroup>(snapshots, "groups");
    const forwarders = stampRows<MailForwarder>(snapshots, "forwarders");
    const domainForwarders = stampRows<DomainForwarder>(snapshots, "domain_forwarders");
    const autoresponders = snapshots.flatMap((snapshot) =>
      snapshot.autoresponders.map((row, index) => ({
        ...row,
        row_key: `${snapshot.server.id}:${row.mailbox_id || index}`,
        server_id: snapshot.server.id,
        server_name: snapshot.server.name,
      })),
    );
    return { mailboxes, groups, forwarders, domainForwarders, autoresponders };
  }, [snapshots]);

  const filteredMailboxes = rows.mailboxes.filter((row) =>
    includes([row.email, row.display_name, row.domain_name, row.user_username, row.server_name], search),
  );
  const filteredGroups = rows.groups.filter((row) =>
    includes([row.email, row.display_name, row.description, row.group_kind, row.domain_name, row.server_name], search),
  );
  const filteredForwarders = rows.forwarders.filter((row) =>
    includes([row.mailbox_email, row.local_part, row.target, row.domain_name, row.type, row.server_name], search),
  );
  const filteredDomainForwarders = rows.domainForwarders.filter((row) =>
    includes([row.local_part, row.target, row.domain_name, row.type, row.managed_by, row.server_name], search),
  );
  const filteredAutoresponders = rows.autoresponders.filter((row) =>
    includes([row.mailbox_email, row.domain_name, row.subject, row.server_name], search),
  );

  const mailboxColumns: TableColumnsType<ServerStamped<Mailbox>> = [
    {
      title: t("mail.mailbox"),
      key: "email",
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <Text strong>{row.email}</Text>
          <Text type="secondary">{row.display_name || "—"}</Text>
        </Space>
      ),
      sorter: (a, b) => a.email.localeCompare(b.email),
    },
    { title: t("mail.domain"), dataIndex: "domain_name", key: "domain_name" },
    { title: t("mail.owner"), dataIndex: "user_username", key: "user_username", render: (value) => value || "—" },
    { title: t("mail.quota"), dataIndex: "quota_bytes", key: "quota_bytes", render: (value) => bytes(value) },
    { title: t("mail.used"), dataIndex: "last_usage_bytes", key: "last_usage_bytes", render: (value) => bytes(value) },
    {
      title: t("mail.state"),
      dataIndex: "is_disabled",
      key: "is_disabled",
      render: (disabled) => disabled ? <Tag>disabled</Tag> : <Tag color="green">enabled</Tag>,
    },
    { title: t("mail.server"), dataIndex: "server_name", key: "server_name", render: serverTag },
  ];

  const groupColumns: TableColumnsType<ServerStamped<MailGroup>> = [
    {
      title: t("mail.group"),
      key: "email",
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <Text strong>{row.email}</Text>
          <Text type="secondary">{row.display_name || row.description || "—"}</Text>
        </Space>
      ),
      sorter: (a, b) => a.email.localeCompare(b.email),
    },
    { title: t("mail.kind"), dataIndex: "group_kind", key: "group_kind", render: (value) => <Tag>{value}</Tag> },
    { title: t("mail.members"), dataIndex: "member_count", key: "member_count" },
    {
      title: t("mail.services"),
      key: "services",
      render: (_, row) => (
        <Space wrap>
          {row.has_mailbox && <Tag>mail</Tag>}
          {row.has_calendar && <Tag>calendar</Tag>}
          {row.has_addressbook && <Tag>contacts</Tag>}
          {row.has_files && <Tag>files</Tag>}
        </Space>
      ),
    },
    { title: t("mail.scope"), dataIndex: "internal_only", key: "internal_only", render: (value) => value ? <Tag color="orange">internal</Tag> : <Tag>public</Tag> },
    { title: t("mail.server"), dataIndex: "server_name", key: "server_name", render: serverTag },
  ];

  const forwarderColumns: TableColumnsType<ServerStamped<MailForwarder>> = [
    { title: t("mail.mailbox"), dataIndex: "mailbox_email", key: "mailbox_email" },
    { title: t("mail.type"), dataIndex: "type", key: "type", render: (value) => <Tag>{value}</Tag> },
    { title: t("mail.local_part"), dataIndex: "local_part", key: "local_part", render: (value) => value || "—" },
    { title: t("mail.target"), dataIndex: "target", key: "target" },
    { title: t("mail.keep_copy"), dataIndex: "keep_copy", key: "keep_copy", render: (value) => value ? <CheckCircleOutlined /> : "—" },
    { title: t("mail.state"), dataIndex: "enabled", key: "enabled", render: enabledTag },
    { title: t("mail.server"), dataIndex: "server_name", key: "server_name", render: serverTag },
  ];

  const domainForwarderColumns: TableColumnsType<ServerStamped<DomainForwarder>> = [
    { title: t("mail.domain"), dataIndex: "domain_name", key: "domain_name" },
    { title: t("mail.type"), dataIndex: "type", key: "type", render: (value) => <Tag>{value}</Tag> },
    { title: t("mail.local_part"), dataIndex: "local_part", key: "local_part", render: (value) => value || "*" },
    { title: t("mail.target"), dataIndex: "target", key: "target" },
    { title: t("mail.managed_by"), dataIndex: "managed_by", key: "managed_by", render: (value) => value || "—" },
    { title: t("mail.state"), dataIndex: "enabled", key: "enabled", render: enabledTag },
    { title: t("mail.server"), dataIndex: "server_name", key: "server_name", render: serverTag },
  ];

  const autoresponderColumns: TableColumnsType<ServerStamped<MailAutoresponder>> = [
    { title: t("mail.mailbox"), dataIndex: "mailbox_email", key: "mailbox_email", render: (value, row) => value || row.mailbox_id },
    { title: t("mail.domain"), dataIndex: "domain_name", key: "domain_name", render: (value) => value || "—" },
    { title: t("mail.subject"), dataIndex: "subject", key: "subject", render: (value) => value || "—" },
    { title: t("mail.from"), dataIndex: "from_date", key: "from_date", render: dateText },
    { title: t("mail.to"), dataIndex: "to_date", key: "to_date", render: dateText },
    { title: t("mail.state"), dataIndex: "enabled", key: "enabled", render: enabledTag },
    { title: t("mail.server"), dataIndex: "server_name", key: "server_name", render: serverTag },
  ];

  const tableProps = {
    loading: mail.isLoading,
    pagination: { pageSize: 50, showSizeChanger: true },
    scroll: { x: 1000 },
  };

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <Space wrap style={{ width: "100%", justifyContent: "space-between" }}>
        <Title level={3} style={{ margin: 0 }}>Mail</Title>
        <Button type="primary" icon={<ReloadOutlined />} loading={mail.isFetching} onClick={() => mail.refetch()}>
          Refresh
        </Button>
      </Space>

      {unavailable.length > 0 && (
        <Alert
          type="warning"
          showIcon
          title={t("mail.some_server_mail_data_is_unavailable")}
          description={unavailable.map((row) => `${row.server.name}: ${row.error}`).join(" · ")}
        />
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}>
          <StatCard label={t("mail.mailboxes")} value={rows.mailboxes.length} Icon={MailOutlined} iconColor="#1677ff" />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <StatCard label={t("mail.forwarders")} value={rows.forwarders.length + rows.domainForwarders.length} Icon={SwapOutlined} iconColor="#fa8c16" />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <StatCard label={t("mail.groups")} value={rows.groups.length} Icon={TeamOutlined} iconColor="#9254de" />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <StatCard label={t("mail.autoresponders")} value={rows.autoresponders.length} Icon={CheckCircleOutlined} iconColor="#3f8600" />
        </Col>
      </Row>

      <Card>
        <Space wrap style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
          <Input
            placeholder={t("mail.search_mail_stack")}
            aria-label={t("mail.search_mail")}
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: "100%", maxWidth: 420 }}
            allowClear
          />
          <Text type="secondary">{snapshots.filter((row) => row.available).length} / {snapshots.length} servers available</Text>
        </Space>
        {(() => {
          const mailTabs = [
            {
              key: "mailboxes",
              label: `Mailboxes (${filteredMailboxes.length})`,
              children: (
                <Table<ServerStamped<Mailbox>>
                  {...tableProps}
                  dataSource={filteredMailboxes}
                  columns={sortable(mailboxColumns)}
                  rowKey="row_key"
                />
              ),
            },
            {
              key: "forwarders",
              label: `Forwarders (${filteredForwarders.length})`,
              children: (
                <Table<ServerStamped<MailForwarder>>
                  {...tableProps}
                  dataSource={filteredForwarders}
                  columns={sortable(forwarderColumns)}
                  rowKey="row_key"
                />
              ),
            },
            {
              key: "domain-forwarders",
              label: `Domain Forwarders (${filteredDomainForwarders.length})`,
              children: (
                <Table<ServerStamped<DomainForwarder>>
                  {...tableProps}
                  dataSource={filteredDomainForwarders}
                  columns={sortable(domainForwarderColumns)}
                  rowKey="row_key"
                />
              ),
            },
            {
              key: "groups",
              label: `Groups (${filteredGroups.length})`,
              children: (
                <Table<ServerStamped<MailGroup>>
                  {...tableProps}
                  dataSource={filteredGroups}
                  columns={sortable(groupColumns)}
                  rowKey="row_key"
                />
              ),
            },
            {
              key: "autoresponders",
              label: `Autoresponders (${filteredAutoresponders.length})`,
              children: (
                <Table<ServerStamped<MailAutoresponder>>
                  {...tableProps}
                  dataSource={filteredAutoresponders}
                  columns={sortable(autoresponderColumns)}
                  rowKey="row_key"
                />
              ),
            },
          ];
          return screens.sm === false ? (
            <>
              <Select
                value={activeTab}
                onChange={setActiveTab}
                style={{ width: "100%", marginBottom: 12 }}
                aria-label={t("mail.mail_category")}
                options={mailTabs.map((t) => ({ value: t.key, label: t.label }))}
              />
              {mailTabs.find((t) => t.key === activeTab)?.children}
            </>
          ) : (
            <Tabs items={mailTabs} activeKey={activeTab} onChange={setActiveTab} />
          );
        })()}
      </Card>
    </Space>
  );
}
