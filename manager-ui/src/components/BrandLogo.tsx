import { Typography } from "antd";

import jabaliLogo from "../assets/jabali-sounder.svg";
import type { ThemeMode } from "../theme/ThemeModeContext";

const { Text, Title } = Typography;

interface BrandLogoProps {
  mode: ThemeMode;
  size?: "header" | "footer" | "login";
}

export function BrandLogo({ mode, size = "header" }: BrandLogoProps) {
  // Login: a larger logo stacked above the product name (SND-16).
  if (size === "login") {
    return (
      <div className="login-logo" style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 10 }}>
        <img
          src={jabaliLogo}
          alt=""
          aria-hidden="true"
          style={{
            display: "block",
            width: "auto",
            height: 96,
            filter: mode === "dark" ? "invert(1)" : "none",
          }}
        />
        <Title
          level={2}
          style={{ margin: 0, lineHeight: 1.1, fontSize: 34, fontWeight: 700, textAlign: "center", color: mode === "dark" ? "#fff" : "#111827" }}
        >
          Jabali Sounder
        </Title>
      </div>
    );
  }

  const logoHeight = size === "footer" ? 24 : 54;
  const textStyle = {
    margin: 0,
    lineHeight: 1,
    whiteSpace: "nowrap" as const,
    fontSize: size === "footer" ? 16 : 30,
    fontWeight: 700,
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
        <span className="brand-wordmark" style={{ ...textStyle, display: "inline-block" }}>
          Jabali Sounder
        </span>
      )}
    </div>
  );
}
