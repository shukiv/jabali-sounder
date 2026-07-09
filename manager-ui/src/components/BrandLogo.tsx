import { Typography } from "antd";

import jabaliLogo from "../assets/jabali-sounder.svg";
import type { ThemeMode } from "../theme/ThemeModeContext";

const { Text, Title } = Typography;

interface BrandLogoProps {
  mode: ThemeMode;
  size?: "header" | "footer" | "login";
}

export function BrandLogo({ mode, size = "header" }: BrandLogoProps) {
  const logoHeight = size === "login" ? 52 : size === "footer" ? 24 : 54;
  const titleLevel = size === "footer" ? 4 : 3;
  const textStyle = {
    margin: 0,
    lineHeight: 1,
    color: mode === "dark" ? "#fff" : "#111827",
  };

  return (
    // Plain flex with align-items:center guarantees true vertical centering.
    // antd <Space> was baseline-aligning the wide logo against the text.
    <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
      <img
        src={jabaliLogo}
        alt=""
        aria-hidden="true"
        style={{
          display: "block",
          width: "auto",
          height: logoHeight,
          filter: mode === "dark" ? "invert(1)" : "none",
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
    </div>
  );
}
