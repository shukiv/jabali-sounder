import { Card, Table, Typography, Tag, Button, Space } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";

const { Title, Text } = Typography;

interface BackupRow {
  id: string;
  server_id: string;
  server_name: string;
  status: string;
  message: string;
  triggered_by: string;
  started_at: string;
  finished_at: string | null;
}

const STATUS_COLOR: Record<string, string> = {
  succeeded: "green",
  running: "blue",
  pending: "gold",
  failed: "red",
};

// Backups shows the history of backup operations Sounder triggered and tracked
// to completion (SND-27). Panels expose no backup listing, so this is Sounder's
// own record. Auto-refreshes so in-progress runs update.
export default function Backups() {
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ["backups"],
    queryFn: async () => (await apiClient.get<{ data: BackupRow[] }>("/admin/backups")).data.data,
    refetchInterval: 20_000,
  });

  const columns = [
    { title: "Server", dataIndex: "server_name", key: "server_name" },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (s: string) => <Tag color={STATUS_COLOR[s] || "default"}>{s}</Tag>,
    },
    {
      title: "Started",
      dataIndex: "started_at",
      key: "started_at",
      render: (t: string) => new Date(t).toLocaleString(),
    },
    {
      title: "Finished",
      dataIndex: "finished_at",
      key: "finished_at",
      render: (t: string | null) => (t ? new Date(t).toLocaleString() : <Text type="secondary">—</Text>),
    },
    {
      title: "Triggered by",
      dataIndex: "triggered_by",
      key: "triggered_by",
      render: (a: string) => a || <Text type="secondary">—</Text>,
    },
    {
      title: "Detail",
      dataIndex: "message",
      key: "message",
      render: (m: string) => (m ? <Text type="secondary">{m}</Text> : null),
    },
  ];

  return (
    <div>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Backups</Title>
        <Space>
          <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => refetch()}>
            Refresh
          </Button>
        </Space>
      </div>
      <Card>
        <Text type="secondary">
          Backup runs triggered from Servers → Backup, tracked to completion by
          the poller. Servers with no recent successful backup raise a
          notification.
        </Text>
        <Table<BackupRow>
          scroll={{ x: "max-content" }}
          style={{ marginTop: 16 }}
          dataSource={data || []}
          columns={columns}
          rowKey="id"
          loading={isLoading}
          size="small"
          pagination={{ pageSize: 20, showSizeChanger: false }}
        />
      </Card>
    </div>
  );
}
