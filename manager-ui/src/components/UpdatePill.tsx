import { Button } from "antd";
import { CloudDownloadOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router";
import { useVersion } from "../hooks/useVersion";

// UpdatePill is a compact header affordance shown only when a newer release is
// available; it routes to Settings where the update can be viewed/installed.
export default function UpdatePill() {
  const nav = useNavigate();
  const { data } = useVersion();
  if (!data?.update_available) return null;
  return (
    <Button
      type="text"
      size="small"
      icon={<CloudDownloadOutlined style={{ color: "#d48806" }} />}
      onClick={() => nav("/settings")}
      title={`Update available: ${data.latest}`}
      style={{ color: "#d48806" }}
    >
      Update
    </Button>
  );
}
