import { type ChipProps, colors } from "@wso2/oxygen-ui";
import {
  Briefcase,
  type LucideIcon,
  Megaphone,
  MessageSquare,
  OctagonAlert,
  RefreshCcw,
  Settings,
  Shield,
} from "@wso2/oxygen-ui-icons-react";

import { CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

export const SUPPORT_TAB_VIEW_CONFIG: Record<CaseType, { title: string; subtitle: string }> = {
  default_case: { title: "Outstanding Cases", subtitle: "Active support tickets" },
  chat: { title: "Chat History", subtitle: "Recent Novera conversations" },
  service_request: {
    title: "Service Requests",
    subtitle: "Managed cloud service requests",
  },
  change_request: {
    title: "Change Requests",
    subtitle: "Scheduled and pending changes",
  },
  security_report_analysis: {
    title: "Security Report Analysis",
    subtitle: "Security findings, assessments, and reviews",
  },
  engagement: {
    title: "Engagements",
    subtitle: "Ongoing and completed client engagements",
  },
  announcement: {
    title: "Announcements",
    subtitle: "View and manage announcements for your project",
  },
};

export const CASE_TYPE_CONFIGS: Record<CaseType, { icon: LucideIcon; color: string }> = {
  [CASE_TYPES.DEFAULT]: {
    icon: OctagonAlert,
    color: colors.red[500],
  },

  [CASE_TYPES.CHAT]: {
    icon: MessageSquare,
    color: colors.blue[500],
  },

  [CASE_TYPES.SERVICE_REQUEST]: {
    icon: Settings,
    color: colors.purple[500],
  },

  [CASE_TYPES.CHANGE_REQUEST]: {
    icon: RefreshCcw,
    color: colors.cyan[500],
  },

  [CASE_TYPES.SECURITY_REPORT_ANALYSIS]: {
    icon: Shield,
    color: colors.orange[500],
  },

  [CASE_TYPES.ENGAGEMENT]: {
    icon: Briefcase,
    color: colors.grey[600],
  },

  [CASE_TYPES.ANNOUNCEMENT]: {
    icon: Megaphone,
    color: colors.red[500],
  },
};

export const PRIORITY_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  13: "success",
  12: "info",
  11: "primary",
  10: "primary",
};

export const IMPACT_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  1: "error",
  2: "warning",
  3: "success",
};

export const CASE_STATUS_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  1: "info",
  10: "primary",
  18: "success",
  1003: "info",
  6: "default",
  3: "info",
  1006: "info",
};

export const CONVERSATION_STATUS_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  1: "info",
  2: "success",
  3: "success",
  4: "primary",
  5: "default",
};

export const CHANGE_REQUEST_STATUS_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  "-5": "default",
  "-4": "info",
  "-3": "warning",
  "5": "warning",
  "-2": "primary",
  "-1": "primary",
  "0": "info",
  "1": "info",
  "2": "error",
  "3": "success",
  "4": "default",
};

export const ITEMS_LIST_FILTERABLE_CASE_TYPES: CaseType[] =
  (Object.values(CASE_TYPES) as CaseType[]).filter(
    (type) => !([CASE_TYPES.ANNOUNCEMENT] as CaseType[]).includes(type)
  );
