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

import type { BeCaseIssueType } from "@api/backend/types";

// Single source of truth for issue-type ordering and labels — shared by the
// case-create form's dropdown and the Cases list's optional "Issue type"
// column, so the two can never drift into showing different wording for the
// same value.

/** All issue types, in the order they appear in the create-case dropdown. */
export const ALL_ISSUE_TYPES: BeCaseIssueType[] = [
  "total_outage",
  "partial_outage",
  "performance_degradation",
  "error",
  "security_or_compliance",
  "question",
];

/** Human-readable label per issue type. */
export const ISSUE_TYPE_LABEL: Record<BeCaseIssueType, string> = {
  total_outage: "Total outage",
  partial_outage: "Partial outage",
  performance_degradation: "Performance degradation",
  error: "Error",
  security_or_compliance: "Security / compliance",
  question: "Question",
};
