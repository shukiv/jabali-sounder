import { Drawer, Statistic, List, Badge, Tag, Empty, Spin, Row, Col } from "antd";
import { useServerHeartbeats } from "../hooks/useServers";
import type { Server } from "../types";

interface Props {
  server: Server | null;
  onClose: () => void;
}

// ServerHistoryDrawer shows a server's recent health-check history (recorded by
// the background poller) plus an uptime summary over the returned window (M1).
export default function ServerHistoryDrawer({ server, onClose }: Props) {
  const { data, isLoading } = useServerHeartbeats(server?.id ?? null);
  const uptimePct = data ? Math.round(data.uptime.ratio * 1000) / 10 : 0;

  return (
    <Drawer
      title={server ? `Health history — ${server.name}` : "Health history"}
      open={!!server}
      onClose={onClose}
      width={480}
      destroyOnClose
    >
      {isLoading ? (
        <Spin />
      ) : !data || data.total === 0 ? (
        <Empty description="No health checks recorded yet" />
      ) : (
        <>
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={12}>
              <Statistic
                title={`Uptime (last ${data.uptime.total})`}
                value={uptimePct}
                precision={1}
                suffix="%"
                valueStyle={{
                  color: uptimePct >= 99 ? "#3f8600" : uptimePct >= 90 ? "#d48806" : "#cf1322",
                }}
              />
            </Col>
            <Col span={12}>
              <Statistic
                title="Healthy checks"
                value={data.uptime.healthy}
                suffix={`/ ${data.uptime.total}`}
              />
            </Col>
          </Row>
          <List
            size="small"
            dataSource={data.data}
            renderItem={(hb) => (
              <List.Item>
                <Badge
                  status={hb.healthy ? "success" : "error"}
                  text={hb.healthy ? "healthy" : "unhealthy"}
                />
                <span style={{ color: "#888", fontSize: 12 }}>
                  {new Date(hb.checked_at).toLocaleString()}
                </span>
                {hb.version ? <Tag>{hb.version}</Tag> : null}
              </List.Item>
            )}
          />
        </>
      )}
    </Drawer>
  );
}
