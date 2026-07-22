import { useEffect } from "react";
import { Routes, Route, Navigate } from "react-router";
import { ConfigProvider } from "antd";
import { useTranslation } from "react-i18next";
import { antdLocale, isRTL } from "./i18n";
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
  // Re-renders on language change so AntD's locale + direction follow.
  const { i18n } = useTranslation();

  // Desktop: open external links in the system browser (the webview ignores
  // target="_blank"). No-op in the browser build.
  useEffect(() => installExternalLinkHandler(), []);

  // SND-59: AntD renders horizontally scrollable tables in a div with no
  // keyboard affordance. Tag those scroll regions so keyboard-only users can
  // focus and scroll them to reach clipped columns/actions.
  useEffect(() => {
    const tag = () => {
      document
        .querySelectorAll<HTMLElement>(".ant-table-content, .ant-table-body")
        .forEach((el) => {
          if (el.scrollWidth <= el.clientWidth) return;
          if (el.getAttribute("tabindex") != null) return;
          el.setAttribute("tabindex", "0");
          el.setAttribute("role", "region");
          el.setAttribute("aria-label", "Table, scrollable");
        });
    };
    tag();
    // Throttle heavily and only react when element nodes were actually added, so
    // the observer never reads layout during scrolling (avoids jank).
    let timer: number | undefined;
    const schedule = (records: MutationRecord[]) => {
      if (timer != null) return;
      const added = records.some((r) => r.addedNodes.length > 0);
      if (!added) return;
      timer = window.setTimeout(() => {
        timer = undefined;
        tag();
      }, 300);
    };
    const obs = new MutationObserver(schedule);
    obs.observe(document.body, { childList: true, subtree: true });
    return () => {
      obs.disconnect();
      if (timer != null) window.clearTimeout(timer);
    };
  }, []);

  return (
    <ConfigProvider
      {...cfg}
      locale={antdLocale(i18n.resolvedLanguage)}
      direction={isRTL(i18n.resolvedLanguage) ? "rtl" : "ltr"}
    >
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
