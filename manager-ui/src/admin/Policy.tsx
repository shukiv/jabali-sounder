import { useState } from "react";
import { useTranslation } from "react-i18next";
import { App, Button, Card, Col, DatePicker, Empty, Form, Input, Modal, Row, Switch, Table, Tag, Tooltip, Typography } from "antd";
import { EyeInvisibleOutlined, FileTextOutlined, ReloadOutlined, SafetyOutlined, UndoOutlined, WarningOutlined } from "@ant-design/icons";
import { StatCard } from "../components/StatCard";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Dayjs } from "dayjs";
import apiClient from "../apiClient";
import { sortable } from "../lib/tableSort";
import { roleAtLeast } from "../hooks/useAuth";
import { fmtAbs, fmtAge } from "../components/monitorFormat";

const { Title, Text } = Typography;

interface Violation {
  server_id: string;
  server_name: string;
  check: string;
  severity: "info" | "warning" | "critical";
  message: string;
  ignored?: boolean;
  reason?: string;
  ignored_by?: string;
  expires_at?: string;
  ignored_at?: string;
}

interface PolicyResp {
  violations: Violation[];
  ignored: Violation[];
  total: number;
  critical: number;
  by_check: Record<string, number>;
  servers_total: number;
  servers_at_risk: number;
}

const SEV_COLOR: Record<string, string> = { info: "blue", warning: "gold", critical: "red" };

export default function Policy() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const qc = useQueryClient();
  const canWrite = roleAtLeast("operator");
  const [showIgnored, setShowIgnored] = useState(false);
  const [ignoring, setIgnoring] = useState<Violation | null>(null);
  const [form] = Form.useForm();

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ["policy"],
    queryFn: async () => (await apiClient.get<PolicyResp>("/admin/policy")).data,
    refetchInterval: 60_000,
  });

  const CHECK_LABEL: Record<string, string> = {
    insecure_tls: t("compliance.check_insecure_tls"),
    credential_invalid: t("compliance.check_credential_invalid"),
    unreachable: t("compliance.check_unreachable"),
    cert_expiring: t("compliance.check_cert_expiring"),
    version_drift: t("compliance.check_version_drift"),
  };

  const restoreMut = useMutation({
    mutationFn: async (v: Violation) =>
      apiClient.delete(`/admin/policy/exceptions?server_id=${encodeURIComponent(v.server_id)}&check=${encodeURIComponent(v.check)}`),
    onSuccess: () => {
      message.success(t("compliance.violation_restored"));
      qc.invalidateQueries({ queryKey: ["policy"] });
    },
    onError: (err) => message.error(err instanceof Error ? err.message : "error"),
  });

  const submitIgnore = async () => {
    let values: { reason: string; expires_at?: Dayjs };
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    if (!ignoring) return;
    try {
      await apiClient.post("/admin/policy/exceptions", {
        server_id: ignoring.server_id,
        check: ignoring.check,
        reason: values.reason,
        expires_at: values.expires_at ? values.expires_at.format("YYYY-MM-DD") : "",
      });
      message.success(t("compliance.violation_ignored"));
      setIgnoring(null);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["policy"] });
    } catch (err) {
      message.error(err instanceof Error ? err.message : "error");
    }
  };

  const baseColumns = [
    { title: t("compliance.server"), dataIndex: "server_name", key: "server_name" },
    { title: t("compliance.check"), dataIndex: "check", key: "check", render: (c: string) => <Tag>{CHECK_LABEL[c] || c}</Tag> },
    { title: t("compliance.severity"), dataIndex: "severity", key: "severity", render: (s: string) => <Tag color={SEV_COLOR[s] || "default"}>{s}</Tag> },
    { title: t("compliance.detail"), dataIndex: "message", key: "message", render: (m: string) => <Text type="secondary">{m}</Text> },
  ];

  const activeColumns = [
    ...baseColumns,
    ...(canWrite
      ? [{
          title: t("compliance.actions"),
          key: "actions",
          render: (_: unknown, v: Violation) => (
            <Button size="small" icon={<EyeInvisibleOutlined />} onClick={() => { setIgnoring(v); form.resetFields(); }}>
              {t("compliance.ignore")}
            </Button>
          ),
        }]
      : []),
  ];

  const ignoredColumns = [
    ...baseColumns,
    {
      title: t("compliance.exception"),
      key: "exception",
      render: (_: unknown, v: Violation) => (
        <span>
          <Text>{v.reason}</Text>
          <br />
          <Text type="secondary" style={{ fontSize: 12 }}>
            {t("compliance.by")} {v.ignored_by || "—"}
            {v.expires_at ? (
              <Tooltip title={fmtAbs(v.expires_at)}> · {t("compliance.expires")} {fmtAge(v.expires_at)}</Tooltip>
            ) : (
              <> · {t("compliance.no_expiry")}</>
            )}
          </Text>
        </span>
      ),
    },
    ...(canWrite
      ? [{
          title: t("compliance.actions"),
          key: "actions",
          render: (_: unknown, v: Violation) => (
            <Button size="small" icon={<UndoOutlined />} loading={restoreMut.isPending} onClick={() => restoreMut.mutate(v)}>
              {t("compliance.restore")}
            </Button>
          ),
        }]
      : []),
  ];

  const ignoredCount = data?.ignored?.length ?? 0;

  return (
    <div>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}><SafetyOutlined /> {t("nav.compliance")}</Title>
        <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => refetch()}>
          {t("compliance.refresh")}
        </Button>
      </div>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label={t("compliance.servers_at_risk")} value={`${data?.servers_at_risk ?? 0} / ${data?.servers_total ?? 0}`} Icon={SafetyOutlined} iconColor={(data?.servers_at_risk ?? 0) ? "#d48806" : "#3f8600"} />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label={t("compliance.critical_violations")} value={data?.critical ?? 0} Icon={WarningOutlined} iconColor={(data?.critical ?? 0) ? "#cf1322" : "#3f8600"} />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label={t("compliance.total_violations")} value={data?.total ?? 0} Icon={FileTextOutlined} iconColor="#8c8c8c" />
        </Col>
      </Row>

      <Card>
        {data && data.total === 0 && !isLoading ? (
          <Empty description={t("compliance.fleet_is_compliant_no_policy_violations")} />
        ) : (
          <Table<Violation>
            scroll={{ x: "max-content" }}
            dataSource={data?.violations || []}
            columns={sortable(activeColumns)}
            rowKey={(r) => r.server_id + r.check}
            loading={isLoading}
            size="small"
            pagination={{ pageSize: 20, showSizeChanger: false }}
          />
        )}
      </Card>

      {ignoredCount > 0 && (
        <Card style={{ marginTop: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: showIgnored ? 12 : 0 }}>
            <Text strong>{t("compliance.ignored_violations")} ({ignoredCount})</Text>
            <span>
              <Switch size="small" checked={showIgnored} onChange={setShowIgnored} />{" "}
              <Text type="secondary">{t("compliance.show_ignored")}</Text>
            </span>
          </div>
          {showIgnored && (
            <Table<Violation>
              scroll={{ x: "max-content" }}
              dataSource={data?.ignored || []}
              columns={sortable(ignoredColumns)}
              rowKey={(r) => r.server_id + r.check}
              size="small"
              pagination={{ pageSize: 20, showSizeChanger: false }}
            />
          )}
        </Card>
      )}

      <Modal
        title={t("compliance.ignore_violation")}
        open={!!ignoring}
        onOk={submitIgnore}
        onCancel={() => { setIgnoring(null); form.resetFields(); }}
        okText={t("compliance.ignore")}
        destroyOnClose
      >
        {ignoring && (
          <Text type="secondary">
            {ignoring.server_name} · {CHECK_LABEL[ignoring.check] || ignoring.check}
          </Text>
        )}
        <Form form={form} layout="vertical" style={{ marginTop: 12 }}>
          <Form.Item name="reason" label={t("compliance.reason")} rules={[{ required: true, message: t("compliance.reason_required") }]}>
            <Input.TextArea rows={3} maxLength={400} placeholder={t("compliance.reason_placeholder")} />
          </Form.Item>
          <Form.Item name="expires_at" label={t("compliance.expiry_optional")} extra={t("compliance.expiry_help")}>
            <DatePicker style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
