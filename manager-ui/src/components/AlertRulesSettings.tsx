import { useState } from "react";
import { Card, Table, Button, Modal, Form, InputNumber, Select, Switch, Tag, Typography, App } from "antd";
import { AlertOutlined } from "@ant-design/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";
import { roleAtLeast } from "../hooks/useAuth";
import { sortable } from "../lib/tableSort";

const { Title, Paragraph } = Typography;

interface AlertRule {
  metric: string;
  threshold: number;
  duration_seconds: number;
  severity: "info" | "warning" | "critical";
  enabled: boolean;
}

const METRIC_LABEL: Record<string, string> = {
  cpu: "CPU %",
  ram: "RAM %",
  disk: "Disk %",
  load1: "Load (1m)",
  service_down: "Service down",
};

const SEV_COLOR: Record<string, string> = {
  info: "blue",
  warning: "gold",
  critical: "red",
};

export function SeverityTag({ severity }: { severity: string }) {
  return <Tag color={SEV_COLOR[severity] || "default"}>{severity}</Tag>;
}

// AlertRulesSettings edits the fleet-wide metric thresholds (SND-20). Reads are
// open; editing requires operator+.
export default function AlertRulesSettings() {
  const qc = useQueryClient();
  const { message } = App.useApp();
  const canEdit = roleAtLeast("operator");
  const { data, isLoading } = useQuery({
    queryKey: ["alert-rules"],
    queryFn: async () => (await apiClient.get<{ data: AlertRule[] }>("/admin/alert-rules")).data.data,
  });
  const [editing, setEditing] = useState<AlertRule | null>(null);
  const [busy, setBusy] = useState(false);
  const [form] = Form.useForm();

  const openEdit = (r: AlertRule) => {
    setEditing(r);
    form.setFieldsValue(r);
  };

  const save = async () => {
    if (!editing) return;
    let values;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setBusy(true);
    try {
      await apiClient.put(`/admin/alert-rules/${editing.metric}`, {
        threshold: editing.metric === "service_down" ? 0 : values.threshold,
        duration_seconds: values.duration_seconds,
        severity: values.severity,
        enabled: values.enabled,
      });
      message.success(`${METRIC_LABEL[editing.metric] || editing.metric} rule updated`);
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["alert-rules"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  const columns = [
    { title: "Metric", dataIndex: "metric", key: "metric", render: (m: string) => METRIC_LABEL[m] || m },
    {
      title: "Fires when",
      key: "cond",
      render: (_: unknown, r: AlertRule) =>
        r.metric === "service_down"
          ? `Any service not healthy for ${r.duration_seconds}s`
          : r.metric === "load1"
            ? `> ${r.threshold} for ${r.duration_seconds}s`
            : `> ${r.threshold}% for ${r.duration_seconds}s`,
    },
    { title: "Severity", dataIndex: "severity", key: "severity", render: (s: string) => <SeverityTag severity={s} /> },
    {
      title: "Enabled",
      dataIndex: "enabled",
      key: "enabled",
      render: (e: boolean) => (e ? <Tag color="green">on</Tag> : <Tag>off</Tag>),
    },
    ...(canEdit
      ? [{
          title: "",
          key: "actions",
          render: (_: unknown, r: AlertRule) => (
            <Button size="small" onClick={() => openEdit(r)}>
              Edit
            </Button>
          ),
        }]
      : []),
  ];

  return (
    <Card style={{ marginTop: 16 }}>
      <Title level={4}>
        <AlertOutlined /> Alert rules
      </Title>
      <Paragraph type="secondary">
        Fleet-wide thresholds on polled metrics. A breach sustained for the given
        duration opens an incident and notifies the configured channels.
      </Paragraph>
      <Table<AlertRule>
        scroll={{ x: "max-content" }}
        dataSource={data || []}
        columns={sortable(columns)}
        rowKey="metric"
        loading={isLoading}
        pagination={false}
        size="small"
      />

      <Modal
        title={editing ? `Edit ${METRIC_LABEL[editing.metric] || editing.metric} rule` : ""}
        open={!!editing}
        onOk={save}
        confirmLoading={busy}
        onCancel={() => setEditing(null)}
        okText="Save"
      >
        <Form form={form} layout="vertical">
          {editing?.metric === "service_down" ? (
            <Paragraph type="secondary" style={{ marginTop: 0 }}>
              Fires when any managed-server service reports a non-healthy status
              (stopped, failed, or degraded) for the duration below.
            </Paragraph>
          ) : (
            <Form.Item name="threshold" label="Threshold" rules={[{ required: true }]}>
              <InputNumber min={0} style={{ width: "100%" }} />
            </Form.Item>
          )}
          <Form.Item name="duration_seconds" label="Sustained for (seconds)" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="severity" label="Severity" rules={[{ required: true }]}>
            <Select
              options={[
                { value: "info", label: "info" },
                { value: "warning", label: "warning" },
                { value: "critical", label: "critical" },
              ]}
            />
          </Form.Item>
          <Form.Item name="enabled" label="Enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
