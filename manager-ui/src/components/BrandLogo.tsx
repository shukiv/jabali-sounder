import { Space, Typography } from "antd";

import jabaliLogo from "../assets/jabali-sounder.svg";
import type { ThemeMode } from "../theme/ThemeModeContext";

const { Text, Title } = Typography;

interface BrandLogoProps {
  mode: ThemeMode;
  size?: "header" | "footer" | "login";
}

export function BrandLogo({ mode, size = "header" }: BrandLogoProps) {
  const logoHeight = size === "login" ? 34 : size === "footer" ? 18 : 30;
  const titleLevel = size === "login" ? 3 : 4;
  const textStyle = {
    margin: 0,
    lineHeight: 1,
    color: mode === "dark" ? "#fff" : "#111827",
  };

  return (
    <Space size={10} align="center">
      <img
        src={jabaliLogo}
        alt=""
        aria-hidden="true"
        style={{
          display: "block",
          width: "auto",
          height: logoHeight,
          filter: mode === "dark" ? "invert(1)" : "none",
          transform: "translateY(2px)",
        }}
      />
      {size === "footer" ? (
        <Text strong style={textStyle}>
          Jabali Sounder
        </Text>
      ) : (
        <Title level={titleLevel} style={textStyle}>
          Jabali Sounder
        </Title>
      )}
    </Space>
  );
}
