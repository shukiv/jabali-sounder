import { useState } from "react";
import { Card, Table, Tag, Input, Space, Typography } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { useUsers } from "../hooks/useInventory";
import type { UserRow } from "../hooks/useInventory";

const { Title } = Typography;

export default function Users() {
  const { data: users, isLoading } = useUsers();
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

  return (
    <div>
      <Title level={3} style={{ marginBottom: 16 }}>Users</Title>
      <Card>
        <Space style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
          <Input
            placeholder="Search by email or username…"
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: 400 }}
            allowClear
          />
          <span style={{ color: "#888" }}>{filtered.length} of {users?.length || 0}</span>
        </Space>
        <Table<UserRow>
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
