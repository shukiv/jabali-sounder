import { useTranslation } from "react-i18next";
import { useState } from "react";
import { Alert, App, Button, Card, Form, Input, Space, Typography, Upload } from "antd";
import { DownloadOutlined, UploadOutlined, LockOutlined } from "@ant-design/icons";
import type { UploadProps } from "antd";
import { useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";
import TwoFactorSettings from "../components/TwoFactorSettings";
import SessionsSettings from "../components/SessionsSettings";
import ApiTokensSettings from "../components/ApiTokensSettings";
import AlertRulesSettings from "../components/AlertRulesSettings";
import AlertChannelsSettings from "../components/AlertChannelsSettings";
import MaintenanceSettings from "../components/MaintenanceSettings";
import AboutSettings from "../components/AboutSettings";
import { roleAtLeast } from "../hooks/useAuth";
import { desktopBridge, isMobileApp } from "../lib/desktop";

const { Title, Text, Paragraph } = Typography;

interface ImportResult {
  imported: number;
  updated: number;
  created: number;
  skipped: number;
  errors?: string[];
}

export default function Settings() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const queryClient = useQueryClient();
  const [exporting, setExporting] = useState(false);
  const [exportingCsv, setExportingCsv] = useState(false);
  const [importing, setImporting] = useState(false);
  const [lastImport, setLastImport] = useState<ImportResult | null>(null);
  const [changingPassword, setChangingPassword] = useState(false);
  const [pwForm] = Form.useForm();

  const handleChangePassword = async () => {
    let values;
    try {
      values = await pwForm.validateFields();
    } catch {
      return;
    }
    setChangingPassword(true);
    try {
      await apiClient.post("/auth/change-password", {
        current_password: values.current_password,
        new_password: values.new_password,
      });
      message.success(t("settings.password_changed"));
      pwForm.resetFields();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setChangingPassword(false);
    }
  };

  const handleExport = async (includeSecrets = false) => {
    setExporting(true);
    try {
      const resp = await apiClient.get(
        `/admin/settings/export${includeSecrets ? "?include_secrets=true" : ""}`,
        { responseType: "blob" },
      );
      const text = await (resp.data as Blob).text();
      const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
      const filename = `jabali-sounder-settings${includeSecrets ? "-with-secrets" : ""}-${stamp}.json`;

      // Desktop (Wails): open a native Save As dialog. The browser <a download>
      // trick below is a no-op inside the WebKit webview.
      const bridge = desktopBridge();
      if (bridge && isMobileApp()) {
        // Android/iOS have no save-file dialog — share the export instead.
        await bridge.ShareText?.(text);
        message.success(t("settings.opened_the_share_sheet_save_the"));
        return;
      }
      if (bridge?.SaveFile) {
        const saved = await bridge.SaveFile(filename, text);
        if (saved) message.success(`Exported to ${saved}`);
        return;
      }

      const url = URL.createObjectURL(new Blob([text], { type: "application/json" }));
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      message.success(t("settings.settings_exported"));
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setExporting(false);
    }
  };

  const handleExportCSV = async () => {
    setExportingCsv(true);
    try {
      const resp = await apiClient.get("/admin/settings/report.csv", { responseType: "blob" });
      const text = await (resp.data as Blob).text();
      const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
      const filename = `jabali-sounder-fleet-${stamp}.csv`;
      const bridge = desktopBridge();
      if (bridge?.SaveFile) {
        const saved = await bridge.SaveFile(filename, text);
        if (saved) message.success(`Exported to ${saved}`);
        return;
      }
      const url = URL.createObjectURL(new Blob([text], { type: "text/csv" }));
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      message.success(t("settings.fleet_csv_exported"));
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setExportingCsv(false);
    }
  };

  const runImport = async (text: string) => {
    setImporting(true);
    setLastImport(null);
    try {
      const payload = JSON.parse(text) as unknown;
      const resp = await apiClient.post<ImportResult>("/admin/settings/import", payload);
      setLastImport(resp.data);
      queryClient.invalidateQueries({ queryKey: ["servers"] });
      queryClient.invalidateQueries({ queryKey: ["dashboard"] });
      if (resp.data.skipped > 0) {
        message.warning(`Imported ${resp.data.imported}; skipped ${resp.data.skipped}`);
      } else {
        message.success(`Imported ${resp.data.imported} server settings`);
      }
    } catch (err) {
      if (err instanceof SyntaxError) {
        message.error(t("settings.import_file_is_not_valid_json"));
      } else if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setImporting(false);
    }
  };

  const uploadProps: UploadProps = {
    accept: "application/json,.json",
    maxCount: 1,
    showUploadList: false,
    beforeUpload: async (file) => {
      await runImport(await file.text());
      return Upload.LIST_IGNORE;
    },
  };

  // Mobile: the WebView <input type=file> does not open a picker, so use the
  // native file picker (SAF on Android) via the bridge.
  const handleMobileImport = async () => {
    try {
      const content = await desktopBridge()?.PickFile?.();
      if (content) await runImport(content);
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  return (
    <div>
      <Space wrap style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Title level={3} style={{ margin: 0 }}>Settings</Title>
      </Space>

      <nav className="settings-section-nav" aria-label={t("settings.settings_sections")}>
        <a href="#sec-import">Import / Export</a>
        <a href="#sec-password">Password</a>
        <a href="#sec-2fa">Two-factor</a>
        <a href="#sec-sessions">Sessions</a>
        {roleAtLeast("operator") ? <a href="#sec-tokens">API tokens</a> : null}
        <a href="#sec-alerts">Alerts</a>
        {roleAtLeast("operator") ? <a href="#sec-maintenance">Maintenance</a> : null}
        <a href="#sec-about">About</a>
      </nav>

      <section id="sec-import">
      <Card title={t("settings.import_export")}>
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Paragraph type="secondary" style={{ margin: 0 }}>
            Export the current Sounder settings and enrolled server list as JSON.
            By default token secrets are exported encrypted (usable only on this install). Use “Export + tokens” to include them in plaintext for migrating to another install.
          </Paragraph>
          <Space wrap>
            <Button
              icon={<DownloadOutlined />}
              loading={exporting}
              onClick={() => handleExport(false)}
            >
              Export Settings
            </Button>
            <Button
              danger
              icon={<DownloadOutlined />}
              loading={exporting}
              onClick={() =>
                modal.confirm({
                  title: t("settings.export_with_token_secrets"),
                  content:
                    "The file will contain your panels' token secrets in PLAINTEXT so it can be imported on another install. Anyone with the file can control those panels — store it like a password and delete it after use.",
                  okText: t("settings.export_with_secrets"),
                  okButtonProps: { danger: true },
                  onOk: () => handleExport(true),
                })
              }
            >
              Export + tokens
            </Button>
            <Button
              icon={<DownloadOutlined />}
              loading={exportingCsv}
              onClick={handleExportCSV}
            >
              Fleet CSV
            </Button>
            {isMobileApp() ? (
              <Button icon={<UploadOutlined />} loading={importing} onClick={handleMobileImport}>
                Import Settings
              </Button>
            ) : (
              <Upload {...uploadProps}>
                <Button icon={<UploadOutlined />} loading={importing}>
                  Import Settings
                </Button>
              </Upload>
            )}
          </Space>
          <Alert
            type="info"
            showIcon
            message="Portable imports"
            description={t("settings.for_a_different_sounder_install_add")}
          />
          {lastImport && (
            <Alert
              type={lastImport.skipped > 0 ? "warning" : "success"}
              showIcon
              message={`Imported ${lastImport.imported} server settings`}
              description={
                <Space direction="vertical" size={4}>
                  <Text>Created: {lastImport.created} · Updated: {lastImport.updated} · Skipped: {lastImport.skipped}</Text>
                  {(lastImport.errors || []).map((error) => (
                    <Text key={error} type="danger">{error}</Text>
                  ))}
                </Space>
              }
            />
          )}
        </Space>
      </Card>
      </section>

      <section id="sec-password">
      <Card title={t("settings.change_password")} style={{ marginTop: 16 }}>
        <Form
          form={pwForm}
          layout="vertical"
          style={{ maxWidth: 420 }}
          requiredMark={false}
        >
          <Form.Item
            name="current_password"
            label={t("settings.current_password")}
            rules={[{ required: true, message: t("settings.enter_your_current_password") }]}
          >
            <Input.Password prefix={<LockOutlined />} autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label={t("settings.new_password")}
            rules={[
              { required: true, message: t("settings.enter_a_new_password") },
              { min: 8, message: t("settings.at_least_8_characters") },
            ]}
          >
            <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="confirm_password"
            label={t("settings.confirm_new_password")}
            dependencies={["new_password"]}
            rules={[
              { required: true, message: t("settings.confirm_the_new_password") },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue("new_password") === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error("Passwords do not match"));
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
          </Form.Item>
          <Button type="primary" loading={changingPassword} onClick={handleChangePassword}>
            Change Password
          </Button>
        </Form>
      </Card>
      </section>

      <section id="sec-2fa"><TwoFactorSettings /></section>

      <section id="sec-sessions"><SessionsSettings /></section>

      {roleAtLeast("operator") ? <section id="sec-tokens"><ApiTokensSettings /></section> : null}

      <section id="sec-alerts">
        <AlertRulesSettings />
        {roleAtLeast("operator") ? <AlertChannelsSettings /> : null}
      </section>
      {roleAtLeast("operator") ? <section id="sec-maintenance"><MaintenanceSettings /></section> : null}

      <section id="sec-about"><AboutSettings /></section>
    </div>
  );
}
