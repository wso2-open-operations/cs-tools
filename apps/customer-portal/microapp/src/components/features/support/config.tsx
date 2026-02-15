import { MessageSquare, OctagonAlert, RefreshCcw, Settings, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { ItemType, Priority, Status } from "./ItemCard";
import { colors, type ChipProps } from "@wso2/oxygen-ui";

export const TYPE_CONFIG: Record<ItemType, { icon: LucideIcon; color: string }> = {
  case: {
    icon: OctagonAlert,
    color: colors.red[500],
  },
  chat: {
    icon: MessageSquare,
    color: colors.blue[500],
  },
  service: {
    icon: Settings,
    color: colors.purple[500],
  },
  change: {
    icon: RefreshCcw,
    color: colors.cyan[500],
  },
};

export const PRIORITY_CHIP_COLOR_CONFIG: Record<Priority, ChipProps["color"]> = {
  low: "success",
  medium: "info",
  high: "primary",
};

export const STATUS_CHIP_COLOR_CONFIG: Record<Status, ChipProps["color"]> = {
  "in progress": "info",
  open: "primary",
  resolved: "success",
  waiting: "info",
  closed: "default",
  active: "info",
  scheduled: "warning",
  approved: "success",
  draft: "default",
  rejected: "error",
  "pending approval": "warning",
};
