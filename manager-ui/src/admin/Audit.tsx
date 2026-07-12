import { useState } from "react";
import {
  Card, Table, Typography, Select, Input, Button, Space, Tag, App,
} from "antd";
import { DownloadOutlined, ReloadOutlined } from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";
import { desktopBridge } from "../lib/desktop";

const { Title, Text } = Typography;

interface AuditRow {
  id: string;
  event: string;
  actor: string;
  actor_id: string;
  server_id: string;
  server_name: string;
  source_ip: string;
  request_id: string;
  created_at: string;
}

const DAYS_OPTIONS = [
  { value: "1", label: "Last 24h" },
  { value: "7", label: "Last 7 days" },
  { value: "30", label: "Last 30 days" },
  { value: "", label: "All time" },
];

// Audit shows the persisted trail of privileged mutations (SND-24) with filters
// and a CSV export. Read-only.
export default function Audit() {
  const { message } = App.useApp();
  const [days, setDays] = useState("7");
  const [actor, setActor] = useState("");
  const [event, setEvent] = useState("");
  const [exporting, setExporting] = useState(false);

  const params = () => {
    const p = new URLSearchParams();
    if (days) p.set("since", days);
    if (actor.trim()) p.set("actor", actor.trim());
    if (event.trim()) p.set("event", event.trim());
    return p;
  };

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ["audit", days, actor, event],
    queryFn: async () => {
      const p = params();
      p.set("limit", "500");
      return (await apiClient.get<{ data: AuditRow[]; total: number }>(`/admin/audit?${p}`)).data;
    },
  });

  const exportCSV = async () => {
    setExporting(true);
    try {
      const p = params();
      const resp = await apiClient.get(`/admin/audit.csv?${p}`, { responseType: "blob" });
      const text = await (resp.data as Blob).text();
      const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
      const filename = `jabali-sounder-audit-${stamp}.csv`;
      const bridge = desktopBridge();
      if (bridge?.SaveFile) {
        const saved = await bridge.SaveFile(filename, text);
        if (saved) message.success(`Exported to ${saved}`);
        return;
      }
      const url = URL.createObjectURL(new Blob([text], { type: "text/csv" }));
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      message.success("Audit CSV exported");
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setExporting(false);
    }
  };

  const columns = [
    {
      title: "Time",
      dataIndex: "created_at",
      key: "created_at",
      render: (t: string) => new Date(t).toLocaleString(),
      width: 190,
    },
    { title: "Event", dataIndex: "event", key: "event", render: (e: string) => <Tag>{e}</Tag> },
    { title: "Actor", dataIndex: "actor", key: "actor", render: (a: string) => a || <Text type="secondary">—</Text> },
    {
      title: "Server",
      key: "server",
      render: (_: unknown, r: AuditRow) => r.server_name || <Text type="secondary">—</Text>,
    },
    { title: "Source IP", dataIndex: "source_ip", key: "source_ip" },
    {
      title: "Request",
      dataIndex: "request_id",
      key: "request_id",
      render: (r: string) => (r ? <Text code style={{ fontSize: 11 }}>{r.slice(0, 8)}</Text> : null),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Audit log</Title>
        <Button icon={<DownloadOutlined />} loading={exporting} onClick={exportCSV}>
          Export CSV
        </Button>
      </div>
      <Card>
        <Space wrap style={{ marginBottom: 16 }}>
          <Select value={days} onChange={setDays} options={DAYS_OPTIONS} style={{ width: 140 }} />
          <Input
            placeholder="Filter by actor"
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            allowClear
            style={{ width: 180 }}
          />
          <Input
            placeholder="Filter by event (e.g. server.delete)"
            value={event}
            onChange={(e) => setEvent(e.target.value)}
            allowClear
            style={{ width: 240 }}
          />
          <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => refetch()}>
            Refresh
          </Button>
        </Space>
        <Table<AuditRow>
          dataSource={data?.data || []}
          columns={columns}
          rowKey="id"
          loading={isLoading}
          size="small"
          pagination={{ pageSize: 20, showSizeChanger: false }}
          footer={() => <Text type="secondary">{data?.total ?? 0} events</Text>}
        />
      </Card>
    </div>
  );
}
