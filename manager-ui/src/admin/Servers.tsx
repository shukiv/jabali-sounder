import { useTranslation } from "react-i18next";
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
  AutoComplete,
  Modal,
} from "antd";
import {
  PlusOutlined,
  DeleteOutlined,
  ReloadOutlined,
  EditOutlined,
  PoweroffOutlined,
  PlayCircleOutlined,
  HistoryOutlined,
} from "@ant-design/icons";
import {
  useServers,
  useCreateServer,
  useUpdateServer,
  useDeleteServer,
  useDisableServer,
  useEnableServer,
  useCheckHealth,
  useServerAction,
} from "../hooks/useServers";
import { RowActions } from "../components/RowActions";
import ServerHistoryDrawer from "../components/ServerHistoryDrawer";
import type { Server } from "../types";
import { roleAtLeast } from "../hooks/useAuth";
import apiClient from "../apiClient";
import { sortable } from "../lib/tableSort";

const scopeOptions = [
  { label: "read:* (all read access)", value: "read:*" },
  { label: "read:domains", value: "read:domains" },
  { label: "read:users", value: "read:users" },
  { label: "read:applications", value: "read:applications" },
  { label: "read:mail", value: "read:mail" },
  { label: "read:status", value: "read:status" },
  { label: "read:metrics", value: "read:metrics" },
  { label: "write:* (all write access)", value: "write:*" },
  { label: "write:services (restart)", value: "write:services" },
  { label: "write:users (disable/enable)", value: "write:users" },
  { label: "write:domains (suspend)", value: "write:domains" },
  { label: "write:cache (purge)", value: "write:cache" },
  { label: "write:backups (trigger)", value: "write:backups" },
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
  const { t } = useTranslation();
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
  const [historyServer, setHistoryServer] = useState<Server | null>(null);
  const [tagFilter, setTagFilter] = useState<string[]>([]);
  const [envFilter, setEnvFilter] = useState<string | undefined>(undefined);
  const [form] = Form.useForm();
  const canWrite = roleAtLeast("operator");
  const supports = (srv: Server, kw: string) =>
    !srv.capabilities?.length || srv.capabilities.some((c) => c.toLowerCase().includes(kw));
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const actionMut = useServerAction();
  const [restartServer, setRestartServer] = useState<Server | null>(null);
  const [restartForm] = Form.useForm();

  const pollOperation = (id: string, opId: string) => {
    let attempts = 0;
    const timer = setInterval(async () => {
      attempts++;
      try {
        const resp = await apiClient.get<{ status: string; message?: string }>(
          `/admin/servers/${id}/operations/${opId}`,
        );
        const st = resp.data.status;
        if (st === "done" || st === "failed" || attempts > 60) {
          clearInterval(timer);
          if (st === "done") message.success(`Operation ${opId} completed`);
          else if (st === "failed") message.error(`Operation ${opId} failed`);
        }
      } catch {
        clearInterval(timer);
      }
    }, 5000);
  };

  const runAction = async (
    id: string,
    action: string,
    body: Record<string, unknown>,
    okMsg: string,
  ) => {
    try {
      const res = await actionMut.mutateAsync({ id, action, body });
      if (res?.operation_id) {
        message.success(`${okMsg} (op ${res.operation_id})`);
        pollOperation(id, res.operation_id);
      } else {
        message.success(okMsg);
      }
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const bulk = async (label: string, fn: (id: string) => Promise<unknown>) => {
    const ids = [...selectedIds];
    const results = await Promise.allSettled(ids.map((id) => fn(id)));
    const failed = results.filter((r) => r.status === "rejected").length;
    if (failed) message.warning(`${label}: ${ids.length - failed} ok, ${failed} failed`);
    else message.success(`${label}: ${ids.length} server(s)`);
    setSelectedIds([]);
  };

  const submitRestart = async () => {
    if (!restartServer) return;
    let values;
    try {
      values = await restartForm.validateFields();
    } catch {
      return;
    }
    await runAction(restartServer.id, "restart-service", { name: values.name }, `Restarted ${values.name}`);
    setRestartServer(null);
    restartForm.resetFields();
  };

  const tagOptions = useMemo(
    () => Array.from(new Set((servers || []).flatMap((server) => server.tags || [])))
      .sort()
      .map((tag) => ({ label: tag, value: tag })),
    [servers],
  );
  const envOptions = useMemo(
    () => Array.from(new Set((servers || []).map((s) => s.environment).filter(Boolean) as string[]))
      .sort()
      .map((e) => ({ label: e, value: e })),
    [servers],
  );
  const filteredServers = useMemo(
    () => (servers || []).filter((server) =>
      tagFilter.every((tag) => (server.tags || []).includes(tag)) &&
      (!envFilter || server.environment === envFilter)),
    [servers, tagFilter, envFilter],
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
      environment: server.environment,
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
          environment: values.environment,
          insecure_skip_verify: values.insecure_skip_verify,
          token_id: values.token_id,
          ...(values.token_secret
            ? { token_secret: values.token_secret }
            : {}),
        });
        message.success(t("servers.server_updated_successfully"));
      } else {
        await createMut.mutateAsync({
          name: values.name,
          base_url: baseURL,
          token_id: values.token_id,
          token_secret: values.token_secret,
          scopes: values.scopes,
          tags: values.tags,
          environment: values.environment,
          insecure_skip_verify: values.insecure_skip_verify,
        });
        message.success(t("servers.server_enrolled_successfully"));
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
        message.error(t("servers.server_unreachable"));
      } else {
        message.warning(t("servers.server_reachable_but_credentials_invalid"));
      }
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    { title: t("servers.name"), dataIndex: "name", key: "name" },
    { title: t("servers.version"), dataIndex: "version", key: "version" },
    {
      title: t("servers.tags"),
      dataIndex: "tags",
      key: "tags",
      render: (tags: string[]) => (
        <Space wrap size={[4, 4]}>
          {(tags || []).map((tag) => (
            <Tag key={tag} color="blue">{tag}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t("servers.status"),
      dataIndex: "status",
      key: "status",
      render: (s: string) => statusTag(s),
    },
    {
      title: t("servers.credentials"),
      dataIndex: "credential_status",
      key: "credential_status",
      render: (c: string) => credTag(c),
    },
    {
      title: t("servers.scopes"),
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
      title: t("servers.url"),
      dataIndex: "base_url",
      key: "base_url",
      render: (u: string) => (
        <a href={u} target="_blank" rel="noopener noreferrer">
          {u}
        </a>
      ),
    },
    {
      title: t("servers.actions"),
      key: "actions",
      width: 130,
      render: (_: unknown, record: Server) => (
        <RowActions
          actions={[
            {
              key: "check",
              label: t("servers.check"),
              icon: <ReloadOutlined />,
              loading: checkMut.isPending && checkMut.variables === record.id,
              onClick: () => handleCheck(record.id),
            },
            {
              key: "history",
              label: t("servers.history"),
              icon: <HistoryOutlined />,
              onClick: () => setHistoryServer(record),
            },
            ...(canWrite
              ? [
                  {
                    key: "edit",
                    label: t("servers.edit"),
                    icon: <EditOutlined />,
                    onClick: () => openEdit(record),
                  },
                  record.status === "disabled"
                    ? {
                        key: "enable",
                        label: t("servers.enable"),
                        icon: <PlayCircleOutlined />,
                        onClick: () => handleEnable(record.id, record.name),
                      }
                    : {
                        key: "disable",
                        label: t("servers.disable"),
                        icon: <PoweroffOutlined />,
                        onClick: () => handleDisable(record.id, record.name),
                      },
                  ...(supports(record, "restart") || supports(record, "service")
                    ? [{
                        key: "restart-service",
                        label: t("servers.restart_service"),
                        icon: <ReloadOutlined />,
                        onClick: () => setRestartServer(record),
                      }]
                    : []),
                  ...(supports(record, "cache")
                    ? [{
                        key: "purge-cache",
                        label: t("servers.purge_cache"),
                        icon: <ReloadOutlined />,
                        onClick: () => runAction(record.id, "purge-cache", { scope: "all" }, "Cache purged"),
                        confirm: { title: `Purge all cache on "${record.name}"?`, okText: t("servers.purge") },
                      }]
                    : []),
                  ...(supports(record, "backup")
                    ? [{
                        key: "backup",
                        label: t("servers.trigger_backup"),
                        icon: <ReloadOutlined />,
                        onClick: () => runAction(record.id, "backup", {}, "Backup started"),
                        confirm: { title: `Start a backup on "${record.name}"?`, okText: t("servers.start") },
                      }]
                    : []),
                  {
                    key: "delete",
                    label: t("servers.delete"),
                    icon: <DeleteOutlined />,
                    danger: true,
                    onClick: () => handleDelete(record.id, record.name),
                    confirm: {
                      title: `Delete server "${record.name}"?`,
                      description: t("servers.permanently_removes_it_and_its_heartbeat"),
                      okText: t("servers.delete"),
                    },
                  },
                ]
              : []),
          ]}
        />
      ),
    },
  ];

  return (
    <div>
      <Space wrap style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Space wrap className="servers-filters">
          <h3 style={{ margin: 0 }}>Managed Servers</h3>
          <Select
            mode="multiple"
            allowClear
            aria-label={t("servers.filter_servers_by_tags")}
            placeholder={t("servers.filter_by_tags")}
            value={tagFilter}
            options={tagOptions}
            onChange={setTagFilter}
            maxTagCount="responsive"
            style={{ minWidth: 240 }}
          />
          <Select
            allowClear
            aria-label={t("servers.filter_servers_by_environment")}
            placeholder={t("servers.environment")}
            value={envFilter}
            options={envOptions}
            onChange={setEnvFilter}
            style={{ minWidth: 160 }}
          />
        </Space>
        {canWrite ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            Add Server
          </Button>
        ) : null}
      </Space>
      {selectedIds.length > 0 ? (
        <Space style={{ marginBottom: 12 }} wrap>
          <span>{selectedIds.length} selected:</span>
          <Button size="small" onClick={() => bulk("Checked", (id) => checkMut.mutateAsync(id))}>
            Check
          </Button>
          {canWrite ? (
            <>
              <Button size="small" onClick={() => bulk("Disabled", (id) => disableMut.mutateAsync(id))}>
                Disable
              </Button>
              <Button size="small" onClick={() => bulk("Enabled", (id) => enableMut.mutateAsync(id))}>
                Enable
              </Button>
              <Button size="small" onClick={() => bulk("Cache purged", (id) => actionMut.mutateAsync({ id, action: "purge-cache", body: { scope: "all" } }))}>
                Purge cache
              </Button>
              <Button size="small" onClick={() => bulk("Backup started", (id) => actionMut.mutateAsync({ id, action: "backup", body: {} }))}>
                Backup
              </Button>
            </>
          ) : null}
          <Button size="small" type="text" onClick={() => setSelectedIds([])}>
            Clear
          </Button>
        </Space>
      ) : null}
      <Card>
        <Table<Server>
          rowSelection={{
            selectedRowKeys: selectedIds,
            onChange: (keys) => setSelectedIds(keys as string[]),
          }}
          dataSource={filteredServers}
          columns={sortable(columns)}
          rowKey="id"
          loading={isLoading}
          scroll={{ x: "max-content" }}
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
            label={t("servers.server_name")}
            rules={[{ required: true, message: t("servers.enter_a_display_name") }]}
          >
            <Input placeholder={t("servers.e_g_panel_01_example_com")} />
          </Form.Item>
          <Form.Item
            name="panel_host"
            label={t("servers.server_hostname")}
            rules={[
              { required: true, message: t("servers.enter_the_panel_hostname") },
              {
                pattern: /^[a-zA-Z0-9.-]+$/,
                message: t("servers.enter_only_the_hostname_without_https"),
              },
            ]}
            extra="Sounder connects to the panel at https://hostname:8443."
          >
            <Input addonBefore="https://" addonAfter=":8443" placeholder="panel-01.example.com" />
          </Form.Item>
          <Form.Item
            name="token_id"
            label={t("servers.automation_token_id")}
            rules={[{ required: true, message: t("servers.enter_the_token_id_ulid") }]}
          >
            <Input placeholder="01J..." />
          </Form.Item>
          <Form.Item
            name="token_secret"
            label={t("servers.automation_token_secret")}
            rules={
              editingServer
                ? []
                : [{ required: true, message: t("servers.enter_the_token_secret") }]
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
          <Form.Item name="scopes" label={t("servers.scopes")}>
            <Select
              mode="multiple"
              placeholder={t("servers.leave_empty_for_read_all_read")}
              options={scopeOptions}
            />
          </Form.Item>
          <Form.Item
            name="tags"
            label={t("servers.tags")}
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
              placeholder={t("servers.production_eu_west_customer_a")}
            />
          </Form.Item>
          <Form.Item name="environment" label={t("servers.environment")}>
            <AutoComplete
              allowClear
              options={envOptions.length ? envOptions : [
                { label: "production", value: "production" },
                { label: "staging", value: "staging" },
                { label: "development", value: "development" },
              ]}
              placeholder="production"
              filterOption={(input, option) =>
                (option?.value ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
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
      <ServerHistoryDrawer
        server={historyServer}
        onClose={() => setHistoryServer(null)}
      />
      <Modal
        title={restartServer ? `Restart a service on ${restartServer.name}` : "Restart service"}
        open={!!restartServer}
        onOk={submitRestart}
        confirmLoading={actionMut.isPending}
        onCancel={() => {
          setRestartServer(null);
          restartForm.resetFields();
        }}
        okText={t("servers.restart")}
      >
        <Form form={restartForm} layout="vertical">
          <Form.Item name="name" label={t("servers.service_name")} rules={[{ required: true, message: t("servers.enter_a_service_name") }]}>
            <Input placeholder="nginx" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
