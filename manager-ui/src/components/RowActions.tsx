// RowActions — the canonical per-row actions cell. Renders the FIRST visible
// action as a full RowActionButton (icon + text) and collapses the rest into an
// overflow ("...") menu whose items keep their text + icon. Keeps dense tables
// narrow and uniform. Ported from jabali2 panel-ui; uses AntD's EllipsisOutlined
// (horizontal dots) for the overflow trigger to match the panel.
//
// Destructive actions pass `confirm` (a Modal.confirm pops before onClick) so
// the menu can hold a Delete without nesting a Popconfirm inside the dropdown.

import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";
import { Dropdown, Modal, Space, Tooltip } from "antd";
import { EllipsisOutlined } from "@ant-design/icons";
import { RowActionButton } from "./RowActionButton";

export interface RowAction {
  /** Stable key (menu item id). */
  key: string;
  /** Full text — shown on the first button AND as the menu item label. */
  label: string;
  /** Required icon (matches RowActionButton's contract). */
  icon: ReactNode;
  onClick?: () => void;
  danger?: boolean;
  disabled?: boolean;
  loading?: boolean;
  /** When true the action is omitted entirely (e.g. capability gate). */
  hidden?: boolean;
  /** Tooltip for the first button (e.g. why it's disabled). */
  tooltip?: string;
  /** When set, a Modal.confirm pops before onClick runs. */
  confirm?: { title?: string; description?: ReactNode; okText?: string };
}

function run(a: RowAction) {
  if (!a.confirm) {
    a.onClick?.();
    return;
  }
  Modal.confirm({
    title: a.confirm.title ?? "Are you sure?",
    content: a.confirm.description,
    okText: a.confirm.okText ?? "OK",
    okButtonProps: { danger: a.danger },
    onOk: a.onClick,
  });
}

export function RowActions({ actions }: { actions: RowAction[] }) {
  const { t } = useTranslation();
  const visible = actions.filter((a) => !a.hidden);
  if (visible.length === 0) {
    return null;
  }
  const [first, ...rest] = visible;

  const firstButton = (
    <RowActionButton
      icon={first.icon}
      danger={first.danger}
      disabled={first.disabled}
      loading={first.loading}
      onClick={() => run(first)}
    >
      {first.label}
    </RowActionButton>
  );

  return (
    <Space size={4}>
      {first.tooltip ? <Tooltip title={first.tooltip}>{firstButton}</Tooltip> : firstButton}
      {rest.length > 0 && (
        <Dropdown
          trigger={["click"]}
          menu={{
            items: rest.map((a) => ({
              key: a.key,
              label: a.label,
              icon: a.icon,
              danger: a.danger,
              disabled: a.disabled,
              onClick: () => run(a),
            })),
          }}
        >
          <RowActionButton icon={<EllipsisOutlined />} color="default" aria-label={t("actions.more_actions")} />
        </Dropdown>
      )}
    </Space>
  );
}
