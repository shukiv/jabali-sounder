import { Badge, Button, Dropdown, List, Typography, Empty, Tag, Space, App } from "antd";
import {
  BellOutlined, CheckOutlined, ClockCircleOutlined, StopOutlined,
} from "@ant-design/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import apiClient from "../apiClient";
import { roleAtLeast } from "../hooks/useAuth";

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
  severity: "info" | "warning" | "critical";
  created_at: string;
  read: boolean;
  resolved: boolean;
  acked: boolean;
  acked_by: string;
}

interface NotificationList {
  data: Notification[];
  unread_count: number;
}

const SEV_COLOR: Record<string, string> = { info: "blue", warning: "gold", critical: "red" };

// NotificationBell shows in-app fleet incidents (SND-18/21) with an unread
// badge and a dropdown; incidents can be acknowledged, snoozed, or muted.
// Polls every 30s.
export default function NotificationBell() {
  const qc = useQueryClient();
  const nav = useNavigate();
  const { message } = App.useApp();
  const canAct = roleAtLeast("operator");
  const { data } = useQuery({
    queryKey: ["notifications"],
    queryFn: async () => (await apiClient.get<NotificationList>("/admin/notifications")).data,
    refetchInterval: 30_000,
  });

  const rows = data?.data || [];
  const unread = data?.unread_count || 0;
  const invalidate = () => qc.invalidateQueries({ queryKey: ["notifications"] });

  const markRead = async (id: string) => {
    try {
      await apiClient.post(`/admin/notifications/${id}/read`);
      invalidate();
    } catch { /* non-fatal */ }
  };
  const markAll = async () => {
    try {
      await apiClient.post("/admin/notifications/read-all");
      invalidate();
    } catch { /* non-fatal */ }
  };
  const ack = async (id: string) => {
    try {
      await apiClient.post(`/admin/notifications/${id}/ack`);
      message.success("Acknowledged");
      invalidate();
    } catch (err) { if (err instanceof Error) message.error(err.message); }
  };
  const snooze = async (id: string) => {
    try {
      await apiClient.post(`/admin/notifications/${id}/snooze`, { minutes: 60 });
      message.success("Snoozed for 1h");
      invalidate();
    } catch (err) { if (err instanceof Error) message.error(err.message); }
  };
  const mute = async (n: Notification) => {
    try {
      await apiClient.post("/admin/muted", { server_id: n.server_id, kind: n.kind });
      message.success(`Muted ${n.kind} for ${n.server_name}`);
      invalidate();
    } catch (err) { if (err instanceof Error) message.error(err.message); }
  };

  const openItem = (n: Notification) => {
    if (!n.read) markRead(n.id);
    nav("/monitor");
  };

  const panel = (
    <div
      style={{
        width: "min(400px, calc(100vw - 16px))", maxHeight: "min(460px, 70vh)", overflowY: "auto",
        background: "var(--ant-color-bg-elevated, #fff)", borderRadius: 8,
        boxShadow: "0 6px 24px rgba(0,0,0,0.18)",
      }}
      onClick={(e) => e.stopPropagation()}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 14px", borderBottom: "1px solid rgba(0,0,0,0.06)" }}>
        <Text strong>Notifications</Text>
        {unread > 0 ? <Button type="link" size="small" onClick={markAll}>Mark all read</Button> : null}
      </div>
      {rows.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No notifications" style={{ padding: 24 }} />
      ) : (
        <List
          size="small"
          dataSource={rows}
          renderItem={(n) => (
            <List.Item
              role="button"
              tabIndex={0}
              aria-label={`${n.severity} alert on ${n.server_name}: ${n.message} — open Monitor`}
              style={{ cursor: "pointer", padding: "10px 14px", background: n.read ? undefined : "rgba(24,144,255,0.06)", display: "block" }}
              onClick={() => openItem(n)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  openItem(n);
                }
              }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <Text strong>{n.server_name}</Text>
                <Space size={4}>
                  <Tag color={SEV_COLOR[n.severity] || "default"}>{n.severity}</Tag>
                  {n.resolved ? <Tag color="success">resolved</Tag> : n.acked ? <Tag color="blue">acked</Tag> : <Tag color="error">active</Tag>}
                </Space>
              </div>
              <Text type="secondary">{n.message}</Text>
              <br />
              <Text type="secondary" style={{ fontSize: 12 }}>{new Date(n.created_at).toLocaleString()}</Text>
              {canAct && !n.resolved ? (
                <div style={{ marginTop: 6 }} onClick={(e) => e.stopPropagation()}>
                  <Space size={4}>
                    {!n.acked ? (
                      <Button size="small" icon={<CheckOutlined />} onClick={() => ack(n.id)}>Ack</Button>
                    ) : null}
                    <Button size="small" icon={<ClockCircleOutlined />} onClick={() => snooze(n.id)}>Snooze 1h</Button>
                    <Button size="small" icon={<StopOutlined />} onClick={() => mute(n)}>Mute</Button>
                  </Space>
                </div>
              ) : null}
            </List.Item>
          )}
        />
      )}
    </div>
  );

  return (
    <Dropdown popupRender={() => panel} trigger={["click"]} placement="bottomRight">
      <Badge count={unread} size="small" offset={[-2, 4]}>
        <Button type="text" icon={<BellOutlined />} title="Notifications" />
      </Badge>
    </Dropdown>
  );
}
