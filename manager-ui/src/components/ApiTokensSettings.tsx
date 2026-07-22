import { useTranslation } from "react-i18next";
import { useState } from "react";
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  InputNumber,
  Typography,
  Alert,
  App,
  Popconfirm,
  Space,
  Select,
  Tag,
} from "antd";
import { KeyOutlined } from "@ant-design/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";
import { sortable } from "../lib/tableSort";

const { Title, Text, Paragraph } = Typography;

interface ApiToken {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
  scopes?: string[];
  allowed_ips?: string[];
  rate_limit_per_min?: number;
}

const SCOPE_OPTIONS = [
  { value: "read:*", label: "read:* (all)" },
  { value: "fleet", label: "fleet (servers, dashboard)" },
  { value: "monitor", label: "monitor" },
  { value: "inventory", label: "inventory (users, domains, mail)" },
  { value: "metrics", label: "metrics (Prometheus)" },
  { value: "audit", label: "audit" },
  { value: "backups", label: "backups" },
];

// ApiTokensSettings mints/lists/revokes read-only API tokens for external
// tooling (M4). The plaintext token is shown exactly once, at creation.
export default function ApiTokensSettings() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { message } = App.useApp();
  const { data, isLoading } = useQuery({
    queryKey: ["api-tokens"],
    queryFn: async () => (await apiClient.get<{ data: ApiToken[] }>("/admin/api-tokens")).data.data,
  });
  const [open, setOpen] = useState(false);
  const [minted, setMinted] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [form] = Form.useForm();

  const mint = async () => {
    let values;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setBusy(true);
    try {
      const resp = await apiClient.post<{ token: string }>("/admin/api-tokens", {
        name: values.name,
        expires_in_days: values.expires_in_days || 0,
        scopes: values.scopes || [],
        allowed_ips: (values.allowed_ips || "")
          .split(",")
          .map((x: string) => x.trim())
          .filter(Boolean),
        rate_limit_per_min: values.rate_limit_per_min || 0,
      });
      setMinted(resp.data.token);
      setOpen(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["api-tokens"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (id: string) => {
    try {
      await apiClient.delete(`/admin/api-tokens/${id}`);
      message.success(t("api_tokens.token_revoked"));
      qc.invalidateQueries({ queryKey: ["api-tokens"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const rotate = async (id: string) => {
    try {
      const resp = await apiClient.post<{ token: string }>(`/admin/api-tokens/${id}/rotate`);
      setMinted(resp.data.token);
      message.success(t("api_tokens.token_rotated_copy_the_new_value"));
      qc.invalidateQueries({ queryKey: ["api-tokens"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    { title: t("api_tokens.name"), dataIndex: "name", key: "name" },
    {
      title: t("api_tokens.created"),
      dataIndex: "created_at",
      key: "created_at",
      render: (t: string) => new Date(t).toLocaleDateString(),
    },
    {
      title: t("api_tokens.last_used"),
      dataIndex: "last_used_at",
      key: "last_used_at",
      render: (t?: string) => (t ? new Date(t).toLocaleString() : "never"),
    },
    {
      title: t("api_tokens.expires"),
      dataIndex: "expires_at",
      key: "expires_at",
      render: (t?: string) => (t ? new Date(t).toLocaleDateString() : "never"),
    },
    {
      title: t("api_tokens.scopes"),
      dataIndex: "scopes",
      key: "scopes",
      render: (sc?: string[]) =>
        !sc || sc.length === 0 ? <Tag>read:*</Tag> : sc.map((x) => <Tag key={x}>{x}</Tag>),
    },
    {
      title: t("api_tokens.source_ips"),
      dataIndex: "allowed_ips",
      key: "allowed_ips",
      render: (ips?: string[]) => (ips && ips.length ? ips.join(", ") : <Text type="secondary">any</Text>),
    },
    {
      title: t("api_tokens.rate_min"),
      dataIndex: "rate_limit_per_min",
      key: "rate_limit_per_min",
      render: (n?: number) => (n && n > 0 ? n : <Text type="secondary">∞</Text>),
    },
    {
      title: t("api_tokens.actions"),
      key: "actions",
      render: (_: unknown, r: ApiToken) => (
        <Space size={4}>
          <Popconfirm
            title={`Rotate "${r.name}"? The current token stops working immediately.`}
            okText={t("api_tokens.rotate")}
            onConfirm={() => rotate(r.id)}
          >
            <Button size="small">Rotate</Button>
          </Popconfirm>
          <Popconfirm
            title={`Revoke "${r.name}"?`}
            okText={t("api_tokens.revoke")}
            okButtonProps={{ danger: true }}
            onConfirm={() => revoke(r.id)}
          >
            <Button danger size="small">
              Revoke
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card style={{ marginTop: 16 }}>
      <Title level={4}>
        <KeyOutlined /> API tokens
      </Title>
      <Paragraph type="secondary">
        Read-only tokens for external tooling. Send as{" "}
        <Text code>Authorization: Bearer snd_…</Text>. Tokens grant viewer access
        (read-only) and can be revoked any time.
      </Paragraph>

      {minted ? (
        <Alert
          type="success"
          showIcon
          style={{ marginBottom: 12 }}
          message="Copy your token now — it won't be shown again"
          description={
            <Text code copyable style={{ wordBreak: "break-all" }}>
              {minted}
            </Text>
          }
          closable
          onClose={() => setMinted(null)}
        />
      ) : null}

      <Button type="primary" onClick={() => setOpen(true)} style={{ marginBottom: 12 }}>
        Create token
      </Button>
      <Table<ApiToken>
        scroll={{ x: "max-content" }}
        dataSource={data || []}
        columns={sortable(columns)}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="small"
      />

      <Modal
        title={t("api_tokens.create_api_token")}
        open={open}
        onOk={mint}
        confirmLoading={busy}
        onCancel={() => {
          setOpen(false);
          form.resetFields();
        }}
        okText={t("api_tokens.create")}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t("api_tokens.name")} rules={[{ required: true, message: t("api_tokens.name_is_required") }]}>
            <Input placeholder={t("api_tokens.ci_monitoring")} />
          </Form.Item>
          <Form.Item name="expires_in_days" label={t("api_tokens.expires_in_days_blank_never")}>
            <InputNumber min={1} max={3650} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="scopes" label={t("api_tokens.scopes_blank_all_reads")}>
            <Select mode="multiple" allowClear options={SCOPE_OPTIONS} placeholder={t("api_tokens.read")} />
          </Form.Item>
          <Form.Item name="allowed_ips" label={t("api_tokens.source_ip_allowlist_comma_separated_ip")}>
            <Input placeholder={t("api_tokens.203_0_113_7_10_0")} />
          </Form.Item>
          <Form.Item name="rate_limit_per_min" label={t("api_tokens.rate_limit_requests_min_blank_unlimited")}>
            <InputNumber min={1} max={100000} style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
