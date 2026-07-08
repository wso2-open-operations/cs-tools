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

import type { BeCaseCause, BeCaseResolutionCode } from "@api/backend/types";

/** All backend resolution codes, in the order declared in openapi.yaml. */
export const RESOLUTION_CODES: BeCaseResolutionCode[] = [
  "SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED",
  "SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT",
  "SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET",
  "SOLVED_WORKAROUND_PROVIDED",
  "SOLVED_BY_CUSTOMER",
  "CONSIDERED_FOR_ROADMAP",
  "INCONCLUSIVE_OUT_OF_SCOPE",
  "INCONCLUSIVE_CANNOT_REPRODUCE",
  "INCONCLUSIVE_NO_WORKAROUND",
  "DUPLICATE_ISSUE",
  "VOIDED_CANCELED",
  "ON_HOLD",
  "CONSIDERED_FOR_ROADMAP_ALT",
  "SOLVED_FIXED_THE_ISSUE",
  "SOLVED_WORKAROUND_PROVIDED_ALT",
  "SOLVED_BY_CONTRIBUTOR",
  "SOLVED_BY_NOVERA",
  "ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS",
];

/** All backend root-cause categories, in the order declared in openapi.yaml. */
export const CASE_CAUSES: BeCaseCause[] = [
  "USER_MISUNDERSTANDING_CONCEPTS",
  "USER_MISUNDERSTANDING_DOCUMENTATION",
  "USER_NOT_FOLLOWING_DOCUMENTATION",
  "USER_MISTAKE",
  "SOLUTION_PROBLEMATIC_SOLUTION_ARCHITECTURE",
  "SOLUTION_PROBLEMATIC_CODE",
  "APPLICATION_BUG",
  "APPLICATION_MISLEADING_UX_UI",
  "APPLICATION_LIMITATION",
  "APPLICATION_MISSING_FEATURE",
  "APPLICATION_DOCUMENTATION_GAP",
  "APPLICATION_DOCUMENTATION_ERROR",
  "INFRASTRUCTURE_CUSTOMERS_SIDE",
  "INFRASTRUCTURE_SAAS_SIDE_NOT_ENOUGH",
  "INFRASTRUCTURE_SAAS_SIDE_OTHER",
  "UNKNOWN",
];

/**
 * Title-cases an UPPER_SNAKE_CASE backend enum value for display, e.g.
 * "SOLVED_BY_CUSTOMER" -> "Solved by customer". Mirrors `humanizeState`
 * (abtDashboard.ts) so an enum value added on the backend still renders
 * readably with no frontend change required.
 */
export function humanizeResolutionEnum(value: string): string {
  const words = value.toLowerCase().split("_").filter(Boolean);
  return words
    .map((w, i) => (i === 0 ? w.charAt(0).toUpperCase() + w.slice(1) : w))
    .join(" ");
}
