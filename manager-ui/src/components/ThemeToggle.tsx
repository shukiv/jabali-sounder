// ThemeToggle — one-click sun/moon switch. Mirrors jabali2.
import { MoonOutlined, SunOutlined } from "@ant-design/icons";
import { Button, Tooltip } from "antd";

import { useThemeMode } from "../theme/ThemeModeContext";

export function ThemeToggle() {
  const { mode, toggle } = useThemeMode();
  const next = mode === "dark" ? "light" : "dark";

  return (
    <Tooltip title={`Switch to ${next} mode`}>
      <Button
        type="text"
        aria-label={`Switch to ${next} mode`}
        icon={mode === "dark" ? <SunOutlined /> : <MoonOutlined />}
        onClick={toggle}
        style={{
          width: 40,
          height: 40,
          padding: 0,
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      />
    </Tooltip>
  );
}
