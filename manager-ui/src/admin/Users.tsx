import { useState } from "react";
import { Card, Table, Tag, Input, Space, Typography, Button, Popconfirm, App } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { useUsers } from "../hooks/useInventory";
import { useServerAction } from "../hooks/useServers";
import { roleAtLeast } from "../hooks/useAuth";
import type { UserRow } from "../hooks/useInventory";
import { sortable } from "../lib/tableSort";

const { Title } = Typography;

export default function Users() {
  const { data: users, isLoading } = useUsers();
  const { message } = App.useApp();
  const canWrite = roleAtLeast("operator");
  const actionMut = useServerAction();
  const userAction = async (r: UserRow, enabled: boolean) => {
    try {
      await actionMut.mutateAsync({ id: r.server_id, action: "user", body: { user_id: r.id, enabled } });
      message.success(enabled ? "User enabled" : "User disabled");
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };
  const [search, setSearch] = useState("");

  const filtered = (users || []).filter((u) =>
    u.email.toLowerCase().includes(search.toLowerCase()) ||
    (u.username || "").toLowerCase().includes(search.toLowerCase()),
  );

  const columns = [
    { title: "Email", dataIndex: "email", key: "email" },
    { title: "Username", dataIndex: "username", key: "username", render: (u: string) => u || "—" },
    {
      title: "Admin",
      dataIndex: "is_admin",
      key: "is_admin",
      render: (admin: boolean) => admin ? <Tag color="gold">admin</Tag> : null,
    },
    { title: "Package", dataIndex: "package_id", key: "package_id", render: (p: string) => p ? p.slice(0, 10) + "…" : "—" },
    {
      title: "Server",
      dataIndex: "server_name",
      key: "server_name",
      render: (name: string) => <Tag color="blue">{name}</Tag>,
    },
  ];

  const userActionsCol = {
    title: "Actions",
    key: "actions",
    render: (_: unknown, r: UserRow) => (
      <Space>
        <Popconfirm
          title={`Disable ${r.email || r.username}?`}
          okText="Disable"
          okButtonProps={{ danger: true }}
          onConfirm={() => userAction(r, false)}
        >
          <Button danger size="small">Disable</Button>
        </Popconfirm>
        <Button size="small" onClick={() => userAction(r, true)}>Enable</Button>
      </Space>
    ),
  };

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>Users</Title>
      <Card>
        <Space wrap style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
          <Input
            placeholder="Search by email or username…"
            aria-label="Search users"
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: "100%", maxWidth: 400 }}
            allowClear
          />
          <span style={{ color: "#888" }}>{filtered.length} of {users?.length || 0}</span>
        </Space>
        <Table<UserRow>
          dataSource={filtered}
          columns={sortable(canWrite ? [...columns, userActionsCol] : columns)}
          rowKey={(r) => r.server_id + ":" + r.id}
          loading={isLoading}
          pagination={{ pageSize: 50, showSizeChanger: true }}
          scroll={{ x: "max-content" }}
        />
      </Card>
    </div>
  );
}
