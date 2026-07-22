import { useEffect, useState } from "react";
import { Card, Form, Input, Button, Typography, App } from "antd";
import { LockOutlined, UserOutlined } from "@ant-design/icons";
import apiClient from "../apiClient";
import { useTranslation } from "react-i18next";
import { useAuth } from "../hooks/useAuth";
import { useThemeMode } from "../theme/ThemeModeContext";
import { ThemeToggle } from "../components/ThemeToggle";
import { BrandLogo } from "../components/BrandLogo";

const { Text } = Typography;

export default function Login() {
  const { t } = useTranslation();
  const { login, setup } = useAuth();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [setupAvailable, setSetupAvailable] = useState(false);
  const [needs2FA, setNeeds2FA] = useState(false);
  const { mode } = useThemeMode();

  useEffect(() => {
    apiClient.get<{ available: boolean }>("/auth/setup")
      .then((resp) => setSetupAvailable(resp.data.available))
      .catch(() => setSetupAvailable(false));
  }, []);

  const handleSubmit = async (values: { username: string; password: string; totp_code?: string }) => {
    setLoading(true);
    try {
      if (setupAvailable) {
        await setup(values.username, values.password);
        message.success(t("login.admin_created"));
        window.location.reload();
        return;
      }
      const res = await login(values.username, values.password, values.totp_code);
      if (res.twoFactorRequired) {
        setNeeds2FA(true);
        message.info(t("login.enter_2fa"));
        return;
      }
      message.success(t("login.logged_in"));
      window.location.reload();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        minHeight: "100vh",
        background: mode === "dark" ? "#141414" : "#f9fafb",
      }}
    >
      <div style={{ position: "absolute", top: 20, right: 20 }}>
        <ThemeToggle />
      </div>
      <Card style={{ width: "min(400px, calc(100vw - 32px))", boxShadow: "0 1px 3px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.06)" }}>
        <div style={{ textAlign: "center", marginBottom: 24 }}>
          <div style={{ display: "flex", justifyContent: "center", marginBottom: 12 }}>
            <BrandLogo mode={mode} size="login" />
          </div>
          <Text type="secondary">{setupAvailable ? t("login.create_first_admin") : t("login.tagline")}</Text>
        </div>
        <Form onFinish={handleSubmit} size="large" requiredMark={false}>
          <Form.Item
            name="username"
            rules={[{ required: true, message: t("login.enter_username") }]}
          >
            <Input prefix={<UserOutlined />} placeholder={t("login.username")} aria-label={t("login.username")} />
          </Form.Item>
          <Form.Item
            name="password"
            rules={[{ required: true, message: t("login.enter_password") }]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder={t("login.password")} aria-label={t("login.password")} />
          </Form.Item>
          {needs2FA ? (
            <Form.Item
              name="totp_code"
              rules={[{ required: true, message: t("login.enter_code") }]}
            >
              <Input prefix={<LockOutlined />} placeholder={t("login.auth_code")} inputMode="numeric" />
            </Form.Item>
          ) : null}
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              {setupAvailable ? t("login.create_admin") : t("login.log_in")}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
