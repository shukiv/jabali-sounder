import { useTranslation } from "react-i18next";
import { useState } from "react";
import {
  Card, Table, Button, Modal, Form, Input, Select, Switch, Typography, App, Popconfirm, Tag,
} from "antd";
import { SendOutlined } from "@ant-design/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";
import { SeverityTag } from "./AlertRulesSettings";
import { sortable } from "../lib/tableSort";

const { Title, Paragraph, Text } = Typography;

interface Channel {
  id: string;
  name: string;
  type: string;
  min_severity: string;
  enabled: boolean;
  created_at: string;
}

// Per-type config fields. Secret fields are never returned by the API, so they
// are re-entered on edit.
const CONFIG_FIELDS: Record<string, { key: string; label: string; secret?: boolean; placeholder?: string }[]> = {
  webhook: [{ key: "url", label: "Webhook URL", placeholder: "https://hooks.slack.com/…" }],
  ntfy: [
    { key: "url", label: "Server URL", placeholder: "https://ntfy.sh" },
    { key: "topic", label: "Topic" },
    { key: "token", label: "Access token (optional)", secret: true },
  ],
  smtp: [
    { key: "host", label: "SMTP host" },
    { key: "port", label: "Port", placeholder: "587" },
    { key: "username", label: "Username (optional)" },
    { key: "password", label: "Password (optional)", secret: true },
    { key: "from", label: "From address" },
    { key: "to", label: "To (comma-separated)" },
  ],
  pagerduty: [{ key: "routing_key", label: "Integration routing key", secret: true }],
};

// AlertChannelsSettings manages alert delivery destinations (SND-20, operator+).
export default function AlertChannelsSettings() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { message } = App.useApp();
  const { data, isLoading } = useQuery({
    queryKey: ["alert-channels"],
    queryFn: async () => (await apiClient.get<{ data: Channel[] }>("/admin/alert-channels")).data.data,
  });
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [type, setType] = useState("webhook");
  const [form] = Form.useForm();

  const invalidate = () => qc.invalidateQueries({ queryKey: ["alert-channels"] });

  const create = async () => {
    let values;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const config: Record<string, string> = {};
    for (const f of CONFIG_FIELDS[values.type] || []) {
      if (values[f.key]) config[f.key] = values[f.key];
    }
    setBusy(true);
    try {
      await apiClient.post("/admin/alert-channels", {
        name: values.name,
        type: values.type,
        min_severity: values.min_severity,
        enabled: values.enabled ?? true,
        config,
      });
      message.success(t("alert_channels.channel_created"));
      setOpen(false);
      form.resetFields();
      invalidate();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  const test = async (id: string) => {
    try {
      await apiClient.post(`/admin/alert-channels/${id}/test`);
      message.success(t("alert_channels.test_alert_sent"));
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const remove = async (id: string) => {
    try {
      await apiClient.delete(`/admin/alert-channels/${id}`);
      message.success(t("alert_channels.channel_deleted"));
      invalidate();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    { title: t("alert_channels.name"), dataIndex: "name", key: "name" },
    { title: t("alert_channels.type"), dataIndex: "type", key: "type", render: (t: string) => <Tag>{t}</Tag> },
    {
      title: t("alert_channels.min_severity"),
      dataIndex: "min_severity",
      key: "min_severity",
      render: (s: string) => <SeverityTag severity={s} />,
    },
    {
      title: t("alert_channels.enabled"),
      dataIndex: "enabled",
      key: "enabled",
      render: (e: boolean) => (e ? <Tag color="green">on</Tag> : <Tag>off</Tag>),
    },
    {
      title: t("alert_channels.actions"),
      key: "actions",
      render: (_: unknown, r: Channel) => (
        <>
          <Button size="small" icon={<SendOutlined />} onClick={() => test(r.id)}>
            Test
          </Button>{" "}
          <Popconfirm title={`Delete "${r.name}"?`} okText={t("alert_channels.delete")} okButtonProps={{ danger: true }} onConfirm={() => remove(r.id)}>
            <Button size="small" danger>
              Delete
            </Button>
          </Popconfirm>
        </>
      ),
    },
  ];

  return (
    <Card style={{ marginTop: 16 }}>
      <Title level={4}>Alert channels</Title>
      <Paragraph type="secondary">
        Where alerts are delivered. A channel receives an alert when the incident
        severity is at least its minimum. Secrets are stored encrypted and never
        shown again.
      </Paragraph>
      <Button type="primary" onClick={() => setOpen(true)} style={{ marginBottom: 12 }}>
        Add channel
      </Button>
      <Table<Channel>
        scroll={{ x: "max-content" }}
        dataSource={data || []}
        columns={sortable(columns)}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="small"
      />

      <Modal
        title={t("alert_channels.add_alert_channel")}
        open={open}
        onOk={create}
        confirmLoading={busy}
        onCancel={() => {
          setOpen(false);
          form.resetFields();
        }}
        okText={t("alert_channels.create")}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ type: "webhook", min_severity: "warning", enabled: true }}
          onValuesChange={(chg) => chg.type && setType(chg.type)}
        >
          <Form.Item name="name" label={t("alert_channels.name")} rules={[{ required: true, message: t("alert_channels.name_is_required") }]}>
            <Input placeholder={t("alert_channels.ops_slack")} />
          </Form.Item>
          <Form.Item name="type" label={t("alert_channels.type")} rules={[{ required: true }]}>
            <Select
              options={[
                { value: "webhook", label: t("alert_channels.webhook_slack_discord_mattermost") },
                { value: "ntfy", label: t("alert_channels.ntfy_push") },
                { value: "smtp", label: t("alert_channels.email_smtp") },
                { value: "pagerduty", label: t("alert_channels.pagerduty") },
              ]}
            />
          </Form.Item>
          <Form.Item name="min_severity" label={t("alert_channels.minimum_severity")} rules={[{ required: true }]}>
            <Select
              options={[
                { value: "info", label: t("alert_channels.info_all") },
                { value: "warning", label: t("alert_channels.warning") },
                { value: "critical", label: t("alert_channels.critical_only") },
              ]}
            />
          </Form.Item>
          <Text type="secondary">Configuration</Text>
          {(CONFIG_FIELDS[type] || []).map((f) => (
            <Form.Item key={f.key} name={f.key} label={f.label} style={{ marginTop: 8 }}>
              {f.secret ? <Input.Password placeholder={f.placeholder} /> : <Input placeholder={f.placeholder} />}
            </Form.Item>
          ))}
          <Form.Item name="enabled" label={t("alert_channels.enabled")} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
