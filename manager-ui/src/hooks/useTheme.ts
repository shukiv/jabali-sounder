// useTheme — AntD ConfigProvider configuration mirroring jabali2's muiTheme.
// Minimal: algorithm switch for dark/light, Inter font, red accent for
// selected menu items + tabs, gray-50 page bg in light mode.
import { useMemo } from "react";
import { theme } from "antd";
import type { ConfigProviderProps } from "antd";

import type { ThemeMode } from "../theme/ThemeModeContext";

export function useTheme(mode: ThemeMode): ConfigProviderProps {
  const accent = mode === "dark" ? "#ef4444" : "#dc2626";
  return useMemo<ConfigProviderProps>(
    () => ({
      theme: {
        algorithm:
          mode === "dark" ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          fontSize: 15,
          // WCAG AA: secondary text must hit 4.5:1 on the page bg (SND-62).
          colorTextSecondary: mode === "dark" ? "rgba(255, 255, 255, 0.72)" : "#595959",
          colorTextDescription: mode === "dark" ? "rgba(255, 255, 255, 0.72)" : "#595959",
          fontFamily:
            "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
          ...(mode === "light"
            ? { colorBgLayout: "#f9fafb" }
            : {}),
        },
        components: {
          Layout: {
            triggerBg: "transparent",
            ...(mode === "light" ? { bodyBg: "#f9fafb" } : {}),
          },
          Menu:
            mode === "dark"
              ? {
                  darkItemColor: "rgba(255, 255, 255, 0.85)",
                  darkItemSelectedBg: "#1f1f1f",
                  darkItemSelectedColor: accent,
                  darkItemHoverBg: "#1f1f1f",
                  darkItemHoverColor: "rgba(255, 255, 255, 0.85)",
                }
              : {
                  itemSelectedBg: "#f3f4f6",
                  itemSelectedColor: accent,
                  itemHoverBg: "#f3f4f6",
                  itemHoverColor: "rgba(0, 0, 0, 0.88)",
                },
          Tabs: {
            itemSelectedColor: accent,
            inkBarColor: accent,
            itemHoverColor: accent,
            itemActiveColor: accent,
          },
        },
      },
    }),
    [mode, accent],
  );
}
