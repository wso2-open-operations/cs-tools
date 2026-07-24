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

import type { ChipProps } from "@wso2/oxygen-ui";
import type { ChangeRequestFilterableState, ChangeRequestImpact, ChangeRequestState } from "@src/types";

// Direct port of the webapp's utils/changeRequests.ts label/color maps
// (apps/csm-portal/webapp/src/features/csm-operations/utils/changeRequests.ts).

export const CHANGE_REQUEST_STATE_LABELS: Record<ChangeRequestState, string> = {
  new: "New",
  assess: "Assess",
  authorize: "Authorize",
  customer_approval: "Customer Approval",
  scheduled: "Scheduled",
  implement: "Implement",
  review: "Review",
  customer_review: "Customer Review",
  rollback: "Rollback",
  closed: "Closed",
  canceled: "Canceled",
};

export const CHANGE_REQUEST_STATE_COLORS: Record<ChangeRequestState, NonNullable<ChipProps["color"]>> = {
  new: "default",
  assess: "info",
  authorize: "info",
  customer_approval: "info",
  scheduled: "info",
  implement: "warning",
  review: "info",
  customer_review: "info",
  rollback: "error",
  closed: "success",
  canceled: "error",
};

export const CHANGE_REQUEST_IMPACT_LABELS: Record<ChangeRequestImpact, string> = {
  high: "High",
  medium: "Medium",
  low: "Low",
};

export const CHANGE_REQUEST_IMPACT_COLORS: Record<ChangeRequestImpact, NonNullable<ChipProps["color"]>> = {
  high: "error",
  medium: "warning",
  low: "default",
};

// The backend documents `state`/`impact` on the search response as loose strings (not a strict
// enum, unlike the search *filter*'s enum) — a value that doesn't match the maps above (different
// casing, a value not in our list) would otherwise render as a blank Chip. Falls back to a
// humanized version of the raw value instead, mirroring the webapp's own
// changeRequestStateLabel/changeRequestImpactLabel.
function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

export function changeRequestStateLabel(state?: string | null): string {
  if (!state) return "—";
  return CHANGE_REQUEST_STATE_LABELS[state as ChangeRequestState] ?? humanize(state);
}

export function changeRequestStateColor(state?: string | null): NonNullable<ChipProps["color"]> {
  if (!state) return "default";
  return CHANGE_REQUEST_STATE_COLORS[state as ChangeRequestState] ?? "default";
}

export function changeRequestImpactLabel(impact?: string | null): string {
  if (!impact) return "—";
  return CHANGE_REQUEST_IMPACT_LABELS[impact as ChangeRequestImpact] ?? humanize(impact);
}

export function changeRequestImpactColor(impact?: string | null): NonNullable<ChipProps["color"]> {
  if (!impact) return "default";
  return CHANGE_REQUEST_IMPACT_COLORS[impact as ChangeRequestImpact] ?? "default";
}

// Only the 8 states the backend's search filter actually accepts (new/assess/authorize are
// returnable but not filterable — see changeRequest.dto.ts).
export const CHANGE_REQUEST_FILTERABLE_STATES: ChangeRequestFilterableState[] = [
  "customer_approval",
  "scheduled",
  "implement",
  "review",
  "customer_review",
  "rollback",
  "closed",
  "canceled",
];

export const CHANGE_REQUEST_IMPACTS: ChangeRequestImpact[] = ["high", "medium", "low"];
