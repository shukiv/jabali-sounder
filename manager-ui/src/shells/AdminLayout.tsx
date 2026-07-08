import { useState } from "react";
import {
  Avatar,
  Button,
  Drawer,
  Dropdown,
  Grid,
  Layout,
  Menu,
  Space,
  Typography,
  theme,
} from "antd";
import {
  LeftOutlined,
  RightOutlined,
  LogoutOutlined,
  MenuOutlined,
  UserOutlined,
  HomeOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  GlobalOutlined,
  MailOutlined,
  TeamOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import { Outlet, useLocation, useNavigate } from "react-router";

import { useAuth } from "../hooks/useAuth";
import { useThemeMode } from "../theme/ThemeModeContext";
import { ThemeToggle } from "../components/ThemeToggle";
import { BrandLogo } from "../components/BrandLogo";

const { Header, Sider, Content, Footer } = Layout;
const { Text, Link } = Typography;

const navItems = [
  { key: "/", label: "Dashboard", icon: <HomeOutlined style={{ fontSize: 20, color: "#6b7280" }} /> },
  { key: "/servers", label: "Servers", icon: <CloudServerOutlined style={{ fontSize: 20, color: "#6b7280" }} /> },
  { key: "/monitor", label: "Monitor", icon: <DashboardOutlined style={{ fontSize: 20, color: "#6b7280" }} /> },
  { key: "/mail", label: "Mail", icon: <MailOutlined style={{ fontSize: 20, color: "#6b7280" }} /> },
  { key: "/domains", label: "Domains", icon: <GlobalOutlined style={{ fontSize: 20, color: "#6b7280" }} /> },
  { key: "/users", label: "Users", icon: <TeamOutlined style={{ fontSize: 20, color: "#6b7280" }} /> },
  { key: "/settings", label: "Settings", icon: <SettingOutlined style={{ fontSize: 20, color: "#6b7280" }} /> },
];

export default function AdminLayout() {
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const location = useLocation();
  const navigate = useNavigate();
  const { auth, logout } = useAuth();
  const { mode } = useThemeMode();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isDesktop = screens.lg ?? (typeof window !== "undefined" ? window.innerWidth >= 992 : true);

  const siderBg = token.colorBgLayout;

  const menu = (
    <Menu
      mode="inline"
      theme={mode}
      selectedKeys={[location.pathname]}
      style={{ border: "none", background: siderBg }}
      items={navItems.map((n) => ({
        key: n.key,
        icon: n.icon,
        label: n.label,
        onClick: () => {
          navigate(n.key);
          setDrawerOpen(false);
        },
      }))}
    />
  );

  const userMenu = {
    items: [
      {
        key: "logout",
        label: "Logout",
        icon: <LogoutOutlined />,
        onClick: () => {
          logout();
          window.location.reload();
        },
      },
    ],
  };

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Header
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "0 24px",
          background: mode === "dark" ? token.colorBgLayout : "#fff",
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
          height: 64,
          position: "sticky",
          top: 0,
          zIndex: 100,
        }}
      >
        <Space size={12}>
          {!isDesktop && (
            <Button
              type="text"
              icon={<MenuOutlined />}
              onClick={() => setDrawerOpen(true)}
            />
          )}
          <BrandLogo mode={mode} />
        </Space>
        <Space size={8}>
          <ThemeToggle />
          <Dropdown menu={userMenu} placement="bottomRight">
            <Button type="text" style={{ height: 40 }}>
              <Space size={6}>
                <Avatar size={28} icon={<UserOutlined />} />
                {screens.sm !== false && <Text>{auth.username}</Text>}
              </Space>
            </Button>
          </Dropdown>
        </Space>
      </Header>

      <Layout>
        {isDesktop ? (
          <Sider
            theme={mode}
            width={256}
            breakpoint="lg"
            collapsedWidth="64"
            collapsible
            collapsed={collapsed}
            onCollapse={setCollapsed}
            trigger={
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  width: "100%",
                  height: "100%",
                  color: token.colorTextSecondary,
                  background: siderBg,
                }}
              >
                {collapsed ? <RightOutlined /> : <LeftOutlined />}
              </div>
            }
            style={{
              background: siderBg,
              paddingTop: 16,
              paddingInline: 8,
              height: "calc(100vh - 64px)",
              position: "sticky",
              top: 64,
              overflow: "hidden",
            }}
          >
            <div
              style={{
                height: "100%",
                overflowY: "auto",
                overflowX: "hidden",
                paddingBottom: 48,
              }}
            >
              {menu}
            </div>
          </Sider>
        ) : (
          <Drawer
            open={drawerOpen}
            onClose={() => setDrawerOpen(false)}
            placement="left"
            width={256}
            closable
            title={<BrandLogo mode={mode} />}
            styles={{
              body: { padding: 8, background: siderBg },
              header: { background: siderBg },
            }}
          >
            {menu}
          </Drawer>
        )}

        <Layout>
          <Content
            style={{
              padding: screens.md ? "32px 24px 24px" : "20px 12px 12px",
              minWidth: 0,
              overflowX: "hidden",
            }}
          >
            <Outlet />
          </Content>
          <Footer
            style={{
              display: "flex",
              flexDirection: screens.lg !== false ? "row" : "column",
              alignItems: "center",
              justifyContent: screens.lg !== false ? "space-between" : "center",
              textAlign: screens.lg !== false ? "left" : "center",
              gap: 16,
              padding: "8px 24px",
              background: mode === "dark" ? token.colorBgLayout : "#f9fafb",
            }}
          >
            <Space size={12}>
              <BrandLogo mode={mode} size="footer" />
              {screens.sm !== false && (
                <Text type="secondary">Central Sounding Plane</Text>
              )}
            </Space>
            <Space size={12}>
              <Link href="https://codeberg.org/shukivaknin/jabali-sounder" target="_blank">
                Source
              </Link>
              <Text type="secondary">·</Text>
              <Text type="secondary">AGPL-3.0</Text>
              <Text strong>v0.1.0</Text>
            </Space>
          </Footer>
        </Layout>
      </Layout>
    </Layout>
  );
}
