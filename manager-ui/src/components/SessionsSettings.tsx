import { Card, Table, Button, Tag, Typography, App, Popconfirm } from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";

const { Title, Paragraph } = Typography;

interface Session {
  id: string;
  user_agent: string;
  ip: string;
  created_at: string;
  last_seen_at: string;
  is_current: boolean;
}

interface SessionList {
  data: Session[];
}

// SessionsSettings lists the operator's active logins and lets them revoke
// others (or the current one, which logs out) (M3).
export default function SessionsSettings() {
  const qc = useQueryClient();
  const { message } = App.useApp();
  const { data, isLoading } = useQuery({
    queryKey: ["sessions"],
    queryFn: async () => (await apiClient.get<SessionList>("/auth/sessions")).data.data,
  });

  const revoke = async (s: Session) => {
    try {
      await apiClient.delete(`/auth/sessions/${s.id}`);
      if (s.is_current) {
        message.info("Session revoked — logging out");
        window.location.reload();
        return;
      }
      message.success("Session revoked");
      qc.invalidateQueries({ queryKey: ["sessions"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    {
      title: "Device",
      dataIndex: "user_agent",
      key: "user_agent",
      render: (ua: string, r: Session) => (
        <span>
          {ua || "unknown"} {r.is_current ? <Tag color="green">this device</Tag> : null}
        </span>
      ),
    },
    { title: "IP", dataIndex: "ip", key: "ip" },
    {
      title: "Last seen",
      dataIndex: "last_seen_at",
      key: "last_seen_at",
      render: (t: string) => new Date(t).toLocaleString(),
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, r: Session) => (
        <Popconfirm
          title={r.is_current ? "Revoke this session and log out?" : "Revoke this session?"}
          okText="Revoke"
          okButtonProps={{ danger: true }}
          onConfirm={() => revoke(r)}
        >
          <Button danger size="small">
            Revoke
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Card style={{ marginTop: 16 }}>
      <Title level={4}>Active sessions</Title>
      <Paragraph type="secondary">
        Devices currently signed in with your account. Revoke any you don't
        recognize.
      </Paragraph>
      <Table<Session>
        dataSource={data || []}
        columns={columns}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="small"
        scroll={{ x: "max-content" }}
      />
    </Card>
  );
}
