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
      message.success("Token revoked");
      qc.invalidateQueries({ queryKey: ["api-tokens"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const rotate = async (id: string) => {
    try {
      const resp = await apiClient.post<{ token: string }>(`/admin/api-tokens/${id}/rotate`);
      setMinted(resp.data.token);
      message.success("Token rotated — copy the new value now");
      qc.invalidateQueries({ queryKey: ["api-tokens"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    { title: "Name", dataIndex: "name", key: "name" },
    {
      title: "Created",
      dataIndex: "created_at",
      key: "created_at",
      render: (t: string) => new Date(t).toLocaleDateString(),
    },
    {
      title: "Last used",
      dataIndex: "last_used_at",
      key: "last_used_at",
      render: (t?: string) => (t ? new Date(t).toLocaleString() : "never"),
    },
    {
      title: "Expires",
      dataIndex: "expires_at",
      key: "expires_at",
      render: (t?: string) => (t ? new Date(t).toLocaleDateString() : "never"),
    },
    {
      title: "Scopes",
      dataIndex: "scopes",
      key: "scopes",
      render: (sc?: string[]) =>
        !sc || sc.length === 0 ? <Tag>read:*</Tag> : sc.map((x) => <Tag key={x}>{x}</Tag>),
    },
    {
      title: "Source IPs",
      dataIndex: "allowed_ips",
      key: "allowed_ips",
      render: (ips?: string[]) => (ips && ips.length ? ips.join(", ") : <Text type="secondary">any</Text>),
    },
    {
      title: "Rate/min",
      dataIndex: "rate_limit_per_min",
      key: "rate_limit_per_min",
      render: (n?: number) => (n && n > 0 ? n : <Text type="secondary">∞</Text>),
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, r: ApiToken) => (
        <Space size={4}>
          <Popconfirm
            title={`Rotate "${r.name}"? The current token stops working immediately.`}
            okText="Rotate"
            onConfirm={() => rotate(r.id)}
          >
            <Button size="small">Rotate</Button>
          </Popconfirm>
          <Popconfirm
            title={`Revoke "${r.name}"?`}
            okText="Revoke"
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
        dataSource={data || []}
        columns={columns}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="small"
      />

      <Modal
        title="Create API token"
        open={open}
        onOk={mint}
        confirmLoading={busy}
        onCancel={() => {
          setOpen(false);
          form.resetFields();
        }}
        okText="Create"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true, message: "Name is required" }]}>
            <Input placeholder="ci-monitoring" />
          </Form.Item>
          <Form.Item name="expires_in_days" label="Expires in (days, blank = never)">
            <InputNumber min={1} max={3650} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="scopes" label="Scopes (blank = all reads)">
            <Select mode="multiple" allowClear options={SCOPE_OPTIONS} placeholder="read:*" />
          </Form.Item>
          <Form.Item name="allowed_ips" label="Source IP allowlist (comma-separated IP/CIDR, blank = any)">
            <Input placeholder="203.0.113.7, 10.0.0.0/8" />
          </Form.Item>
          <Form.Item name="rate_limit_per_min" label="Rate limit (requests/min, blank = unlimited)">
            <InputNumber min={1} max={100000} style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
