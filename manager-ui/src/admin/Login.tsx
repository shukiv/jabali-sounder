import { useEffect, useState } from "react";
import { Card, Form, Input, Button, Typography, App } from "antd";
import { LockOutlined, UserOutlined } from "@ant-design/icons";
import apiClient from "../apiClient";
import { useAuth } from "../hooks/useAuth";
import { useThemeMode } from "../theme/ThemeModeContext";
import { ThemeToggle } from "../components/ThemeToggle";
import { BrandLogo } from "../components/BrandLogo";

const { Text } = Typography;

export default function Login() {
  const { login, setup } = useAuth();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [setupAvailable, setSetupAvailable] = useState(false);
  const { mode } = useThemeMode();

  useEffect(() => {
    apiClient.get<{ available: boolean }>("/auth/setup")
      .then((resp) => setSetupAvailable(resp.data.available))
      .catch(() => setSetupAvailable(false));
  }, []);

  const handleSubmit = async (values: { username: string; password: string }) => {
    setLoading(true);
    try {
      if (setupAvailable) {
        await setup(values.username, values.password);
        message.success("Admin account created");
      } else {
        await login(values.username, values.password);
        message.success("Logged in");
      }
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
      <Card style={{ width: 400, boxShadow: "0 1px 3px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.06)" }}>
        <div style={{ textAlign: "center", marginBottom: 24 }}>
          <div style={{ display: "flex", justifyContent: "center", marginBottom: 12 }}>
            <BrandLogo mode={mode} size="login" />
          </div>
          <Text type="secondary">{setupAvailable ? "Create the first admin account" : "Central Sounding Plane"}</Text>
        </div>
        <Form onFinish={handleSubmit} size="large" requiredMark={false}>
          <Form.Item
            name="username"
            rules={[{ required: true, message: "Enter username" }]}
          >
            <Input prefix={<UserOutlined />} placeholder="Username" />
          </Form.Item>
          <Form.Item
            name="password"
            rules={[{ required: true, message: "Enter password" }]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="Password" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              {setupAvailable ? "Create Admin" : "Log in"}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
