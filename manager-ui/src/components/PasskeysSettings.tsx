import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, Table, Button, Typography, App, Popconfirm, Modal, Form, Input } from "antd";
import { KeyOutlined } from "@ant-design/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { listPasskeys, registerPasskey, deletePasskey, type PasskeySummary } from "../lib/passkeys";
import { sortable } from "../lib/tableSort";

const { Title, Paragraph } = Typography;

// PasskeysSettings lets an admin enrol, list, and remove passkeys (SND-109).
// Only rendered where WebAuthn works (browser over https) — see passkeysAvailable.
export default function PasskeysSettings() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { message } = App.useApp();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const { data, isLoading } = useQuery({ queryKey: ["passkeys"], queryFn: listPasskeys });

  const submit = async (values: { password: string; label?: string }) => {
    setSaving(true);
    try {
      await registerPasskey(values.password, values.label?.trim() || "Passkey");
      message.success(t("passkeys.passkey_added"));
      setOpen(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["passkeys"] });
    } catch (err) {
      // User cancelling the browser prompt throws NotAllowedError — stay quiet.
      if (err instanceof Error && err.name !== "NotAllowedError" && err.name !== "AbortError") {
        message.error(err.message);
      }
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await deletePasskey(id);
      message.success(t("passkeys.passkey_removed"));
      qc.invalidateQueries({ queryKey: ["passkeys"] });
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = sortable<PasskeySummary>([
    { title: t("passkeys.label"), dataIndex: "label" },
    { title: t("passkeys.created"), dataIndex: "created_at", render: (v: string) => new Date(v).toLocaleString() },
    {
      title: t("passkeys.last_used"),
      dataIndex: "last_used_at",
      render: (v?: string) => (v ? new Date(v).toLocaleString() : "—"),
    },
    {
      title: t("passkeys.actions"),
      key: "actions",
      render: (_: unknown, r: PasskeySummary) => (
        <Popconfirm
          title={t("passkeys.remove_passkey")}
          onConfirm={() => remove(r.id)}
          okText={t("passkeys.remove")}
          cancelText={t("passkeys.cancel")}
        >
          <Button danger size="small">
            {t("passkeys.remove")}
          </Button>
        </Popconfirm>
      ),
    },
  ]);

  return (
    <Card>
      <Title level={4}>{t("passkeys.passkeys")}</Title>
      <Paragraph type="secondary">{t("passkeys.passkeys_help")}</Paragraph>
      <Button type="primary" icon={<KeyOutlined />} onClick={() => setOpen(true)} style={{ marginBottom: 16 }}>
        {t("passkeys.add_passkey")}
      </Button>
      <Table
        rowKey="id"
        loading={isLoading}
        columns={columns}
        dataSource={data ?? []}
        pagination={false}
        size="small"
        scroll={{ x: "max-content" }}
      />
      <Modal open={open} title={t("passkeys.add_passkey")} onCancel={() => setOpen(false)} footer={null} destroyOnClose>
        <Form form={form} layout="vertical" requiredMark={false} onFinish={submit}>
          <Form.Item name="label" label={t("passkeys.label")}>
            <Input placeholder={t("passkeys.label_placeholder")} />
          </Form.Item>
          <Form.Item
            name="password"
            label={t("passkeys.current_password")}
            rules={[{ required: true, message: t("passkeys.password_required") }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" loading={saving} block>
              {t("passkeys.add_passkey")}
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
