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

import type { SemanticRole } from "@components/SemanticChip";
import type {
  ActivityKey,
  IssueComplexity,
  TimeCardState,
} from "@features/csm-timecards/types/timeCards";

/** A labelled activity row in the time-breakdown editor (log-time form only). */
export interface ActivityBucket {
  key: ActivityKey;
  label: string;
}

/**
 * The five fixed activity categories, in display order — the activity types
 * from the Time Management requirement (ISSU-009). Write-only: the backend
 * accepts these hours on create but never returns them on read.
 */
export const ACTIVITY_BUCKETS: ActivityBucket[] = [
  { key: "analysisDebugging", label: "Analysis and debugging" },
  { key: "reproduce", label: "Reproduce" },
  { key: "settingUp", label: "Setting up" },
  { key: "providingSolution", label: "Providing solution" },
  { key: "answering", label: "Answering" },
];

/** Issue-complexity options (the ServiceNow "Issue Complexity" field). Write-only. */
export const ISSUE_COMPLEXITY_OPTIONS: readonly IssueComplexity[] = [
  "N/A",
  "Low",
  "Medium",
  "High",
];
export const DEFAULT_ISSUE_COMPLEXITY: IssueComplexity = "N/A";

/**
 * Whether logged time is billable to the customer by default (ISSU-009). Most
 * support work is billable; the engineer can flip it per card.
 */
export const DEFAULT_BILLABLE = true;

/** Short label for a card's billable classification. */
export function billableLabel(billable: boolean): string {
  return billable ? "Billable" : "Non-billable";
}

/** Character caps mirroring the ServiceNow form. */
export const WORK_LOG_MAX = 4000;
export const LEAD_COMMENT_MAX = 500;

/**
 * Label + semantic colour for each card state (drives the status chip).
 * `recalled` and `processed` appear in the backend's state enum but are
 * unreachable via the portal's API today — kept here for type completeness.
 * Every state renders `outlined` — a solid-filled chip on every row read as
 * too much colour on the list; `submitted` additionally uses the neutral
 * `default` role rather than the info blue.
 */
export const TIME_CARD_STATE_META: Record<
  TimeCardState,
  { label: string; role: SemanticRole; variant?: "filled" | "outlined" }
> = {
  pending: { label: "Pending", role: "default", variant: "outlined" },
  submitted: { label: "Submitted", role: "default", variant: "outlined" },
  approved: { label: "Approved", role: "success", variant: "outlined" },
  rejected: { label: "Rejected", role: "error", variant: "outlined" },
  recalled: { label: "Recalled", role: "warning", variant: "outlined" },
  processed: { label: "Processed", role: "success", variant: "outlined" },
};

/**
 * Role that grants approve/reject on time cards, matched (case-insensitively)
 * against `roles` from `GET /users/me`.
 */
export const TIMECARD_APPROVER_GROUP = "timecard_approver";

/**
 * Role that grants time-card admin (approve by exception), matched against
 * `roles` from `GET /users/me`.
 * Mapped to the real "admin" role for now so approver/admin-gated paths are
 * testable; revisit once dedicated time-card roles are provisioned.
 */
export const TIMECARD_ADMIN_GROUP = "admin";
