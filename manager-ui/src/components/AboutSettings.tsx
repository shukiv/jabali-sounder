import { useTranslation } from "react-i18next";
import { useState } from "react";
import { Card, Typography, Button, Tag, Space, Alert, App, Descriptions } from "antd";
import { CloudDownloadOutlined, ReloadOutlined, RocketOutlined } from "@ant-design/icons";
import { useQueryClient } from "@tanstack/react-query";
import { useVersion } from "../hooks/useVersion";
import { desktopBridge } from "../lib/desktop";

const { Title, Paragraph, Text } = Typography;

// AboutSettings shows the running build and the update status, with a "check
// now", a link to the release, and (desktop only) a one-click self-update.
export default function AboutSettings() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { message } = App.useApp();
  const { data, isFetching, refetch } = useVersion();
  const [installing, setInstalling] = useState(false);
  const bridge = desktopBridge();
  const canSelfUpdate = !!bridge?.InstallUpdate;

  const install = async () => {
    if (!bridge?.InstallUpdate) return;
    setInstalling(true);
    try {
      const res = await bridge.InstallUpdate();
      if (res.ok) {
        message.success(res.message || "Update installed — restarting");
      } else {
        message.error(res.message || "Update failed");
      }
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setInstalling(false);
    }
  };

  const status = () => {
    if (!data) return null;
    if (data.is_dev) return <Tag>developer build</Tag>;
    if (data.update_error) return <Tag color="default">check unavailable</Tag>;
    if (data.update_available) return <Tag color="gold">update available</Tag>;
    return <Tag color="green">up to date</Tag>;
  };

  return (
    <Card style={{ marginTop: 16 }}>
      <Title level={4}>
        <RocketOutlined /> About &amp; updates {status()}
      </Title>
      <Descriptions size="small" column={1} style={{ maxWidth: 480, marginBottom: 12 }}>
        <Descriptions.Item label={t("about.version")}>
          <Text code>{data?.version ?? "…"}</Text>
        </Descriptions.Item>
        <Descriptions.Item label={t("about.commit")}>
          <Text type="secondary">{data?.commit ?? "—"}</Text>
        </Descriptions.Item>
        <Descriptions.Item label={t("about.built")}>{data?.date ?? "—"}</Descriptions.Item>
        {data?.latest ? <Descriptions.Item label={t("about.latest_release")}>{data.latest}</Descriptions.Item> : null}
      </Descriptions>

      {data?.update_available ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={`Version ${data.latest} is available`}
          description={
            canSelfUpdate
              ? "Install it now, or view the release notes."
              : "Download the new build from the release page."
          }
          action={
            data.release_url ? (
              <a href={data.release_url} target="_blank" rel="noreferrer">
                <Button size="small">Release notes</Button>
              </a>
            ) : null
          }
        />
      ) : null}

      <Space wrap>
        <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => { qc.invalidateQueries({ queryKey: ["version"] }); refetch(); }}>
          Check for updates
        </Button>
        {canSelfUpdate && data?.update_available ? (
          <Button type="primary" icon={<CloudDownloadOutlined />} loading={installing} onClick={install}>
            Install update
          </Button>
        ) : null}
        {!canSelfUpdate && data?.release_url ? (
          <a href={data.release_url} target="_blank" rel="noreferrer">
            <Button>Open downloads</Button>
          </a>
        ) : null}
      </Space>
      <Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0, fontSize: 12 }}>
        The update check queries the public GitHub releases once an hour.
      </Paragraph>
    </Card>
  );
}
