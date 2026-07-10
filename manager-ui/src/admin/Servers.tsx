import { useMemo, useState } from "react";
import {
  Card,
  Table,
  Tag,
  Button,
  Drawer,
  Form,
  Input,
  Space,
  App,
  Select,
  Checkbox,
} from "antd";
import {
  PlusOutlined,
  DeleteOutlined,
  ReloadOutlined,
  EditOutlined,
  PoweroffOutlined,
  PlayCircleOutlined,
} from "@ant-design/icons";
import {
  useServers,
  useCreateServer,
  useUpdateServer,
  useDeleteServer,
  useDisableServer,
  useEnableServer,
  useCheckHealth,
} from "../hooks/useServers";
import { RowActions } from "../components/RowActions";
import type { Server } from "../types";

const scopeOptions = [
  { label: "read:* (all read access)", value: "read:*" },
  { label: "read:domains", value: "read:domains" },
  { label: "read:users", value: "read:users" },
  { label: "read:applications", value: "read:applications" },
  { label: "read:mail", value: "read:mail" },
  { label: "read:status", value: "read:status" },
  { label: "read:metrics", value: "read:metrics" },
];

function statusTag(status: string) {
  const color =
    status === "active" ? "green" :
    status === "unreachable" ? "red" :
    "default";
  return <Tag color={color}>{status}</Tag>;
}

function credTag(cred: string) {
  const color =
    cred === "valid" ? "green" :
    cred === "invalid" ? "red" :
    "orange";
  return <Tag color={color}>{cred}</Tag>;
}

function panelBaseURL(hostname: string) {
  return `https://${hostname.trim()}:8443`;
}

function hostnameFromBaseURL(baseURL: string) {
  try {
    return new URL(baseURL).hostname;
  } catch {
    return baseURL.replace(/^https?:\/\//, "").replace(/:8443\/?$/, "");
  }
}

export default function Servers() {
  const { data: servers, isLoading } = useServers();
  const createMut = useCreateServer();
  const updateMut = useUpdateServer();
  const deleteMut = useDeleteServer();
  const disableMut = useDisableServer();
  const enableMut = useEnableServer();
  const checkMut = useCheckHealth();
  const { message } = App.useApp();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingServer, setEditingServer] = useState<Server | null>(null);
  const [tagFilter, setTagFilter] = useState<string[]>([]);
  const [form] = Form.useForm();

  const tagOptions = useMemo(
    () => Array.from(new Set((servers || []).flatMap((server) => server.tags || [])))
      .sort()
      .map((tag) => ({ label: tag, value: tag })),
    [servers],
  );
  const filteredServers = useMemo(
    () => (servers || []).filter((server) =>
      tagFilter.every((tag) => (server.tags || []).includes(tag))),
    [servers, tagFilter],
  );

  const openCreate = () => {
    setEditingServer(null);
    form.resetFields();
    setDrawerOpen(true);
  };

  const openEdit = (server: Server) => {
    setEditingServer(server);
    form.setFieldsValue({
      name: server.name,
      panel_host: hostnameFromBaseURL(server.base_url),
      scopes: server.scopes,
      tags: server.tags,
      insecure_skip_verify: server.insecure_skip_verify,
      token_id: server.token_id,
    });
    setDrawerOpen(true);
  };

  const closeDrawer = () => {
    setDrawerOpen(false);
    setEditingServer(null);
    form.resetFields();
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      const baseURL = panelBaseURL(values.panel_host);
      if (editingServer) {
        await updateMut.mutateAsync({
          id: editingServer.id,
          name: values.name,
          base_url: baseURL,
          scopes: values.scopes,
          tags: values.tags,
          insecure_skip_verify: values.insecure_skip_verify,
          token_id: values.token_id,
          ...(values.token_secret
            ? { token_secret: values.token_secret }
            : {}),
        });
        message.success("Server updated successfully");
      } else {
        await createMut.mutateAsync({
          name: values.name,
          base_url: baseURL,
          token_id: values.token_id,
          token_secret: values.token_secret,
          scopes: values.scopes,
          tags: values.tags,
          insecure_skip_verify: values.insecure_skip_verify,
        });
        message.success("Server enrolled successfully");
      }
      closeDrawer();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    }
  };

  const handleDelete = async (id: string, name: string) => {
    try {
      await deleteMut.mutateAsync(id);
      message.success(`Deleted server: ${name}`);
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const handleDisable = async (id: string, name: string) => {
    try {
      await disableMut.mutateAsync(id);
      message.success(`Disabled server: ${name}`);
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const handleEnable = async (id: string, name: string) => {
    try {
      await enableMut.mutateAsync(id);
      message.success(`Enabled server: ${name}`);
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const handleCheck = async (id: string) => {
    try {
      const result = await checkMut.mutateAsync(id);
      if (result.reachable && result.credential_valid) {
        message.success(`Server healthy — version ${result.version}`);
      } else if (!result.reachable) {
        message.error("Server unreachable");
      } else {
        message.warning("Server reachable but credentials invalid");
      }
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    { title: "Name", dataIndex: "name", key: "name" },
    { title: "Version", dataIndex: "version", key: "version" },
    {
      title: "Tags",
      dataIndex: "tags",
      key: "tags",
      render: (tags: string[]) => (
        <Space wrap size={[0, 4]}>
          {(tags || []).map((tag) => (
            <Tag key={tag} color="blue">{tag}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (s: string) => statusTag(s),
    },
    {
      title: "Credentials",
      dataIndex: "credential_status",
      key: "credential_status",
      render: (c: string) => credTag(c),
    },
    {
      title: "Scopes",
      dataIndex: "scopes",
      key: "scopes",
      render: (s: string[]) => (
        <Space wrap>
          {(s || []).map((scope) => (
            <Tag key={scope}>{scope}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: "URL",
      dataIndex: "base_url",
      key: "base_url",
      render: (u: string) => (
        <a href={u} target="_blank" rel="noopener noreferrer">
          {u}
        </a>
      ),
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, record: Server) => (
        <RowActions
          actions={[
            {
              key: "check",
              label: "Check",
              icon: <ReloadOutlined />,
              loading: checkMut.isPending && checkMut.variables === record.id,
              onClick: () => handleCheck(record.id),
            },
            {
              key: "edit",
              label: "Edit",
              icon: <EditOutlined />,
              onClick: () => openEdit(record),
            },
            record.status === "disabled"
              ? {
                  key: "enable",
                  label: "Enable",
                  icon: <PlayCircleOutlined />,
                  onClick: () => handleEnable(record.id, record.name),
                }
              : {
                  key: "disable",
                  label: "Disable",
                  icon: <PoweroffOutlined />,
                  onClick: () => handleDisable(record.id, record.name),
                },
            {
              key: "delete",
              label: "Delete",
              icon: <DeleteOutlined />,
              danger: true,
              onClick: () => handleDelete(record.id, record.name),
              confirm: {
                title: `Delete server "${record.name}"?`,
                description:
                  "Permanently removes it and its heartbeat history. This cannot be undone.",
                okText: "Delete",
              },
            },
          ]}
        />
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Space wrap>
          <h3 style={{ margin: 0 }}>Managed Servers</h3>
          <Select
            mode="multiple"
            allowClear
            aria-label="Filter servers by tags"
            placeholder="Filter by tags"
            value={tagFilter}
            options={tagOptions}
            onChange={setTagFilter}
            maxTagCount="responsive"
            style={{ minWidth: 240 }}
          />
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          Add Server
        </Button>
      </Space>
      <Card>
        <Table<Server>
          dataSource={filteredServers}
          columns={columns}
          rowKey="id"
          loading={isLoading}
          pagination={false}
        />
      </Card>

      <Drawer
        title={editingServer ? "Edit Jabali Server" : "Enroll Jabali Server"}
        open={drawerOpen}
        onClose={closeDrawer}
        width={480}
        extra={
          <Space>
            <Button onClick={closeDrawer}>Cancel</Button>
            <Button
              type="primary"
              loading={createMut.isPending || updateMut.isPending}
              onClick={handleSubmit}
            >
              {editingServer ? "Save" : "Enroll"}
            </Button>
          </Space>
        }
      >
        <Form form={form} layout="vertical" requiredMark>
          <Form.Item
            name="name"
            label="Server Name"
            rules={[{ required: true, message: "Enter a display name" }]}
          >
            <Input placeholder="e.g. panel-01.example.com" />
          </Form.Item>
          <Form.Item
            name="panel_host"
            label="Server Hostname"
            rules={[
              { required: true, message: "Enter the panel hostname" },
              {
                pattern: /^[a-zA-Z0-9.-]+$/,
                message: "Enter only the hostname, without https:// or port",
              },
            ]}
            extra="Sounder connects to the panel at https://hostname:8443."
          >
            <Input addonBefore="https://" addonAfter=":8443" placeholder="panel-01.example.com" />
          </Form.Item>
          <Form.Item
            name="token_id"
            label="Automation Token ID"
            rules={[{ required: true, message: "Enter the token ID (ULID)" }]}
          >
            <Input placeholder="01J..." />
          </Form.Item>
          <Form.Item
            name="token_secret"
            label="Automation Token Secret"
            rules={
              editingServer
                ? []
                : [{ required: true, message: "Enter the token secret" }]
            }
            extra={
              editingServer
                ? "Leave blank to keep the current secret. Enter a new value to rotate it."
                : "The hex secret shown once when the token was minted on the managed server."
            }
          >
            <Input.Password
              placeholder={editingServer ? "Leave blank to keep current" : "64-char hex string"}
              autoComplete="new-password"
            />
          </Form.Item>
          <Form.Item name="scopes" label="Scopes">
            <Select
              mode="multiple"
              placeholder="Leave empty for read:* (all read access)"
              options={scopeOptions}
            />
          </Form.Item>
          <Form.Item
            name="tags"
            label="Tags"
            rules={[
              {
                validator: async (_, tags: string[] = []) => {
                  if (tags.length > 20) {
                    throw new Error("Add no more than 20 tags");
                  }
                  const invalid = tags.find((tag) =>
                    tag.trim().length > 40 || !/^[a-z0-9][a-z0-9._-]*$/i.test(tag.trim()));
                  if (invalid) {
                    throw new Error("Tags may contain letters, numbers, dots, underscores, and hyphens");
                  }
                },
              },
            ]}
          >
            <Select
              mode="tags"
              tokenSeparators={[","]}
              maxCount={20}
              placeholder="production, eu-west, customer-a"
            />
          </Form.Item>
          <Form.Item
            name="insecure_skip_verify"
            valuePropName="checked"
            initialValue={false}
            tooltip="Only for panels with self-signed certificates on a trusted LAN. HMAC still authenticates requests, but responses are not verified against a CA."
          >
            <Checkbox>Allow self-signed TLS certificate</Checkbox>
          </Form.Item>
        </Form>
      </Drawer>
    </div>
  );
}
