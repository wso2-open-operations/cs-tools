import type { CaseType } from "@shared/types";

import { CASE_TYPES } from "./app.constants";

export const SEARCH_PLACEHOLDER_CONFIG: Record<CaseType, string> = {
  [CASE_TYPES.DEFAULT]: "Search Cases",
  [CASE_TYPES.CHAT]: "Search Chats",
  [CASE_TYPES.SERVICE_REQUEST]: "Search Service Requests",
  [CASE_TYPES.CHANGE_REQUEST]: "Search Change Requests",
  [CASE_TYPES.SECURITY_REPORT_ANALYSIS]: "Search Security Report Analysis",
  [CASE_TYPES.ENGAGEMENT]: "Search Engagement",
  [CASE_TYPES.ANNOUNCEMENT]: "Search Announcements",
};
