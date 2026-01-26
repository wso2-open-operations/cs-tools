import { MessageSquare, OctagonAlert, RefreshCcw, Settings, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { ItemType } from "./ItemCard";
import { colors } from "@wso2/oxygen-ui";

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
