import { Card, Table, Tag, Typography, Space, Button } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { useDashboard } from "../hooks/useDashboard";
import { useNavigate } from "react-router";
import type { DashboardEntry } from "../types";

const { Title } = Typography;

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

export default function Dashboard() {
  const { data, isLoading, refetch, isFetching } = useDashboard();
  const navigate = useNavigate();

  const columns = [
    { title: "Name", dataIndex: "name", key: "name" },
    { title: "Version", dataIndex: "version", key: "version" },
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
      title: "URL",
      dataIndex: "base_url",
      key: "base_url",
      render: (u: string) => (
        <a href={u} target="_blank" rel="noopener noreferrer">
          {u}
        </a>
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Title level={3} style={{ margin: 0 }}>Dashboard</Title>
        <Button
          icon={<ReloadOutlined />}
          loading={isFetching}
          onClick={() => refetch()}
        >
          Refresh
        </Button>
      </Space>
      <Card>
        <Table<DashboardEntry>
          dataSource={data || []}
          columns={columns}
          rowKey="id"
          loading={isLoading}
          pagination={false}
          onRow={() => ({
            onClick: () => navigate("/servers"),
            style: { cursor: "pointer" },
          })}
        />
      </Card>
    </div>
  );
}
