// StatCard — summary/stat card: a tinted icon chip, a colored label, a large
// value, and an optional subtitle. Accent comes from `iconColor`; the chip tint
// defaults to a translucent shade of it. With `to`, the whole card links.
// Ported from the jabali2 panel-ui component.
import { Card, Grid, Typography } from "antd";
import type { ComponentType, CSSProperties, ReactNode } from "react";
import { Link } from "react-router";

export interface StatCardProps {
  label: ReactNode;
  value: ReactNode;
  subtitle?: ReactNode;
  /** Accent colour for the label + icon, e.g. "#1677ff". */
  iconColor: string;
  /** Chip background; defaults to a translucent shade of iconColor. */
  iconBg?: string;
  /** Icon as a component… */
  Icon?: ComponentType<{ style?: CSSProperties }>;
  /** …or as a ready node. */
  icon?: ReactNode;
  /** When set, the card is a hoverable link to this route. */
  to?: string;
}

function tintFor(color: string): string {
  return /^#[0-9a-fA-F]{6}$/.test(color) ? `${color}22` : color;
}

export function StatCard({ label, value, subtitle, iconColor, iconBg, Icon, icon, to }: StatCardProps) {
  const screens = Grid.useBreakpoint();
  const compact = !!screens.md && !screens.lg;
  const narrow = !screens.sm;
  const size = narrow ? 36 : compact ? 40 : 48;
  const gap = narrow ? 8 : 12;

  const body = (
    <Card hoverable={!!to} size="small" styles={{ body: { padding: narrow ? 10 : compact ? 12 : 16 } }}>
      <div style={{ display: "flex", alignItems: "center", gap }}>
        <div
          style={{
            flex: "0 0 auto",
            width: size,
            height: size,
            borderRadius: 12,
            background: iconBg ?? tintFor(iconColor),
            color: iconColor,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: compact ? 18 : 22,
          }}
        >
          {Icon ? <Icon style={{ fontSize: compact ? 18 : 22 }} /> : icon}
        </div>
        <div style={{ minWidth: 0, overflowWrap: "break-word", wordBreak: "normal" }}>
          <div style={{ color: iconColor, fontSize: 13, fontWeight: 600, marginBottom: 2 }}>{label}</div>
          <div style={{ fontSize: narrow ? 18 : compact ? 20 : 26, fontWeight: 700, lineHeight: 1.1 }}>{value}</div>
          {subtitle != null && (
            <Typography.Text type="secondary" style={{ fontSize: 12, display: "block" }}>
              {subtitle}
            </Typography.Text>
          )}
        </div>
      </div>
    </Card>
  );

  return to ? (
    <Link to={to} style={{ display: "block", color: "inherit" }}>
      {body}
    </Link>
  ) : (
    body
  );
}
