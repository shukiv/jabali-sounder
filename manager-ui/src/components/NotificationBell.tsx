import { Badge, Button, Dropdown, List, Typography, Empty, Tag } from "antd";
import { BellOutlined } from "@ant-design/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import apiClient from "../apiClient";

const { Text } = Typography;

interface Notification {
  id: string;
  kind: string;
  server_id: string;
  server_name: string;
  metric: string;
  value: number;
  threshold: number;
  message: string;
  created_at: string;
  read: boolean;
  resolved: boolean;
}

interface NotificationList {
  data: Notification[];
  unread_count: number;
}

// NotificationBell shows in-app fleet alerts (SND-18) with an unread badge and a
// dropdown; opening an item marks it read and jumps to Monitor. Polls every 30s.
export default function NotificationBell() {
  const qc = useQueryClient();
  const nav = useNavigate();
  const { data } = useQuery({
    queryKey: ["notifications"],
    queryFn: async () =>
      (await apiClient.get<NotificationList>("/admin/notifications")).data,
    refetchInterval: 30_000,
  });

  const rows = data?.data || [];
  const unread = data?.unread_count || 0;

  const invalidate = () => qc.invalidateQueries({ queryKey: ["notifications"] });

  const markRead = async (id: string) => {
    try {
      await apiClient.post(`/admin/notifications/${id}/read`);
      invalidate();
    } catch {
      /* non-fatal */
    }
  };

  const markAll = async () => {
    try {
      await apiClient.post("/admin/notifications/read-all");
      invalidate();
    } catch {
      /* non-fatal */
    }
  };

  const openItem = (n: Notification) => {
    if (!n.read) markRead(n.id);
    nav("/monitor");
  };

  const panel = (
    <div
      style={{
        width: 360,
        maxHeight: 440,
        overflowY: "auto",
        background: "var(--ant-color-bg-elevated, #fff)",
        borderRadius: 8,
        boxShadow: "0 6px 24px rgba(0,0,0,0.18)",
      }}
      onClick={(e) => e.stopPropagation()}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "10px 14px",
          borderBottom: "1px solid rgba(0,0,0,0.06)",
        }}
      >
        <Text strong>Notifications</Text>
        {unread > 0 ? (
          <Button type="link" size="small" onClick={markAll}>
            Mark all read
          </Button>
        ) : null}
      </div>
      {rows.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="No notifications"
          style={{ padding: 24 }}
        />
      ) : (
        <List
          size="small"
          dataSource={rows}
          renderItem={(n) => (
            <List.Item
              style={{
                cursor: "pointer",
                padding: "10px 14px",
                background: n.read ? undefined : "rgba(24,144,255,0.06)",
              }}
              onClick={() => openItem(n)}
            >
              <List.Item.Meta
                title={
                  <span>
                    {n.server_name}{" "}
                    {n.resolved ? (
                      <Tag color="success">resolved</Tag>
                    ) : (
                      <Tag color="error">active</Tag>
                    )}
                  </span>
                }
                description={
                  <>
                    <Text type="secondary">{n.message}</Text>
                    <br />
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {new Date(n.created_at).toLocaleString()}
                    </Text>
                  </>
                }
              />
            </List.Item>
          )}
        />
      )}
    </div>
  );

  return (
    <Dropdown
      popupRender={() => panel}
      trigger={["click"]}
      placement="bottomRight"
    >
      <Badge count={unread} size="small" offset={[-2, 4]}>
        <Button type="text" icon={<BellOutlined />} title="Notifications" />
      </Badge>
    </Dropdown>
  );
}
