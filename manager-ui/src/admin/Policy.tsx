import { Card, Table, Typography, Tag, Button, Empty, Row, Col } from "antd";
import { ReloadOutlined, SafetyOutlined, WarningOutlined, FileTextOutlined } from "@ant-design/icons";
import { StatCard } from "../components/StatCard";
import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";

const { Title, Text } = Typography;

interface Violation {
  server_id: string;
  server_name: string;
  check: string;
  severity: "info" | "warning" | "critical";
  message: string;
}

interface PolicyResp {
  violations: Violation[];
  total: number;
  by_check: Record<string, number>;
  servers_total: number;
  servers_at_risk: number;
}

const SEV_COLOR: Record<string, string> = { info: "blue", warning: "gold", critical: "red" };
const CHECK_LABEL: Record<string, string> = {
  insecure_tls: "Insecure TLS",
  credential_invalid: "Invalid credential",
  unreachable: "Unreachable",
  cert_expiring: "Cert expiring",
  version_drift: "Version drift",
};

// Policy shows fleet compliance drift (SND-32): weak TLS, invalid credentials,
// unreachability, cert expiry, and version drift from the fleet majority.
export default function Policy() {
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ["policy"],
    queryFn: async () => (await apiClient.get<PolicyResp>("/admin/policy")).data,
    refetchInterval: 60_000,
  });

  const columns = [
    { title: "Server", dataIndex: "server_name", key: "server_name" },
    {
      title: "Check",
      dataIndex: "check",
      key: "check",
      render: (c: string) => <Tag>{CHECK_LABEL[c] || c}</Tag>,
    },
    {
      title: "Severity",
      dataIndex: "severity",
      key: "severity",
      render: (s: string) => <Tag color={SEV_COLOR[s] || "default"}>{s}</Tag>,
    },
    { title: "Detail", dataIndex: "message", key: "message", render: (m: string) => <Text type="secondary">{m}</Text> },
  ];

  const critical = (data?.violations || []).filter((v) => v.severity === "critical").length;

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}><SafetyOutlined /> Compliance</Title>
        <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => refetch()}>
          Refresh
        </Button>
      </div>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label="Servers at risk" value={`${data?.servers_at_risk ?? 0} / ${data?.servers_total ?? 0}`} Icon={SafetyOutlined} iconColor={(data?.servers_at_risk ?? 0) ? "#d48806" : "#3f8600"} />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label="Critical violations" value={critical} Icon={WarningOutlined} iconColor={critical ? "#cf1322" : "#3f8600"} />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatCard label="Total violations" value={data?.total ?? 0} Icon={FileTextOutlined} iconColor="#8c8c8c" />
        </Col>
      </Row>
      <Card>
        {data && data.total === 0 && !isLoading ? (
          <Empty description="Fleet is compliant — no policy violations" />
        ) : (
          <Table<Violation>
            dataSource={data?.violations || []}
            columns={columns}
            rowKey={(r) => r.server_id + r.check}
            loading={isLoading}
            size="small"
            pagination={{ pageSize: 20, showSizeChanger: false }}
          />
        )}
      </Card>
    </div>
  );
}
