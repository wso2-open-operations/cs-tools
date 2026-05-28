// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
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

export const CASE_TYPE_PLURAL_LABELS: Record<CaseType, string> = {
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
