import { useState } from "react";
import { Modal, Input, List, Tag, Typography, Empty, Spin } from "antd";
import {
  CloudServerOutlined,
  GlobalOutlined,
  TeamOutlined,
} from "@ant-design/icons";
import { useNavigate } from "react-router";
import { useServers } from "../hooks/useServers";
import { useDomains, useUsers } from "../hooks/useInventory";

const { Text } = Typography;
const MAX_PER_GROUP = 6;

interface Props {
  open: boolean;
  onClose: () => void;
}

interface Hit {
  key: string;
  icon: React.ReactNode;
  primary: string;
  secondary: string;
  to: string;
}

// GlobalSearch is a command-palette that searches enrolled servers plus the
// cross-server domain and user inventories, and jumps to the relevant page (M4).
// Mounted only while open so the inventory fan-out runs on demand.
export default function GlobalSearch({ open, onClose }: Props) {
  const nav = useNavigate();
  const [q, setQ] = useState("");
  const { data: servers } = useServers();
  const { data: domains, isLoading: dLoading } = useDomains();
  const { data: users, isLoading: uLoading } = useUsers();

  const ql = q.trim().toLowerCase();
  const has = (...vals: (string | undefined)[]) =>
    ql !== "" && vals.some((v) => (v ?? "").toLowerCase().includes(ql));

  const hits: Hit[] = ql
    ? [
        ...(servers || [])
          .filter((s) => has(s.name, s.base_url, ...(s.tags || [])))
          .slice(0, MAX_PER_GROUP)
          .map((s) => ({
            key: "srv:" + s.id,
            icon: <CloudServerOutlined />,
            primary: s.name,
            secondary: s.base_url,
            to: "/servers",
          })),
        ...(domains || [])
          .filter((d) => has(d.name))
          .slice(0, MAX_PER_GROUP)
          .map((d) => ({
            key: "dom:" + d.id,
            icon: <GlobalOutlined />,
            primary: d.name,
            secondary: `domain · ${d.server_name}`,
            to: "/domains",
          })),
        ...(users || [])
          .filter((u) => has(u.email, u.username))
          .slice(0, MAX_PER_GROUP)
          .map((u) => ({
            key: "usr:" + u.id,
            icon: <TeamOutlined />,
            primary: u.email || u.username,
            secondary: `user · ${u.server_name}`,
            to: "/users",
          })),
      ]
    : [];

  const go = (to: string) => {
    setQ("");
    onClose();
    nav(to);
  };

  return (
    <Modal
      open={open}
      onCancel={() => {
        setQ("");
        onClose();
      }}
      footer={null}
      destroyOnClose
      styles={{ body: { paddingTop: 8 } }}
    >
      <Input
        autoFocus
        size="large"
        placeholder="Search servers, domains, users…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        allowClear
      />
      <div style={{ marginTop: 12, maxHeight: 420, overflowY: "auto" }}>
        {ql === "" ? (
          <Text type="secondary">Type to search across the fleet.</Text>
        ) : dLoading || uLoading ? (
          <Spin style={{ display: "block", margin: "24px auto" }} />
        ) : hits.length === 0 ? (
          <Empty description="No matches" />
        ) : (
          <List
            size="small"
            dataSource={hits}
            renderItem={(h) => (
              <List.Item
                style={{ cursor: "pointer" }}
                onClick={() => go(h.to)}
              >
                <List.Item.Meta
                  avatar={h.icon}
                  title={h.primary}
                  description={<Text type="secondary">{h.secondary}</Text>}
                />
                <Tag>{h.to.replace("/", "")}</Tag>
              </List.Item>
            )}
          />
        )}
      </div>
    </Modal>
  );
}
