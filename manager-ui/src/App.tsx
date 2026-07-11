import { useEffect } from "react";
import { Routes, Route, Navigate } from "react-router";
import { ConfigProvider } from "antd";
import AdminLayout from "./shells/AdminLayout";
import Dashboard from "./admin/Dashboard";
import Servers from "./admin/Servers";
import Monitor from "./admin/Monitor";
import Mail from "./admin/Mail";
import Domains from "./admin/Domains";
import Users from "./admin/Users";
import Settings from "./admin/Settings";
import Audit from "./admin/Audit";
import Backups from "./admin/Backups";
import Policy from "./admin/Policy";
import Team from "./admin/Team";
import Login from "./admin/Login";
import { useAuth } from "./hooks/useAuth";
import { useThemeMode } from "./theme/ThemeModeContext";
import { useTheme } from "./hooks/useTheme";
import { installExternalLinkHandler } from "./lib/desktop";

export default function App() {
  const { auth } = useAuth();
  const { mode } = useThemeMode();
  const cfg = useTheme(mode);

  // Desktop: open external links in the system browser (the webview ignores
  // target="_blank"). No-op in the browser build.
  useEffect(() => installExternalLinkHandler(), []);

  return (
    <ConfigProvider {...cfg}>
      {!auth.token ? (
        <Routes>
          <Route path="*" element={<Login />} />
        </Routes>
      ) : (
        <Routes>
          <Route path="/" element={<AdminLayout />}>
            <Route index element={<Dashboard />} />
            <Route path="servers" element={<Servers />} />
            <Route path="monitor" element={<Monitor />} />
            <Route path="mail" element={<Mail />} />
            <Route path="domains" element={<Domains />} />
            <Route path="users" element={<Users />} />
            <Route path="settings" element={<Settings />} />
            <Route path="team" element={<Team />} />
            <Route path="audit" element={<Audit />} />
            <Route path="backups" element={<Backups />} />
            <Route path="policy" element={<Policy />} />
            <Route path="login" element={<Navigate to="/" replace />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      )}
    </ConfigProvider>
  );
}
