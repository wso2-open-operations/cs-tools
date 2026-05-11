import type { CaseType } from "@shared/types";

export const OUTSTANDING_SERVICE_REQUESTS_TITLE = "Outstanding Service Requests";
export const OUTSTANDING_CHANGE_REQUESTS_TITLE = "Outstanding Change Requests";
export const OUTSTANDING_CASES_BY_SEVERITY_TITLE = (label: string) => `Outstanding ${label} Cases`;

export const CLOSED_ITEMS_TITLE = "Closed Items (30d)";
export const OUTSTANDING_ITEMS_TITLE = "Outstanding Items";
export const ACTION_REQUIRED_ITEMS_TITLE = "Action Required Items";

export const DASHBOARD_METRIC_ACTION_REQUIRED = "Action Required";
export const DASHBOARD_METRIC_OUTSTANDING = "Outstanding";
export const DASHBOARD_METRIC_CLOSED = "Closed (30d)";
export const DASHBOARD_METRIC_AVG_RESPONSE_TIME = "Average Response Time";
export const DASHBOARD_WIDGET_OUTSTANDING_SUPPORT_CASES = "Outstanding Support Cases";
export const DASHBOARD_WIDGET_OUTSTANDING_OPERATIONS = "Outstanding Operations";
export const DASHBOARD_WIDGET_OUTSTANDING_ENGAGEMENTS = "Outstanding Engagements";

export const SERVICE_REQUESTS_LABEL = "Service Requests"; // TODO: Requires pointing to a global constant instead
export const CHANGE_REQUESTS_LABEL = "Change Requests"; // TODO: Requires pointing to a global constant instead

export const CASE_TYPE_PLURAL_LABEL: Record<CaseType, string> = {
  default_case: "Cases",
  chat: "Chats",
  service_request: "Service Requests",
  change_request: "Change Requests",
  security_report_analysis: "Security Report Analysis",
  engagement: "Engagements",
  announcement: "Announcements",
};

export const STRING_OVERRIDES = {
  // Engagement Type Labels Overrides
  "New Feature / Improvement": "Improvement",

  // Severity Type Names
  "Low (P4)": "S4",
  "Medium (P3)": "S3",
  "High (P2)": "S2",
  "Critical (P1)": "S1",
  "Catastrophic (P0)": "S0",
};
