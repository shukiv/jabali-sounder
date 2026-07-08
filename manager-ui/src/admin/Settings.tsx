import { useState } from "react";
import { Alert, App, Button, Card, Space, Typography, Upload } from "antd";
import { DownloadOutlined, UploadOutlined } from "@ant-design/icons";
import type { UploadProps } from "antd";
import { useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";

const { Title, Text, Paragraph } = Typography;

interface ImportResult {
  imported: number;
  updated: number;
  created: number;
  skipped: number;
  errors?: string[];
}

export default function Settings() {
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [lastImport, setLastImport] = useState<ImportResult | null>(null);

  const handleExport = async () => {
    setExporting(true);
    try {
      const resp = await apiClient.get("/admin/settings/export", {
        responseType: "blob",
      });
      const blob = new Blob([resp.data], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
      link.href = url;
      link.download = `jabali-sounder-settings-${stamp}.json`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      message.success("Settings exported");
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setExporting(false);
    }
  };

  const uploadProps: UploadProps = {
    accept: "application/json,.json",
    maxCount: 1,
    showUploadList: false,
    beforeUpload: async (file) => {
      setImporting(true);
      setLastImport(null);
      try {
        const text = await file.text();
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
          message.error("Import file is not valid JSON");
        } else if (err instanceof Error) {
          message.error(err.message);
        }
      } finally {
        setImporting(false);
      }
      return Upload.LIST_IGNORE;
    },
  };

  return (
    <div>
      <Space style={{ marginBottom: 16, width: "100%", justifyContent: "space-between" }}>
        <Title level={3} style={{ margin: 0 }}>Settings</Title>
      </Space>

      <Card title="Import / Export">
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Paragraph type="secondary" style={{ margin: 0 }}>
            Export the current Sounder settings and enrolled server list as JSON.
            Token secrets are exported only as encrypted local backup data.
          </Paragraph>
          <Space wrap>
            <Button
              icon={<DownloadOutlined />}
              loading={exporting}
              onClick={handleExport}
            >
              Export Settings
            </Button>
            <Upload {...uploadProps}>
              <Button icon={<UploadOutlined />} loading={importing}>
                Import Settings
              </Button>
            </Upload>
          </Space>
          <Alert
            type="info"
            showIcon
            message="Portable imports"
            description="For a different Sounder install, add token_secret values to the imported JSON or rotate tokens after import."
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
    </div>
  );
}
