import { useEffect, useState } from "react";
import { Card, Form, Input, Button, Typography, App, Modal } from "antd";
import { LockOutlined, UserOutlined } from "@ant-design/icons";
import apiClient from "../apiClient";
import { useTranslation } from "react-i18next";
import { useAuth } from "../hooks/useAuth";
import { useThemeMode } from "../theme/ThemeModeContext";
import { ThemeToggle } from "../components/ThemeToggle";
import { BrandLogo } from "../components/BrandLogo";
import { Call } from "@wailsio/runtime";
import { isMobileApp } from "../lib/desktop";

const { Text } = Typography;

export default function Login() {
  const { t } = useTranslation();
  const { login, setup } = useAuth();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [setupAvailable, setSetupAvailable] = useState(false);
  const [needs2FA, setNeeds2FA] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);
  const [resetLoading, setResetLoading] = useState(false);
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

  const handleReset = async (values: { username?: string; new_password: string }) => {
    setResetLoading(true);
    try {
      await Call.ByName(
        "main.Bridge.ResetLostPassword",
        (values.username || "admin").trim(),
        values.new_password,
      );
      message.success(t("login.password_reset_done"));
      setResetOpen(false);
    } catch (err) {
      message.error(err instanceof Error ? err.message : String(err));
    } finally {
      setResetLoading(false);
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
        {!setupAvailable && isMobileApp() ? (
          <>
            <Button type="link" block onClick={() => setResetOpen(true)} style={{ marginTop: 4 }}>
              {t("login.forgot_password")}
            </Button>
            <Modal
              open={resetOpen}
              title={t("login.reset_title")}
              onCancel={() => setResetOpen(false)}
              footer={null}
              destroyOnClose
            >
              <Text type="secondary">{t("login.reset_help")}</Text>
              <Form onFinish={handleReset} layout="vertical" requiredMark={false} style={{ marginTop: 16 }}>
                <Form.Item name="username" initialValue="admin" label={t("login.username")}>
                  <Input prefix={<UserOutlined />} />
                </Form.Item>
                <Form.Item
                  name="new_password"
                  label={t("settings.new_password")}
                  rules={[{ required: true, min: 8, message: t("settings.at_least_8_characters") }]}
                >
                  <Input.Password prefix={<LockOutlined />} />
                </Form.Item>
                <Form.Item
                  name="confirm"
                  label={t("settings.confirm_new_password")}
                  dependencies={["new_password"]}
                  rules={[
                    { required: true, message: t("settings.confirm_the_new_password") },
                    ({ getFieldValue }) => ({
                      validator(_, value) {
                        return !value || getFieldValue("new_password") === value
                          ? Promise.resolve()
                          : Promise.reject(new Error(t("settings.confirm_the_new_password")));
                      },
                    }),
                  ]}
                >
                  <Input.Password prefix={<LockOutlined />} />
                </Form.Item>
                <Form.Item style={{ marginBottom: 0 }}>
                  <Button type="primary" htmlType="submit" loading={resetLoading} block>
                    {t("login.reset_title")}
                  </Button>
                </Form.Item>
              </Form>
            </Modal>
          </>
        ) : null}
      </Card>
    </div>
  );
}
