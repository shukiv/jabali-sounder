import { useTranslation } from "react-i18next";
import { useState } from "react";
import { Card, Button, Modal, Form, Input, Typography, Tag, Alert, App, Space } from "antd";
import { SafetyCertificateOutlined } from "@ant-design/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";
import { QRCodeSVG } from "qrcode.react";

const { Title, Text, Paragraph } = Typography;

interface Me {
  two_factor_enabled: boolean;
}

// TwoFactorSettings enables/disables TOTP two-factor for the signed-in operator
// (M3). Enrollment shows the secret for manual entry into an authenticator app,
// then confirms with a code before enabling.
export default function TwoFactorSettings() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { message } = App.useApp();
  const { data: me } = useQuery({
    queryKey: ["me"],
    queryFn: async () => (await apiClient.get<Me>("/auth/me")).data,
  });
  const enabled = !!me?.two_factor_enabled;

  const [enrollSecret, setEnrollSecret] = useState<string | null>(null);
  const [otpauthUrl, setOtpauthUrl] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [disableOpen, setDisableOpen] = useState(false);
  const [enrollForm] = Form.useForm();
  const [disableForm] = Form.useForm();

  const refetchMe = () => qc.invalidateQueries({ queryKey: ["me"] });

  const startEnroll = async () => {
    setBusy(true);
    try {
      const resp = await apiClient.post<{ secret: string; otpauth_url: string }>("/auth/2fa/setup");
      setEnrollSecret(resp.data.secret);
      setOtpauthUrl(resp.data.otpauth_url);
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  const activate = async () => {
    let values;
    try {
      values = await enrollForm.validateFields();
    } catch {
      return;
    }
    setBusy(true);
    try {
      await apiClient.post("/auth/2fa/activate", { code: values.code });
      message.success(t("two_factor.two_factor_authentication_enabled"));
      setEnrollSecret(null);
      setOtpauthUrl(null);
      enrollForm.resetFields();
      refetchMe();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  const disable = async () => {
    let values;
    try {
      values = await disableForm.validateFields();
    } catch {
      return;
    }
    setBusy(true);
    try {
      await apiClient.post("/auth/2fa/disable", { password: values.password, code: values.code });
      message.success(t("two_factor.two_factor_authentication_disabled"));
      setDisableOpen(false);
      disableForm.resetFields();
      refetchMe();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card style={{ marginTop: 16 }}>
      <Title level={4}>
        <SafetyCertificateOutlined /> Two-factor authentication{" "}
        {enabled ? <Tag color="success">Enabled</Tag> : <Tag>Disabled</Tag>}
      </Title>
      <Paragraph type="secondary">
        Require a time-based code from an authenticator app (Google Authenticator,
        Aegis, 1Password) in addition to your password.
      </Paragraph>

      {enabled ? (
        <Button danger onClick={() => setDisableOpen(true)}>
          Disable 2FA
        </Button>
      ) : enrollSecret ? (
        <Space direction="vertical" style={{ width: "100%" }}>
          <Alert
            type="info"
            showIcon
            message="Scan the QR with your authenticator app (or enter the secret manually), then enter a code to confirm."
          />
          {otpauthUrl ? (
            <div style={{ background: "#fff", padding: 12, width: "fit-content", borderRadius: 6 }}>
              <QRCodeSVG value={otpauthUrl} size={160} />
            </div>
          ) : null}
          <Text>Secret key (manual entry):</Text>
          <Text code copyable style={{ fontSize: 16 }}>
            {enrollSecret}
          </Text>
          <Form form={enrollForm} layout="inline">
            <Form.Item name="code" rules={[{ required: true, message: t("two_factor.enter_the_6_digit_code") }]}>
              <Input placeholder="123456" inputMode="numeric" maxLength={6} />
            </Form.Item>
            <Button type="primary" loading={busy} onClick={activate}>
              Confirm & enable
            </Button>
          </Form>
        </Space>
      ) : (
        <Button type="primary" loading={busy} onClick={startEnroll}>
          Enable 2FA
        </Button>
      )}

      <Modal
        title={t("two_factor.disable_two_factor_authentication")}
        open={disableOpen}
        onOk={disable}
        confirmLoading={busy}
        onCancel={() => {
          setDisableOpen(false);
          disableForm.resetFields();
        }}
        okText={t("two_factor.disable")}
        okButtonProps={{ danger: true }}
      >
        <Form form={disableForm} layout="vertical">
          <Form.Item name="password" label={t("two_factor.current_password")} rules={[{ required: true }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item name="code" label={t("two_factor.authenticator_code")} rules={[{ required: true }]}>
            <Input inputMode="numeric" maxLength={6} placeholder="123456" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
