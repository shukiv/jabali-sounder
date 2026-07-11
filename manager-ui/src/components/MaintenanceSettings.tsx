import { useState } from "react";
import {
  Card, Table, Button, Modal, Form, Input, Select, DatePicker, Typography, App, Popconfirm, Tag,
} from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import dayjs, { Dayjs } from "dayjs";
import apiClient from "../apiClient";
import { useServers } from "../hooks/useServers";

const { Title, Paragraph } = Typography;
const { RangePicker } = DatePicker;

interface Window {
  id: string;
  scope_type: string;
  scope_value: string;
  starts_at: string;
  ends_at: string;
  reason: string;
  created_by: string;
}

// MaintenanceSettings schedules alert-suppression windows (SND-22, operator+).
export default function MaintenanceSettings() {
  const qc = useQueryClient();
  const { message } = App.useApp();
  const { data: servers } = useServers();
  const { data, isLoading } = useQuery({
    queryKey: ["maintenance"],
    queryFn: async () => (await apiClient.get<{ data: Window[] }>("/admin/maintenance")).data.data,
  });
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [scope, setScope] = useState("global");
  const [form] = Form.useForm();

  const environments = Array.from(
    new Set((servers || []).map((s) => s.environment).filter((e): e is string => !!e)),
  );

  const invalidate = () => qc.invalidateQueries({ queryKey: ["maintenance"] });

  const create = async () => {
    let values;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const [start, end] = values.range as [Dayjs, Dayjs];
    setBusy(true);
    try {
      await apiClient.post("/admin/maintenance", {
        scope_type: values.scope_type,
        scope_value: values.scope_type === "global" ? "" : values.scope_value,
        starts_at: start.toISOString(),
        ends_at: end.toISOString(),
        reason: values.reason || "",
      });
      message.success("Maintenance window scheduled");
      setOpen(false);
      form.resetFields();
      invalidate();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await apiClient.delete(`/admin/maintenance/${id}`);
      message.success("Window removed");
      invalidate();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const now = dayjs();
  const columns = [
    {
      title: "Scope",
      key: "scope",
      render: (_: unknown, r: Window) =>
        r.scope_type === "global" ? <Tag color="volcano">whole fleet</Tag> : (
          <span>
            <Tag>{r.scope_type}</Tag>
            {r.scope_value}
          </span>
        ),
    },
    {
      title: "Window",
      key: "window",
      render: (_: unknown, r: Window) => {
        const active = now.isAfter(dayjs(r.starts_at)) && now.isBefore(dayjs(r.ends_at));
        return (
          <span>
            {dayjs(r.starts_at).format("MMM D HH:mm")} → {dayjs(r.ends_at).format("MMM D HH:mm")}{" "}
            {active ? <Tag color="green">active</Tag> : null}
          </span>
        );
      },
    },
    { title: "Reason", dataIndex: "reason", key: "reason" },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, r: Window) => (
        <Popconfirm title="Remove window?" okText="Remove" okButtonProps={{ danger: true }} onConfirm={() => remove(r.id)}>
          <Button size="small" danger>
            Remove
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Card style={{ marginTop: 16 }}>
      <Title level={4}>Maintenance windows</Title>
      <Paragraph type="secondary">
        Suppress alerts for a server, an environment, or the whole fleet during
        planned work, so intentional restarts don't page anyone.
      </Paragraph>
      <Button type="primary" onClick={() => setOpen(true)} style={{ marginBottom: 12 }}>
        Schedule window
      </Button>
      <Table<Window>
        dataSource={data || []}
        columns={columns}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="small"
      />

      <Modal
        title="Schedule maintenance window"
        open={open}
        onOk={create}
        confirmLoading={busy}
        onCancel={() => {
          setOpen(false);
          form.resetFields();
        }}
        okText="Schedule"
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ scope_type: "global" }}
          onValuesChange={(chg) => chg.scope_type && setScope(chg.scope_type)}
        >
          <Form.Item name="scope_type" label="Scope" rules={[{ required: true }]}>
            <Select
              options={[
                { value: "global", label: "Whole fleet" },
                { value: "environment", label: "Environment" },
                { value: "server", label: "Single server" },
              ]}
            />
          </Form.Item>
          {scope === "environment" ? (
            <Form.Item name="scope_value" label="Environment" rules={[{ required: true }]}>
              <Select options={environments.map((e) => ({ value: e, label: e }))} placeholder="prod" />
            </Form.Item>
          ) : null}
          {scope === "server" ? (
            <Form.Item name="scope_value" label="Server" rules={[{ required: true }]}>
              <Select
                showSearch
                optionFilterProp="label"
                options={(servers || []).map((s) => ({ value: s.id, label: s.name }))}
              />
            </Form.Item>
          ) : null}
          <Form.Item name="range" label="From → to" rules={[{ required: true, message: "Pick a window" }]}>
            <RangePicker showTime style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="reason" label="Reason">
            <Input placeholder="kernel upgrade" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
