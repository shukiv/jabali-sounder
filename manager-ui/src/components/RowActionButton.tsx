// RowActionButton — the canonical per-row action button. Filled style,
// required icon, default color="primary". The only button shape that should
// appear inside a table's "Actions" column. Ported from the jabali2 panel-ui
// pattern so Sounder's tables read the same way.
//
// Filled (AntD `variant="filled"`) gives a single visual class — primary =
// tinted, danger = red — so dense tables scan cleanly. Icon is required.

import type { ButtonProps } from "antd";
import { Button } from "antd";
import { forwardRef, type ReactNode, type Ref } from "react";

export interface RowActionButtonProps
  extends Omit<ButtonProps, "type" | "variant" | "color" | "icon"> {
  /** Required — every row action carries an icon. */
  icon: ReactNode;
  /** Optional label; omit for icon-only buttons. */
  children?: ReactNode;
  /** Override the default `color="primary"` (e.g. `default`). */
  color?: ButtonProps["color"];
}

// forwardRef so AntD <Dropdown>/<Tooltip>/<Popconfirm> can anchor and inject
// onClick/aria props onto the underlying Button.
export const RowActionButton = forwardRef<HTMLButtonElement, RowActionButtonProps>(
  function RowActionButton(
    { icon, children, color = "primary", ...rest },
    ref: Ref<HTMLButtonElement>,
  ) {
    return (
      <Button
        ref={ref}
        variant="filled"
        color={rest.danger ? "danger" : color}
        icon={icon}
        {...rest}
      >
        {children}
      </Button>
    );
  },
);
