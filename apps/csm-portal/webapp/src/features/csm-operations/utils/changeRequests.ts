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

import type {
  BeChangeRequestImpact,
  BeChangeRequestState,
} from "@api/backend/types";

type ChipColor = "default" | "info" | "warning" | "success" | "error";

const STATE_LABEL: Record<BeChangeRequestState, string> = {
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

// State chip colour: approvals/reviews are in-flight (info), implement is active
// (warning), rollback/cancel are problem states (error), closed is terminal-good.
const STATE_COLOR: Record<BeChangeRequestState, ChipColor> = {
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

const IMPACT_LABEL: Record<BeChangeRequestImpact, string> = {
  high: "High",
  medium: "Medium",
  low: "Low",
};

const IMPACT_COLOR: Record<BeChangeRequestImpact, ChipColor> = {
  high: "error",
  medium: "warning",
  low: "default",
};

/** All CR states, for a filter control. */
export const CHANGE_REQUEST_STATES = Object.keys(STATE_LABEL) as BeChangeRequestState[];

/** All CR impact levels, for a filter control. */
export const CHANGE_REQUEST_IMPACTS = Object.keys(IMPACT_LABEL) as BeChangeRequestImpact[];

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

export function changeRequestStateLabel(state?: string | null): string {
  if (!state) return "—";
  return STATE_LABEL[state as BeChangeRequestState] ?? humanize(state);
}

export function changeRequestStateColor(state?: string | null): ChipColor {
  return STATE_COLOR[state as BeChangeRequestState] ?? "default";
}

export function changeRequestImpactLabel(impact?: string | null): string {
  if (!impact) return "—";
  return IMPACT_LABEL[impact as BeChangeRequestImpact] ?? humanize(impact);
}

export function changeRequestImpactColor(impact?: string | null): ChipColor {
  return IMPACT_COLOR[impact as BeChangeRequestImpact] ?? "default";
}

// Approval-stage / approver status labels and colours. The backend passes
// these through from the data source without validating them (see
// `BeChangeRequestApprover.status`), so both maps are deliberately partial —
// unrecognized values fall back to a humanized version of the raw string
// rather than crashing or rendering nothing.
const APPROVAL_STATUS_LABEL: Record<string, string> = {
  APPROVED: "Approved",
  REJECTED: "Rejected",
  PENDING: "Pending",
  REQUESTED: "Requested",
  NOT_REQUIRED: "Not required",
  CANCELLED: "Cancelled",
  NO_CONSENSUS: "No consensus",
};

const APPROVAL_STATUS_COLOR: Record<string, ChipColor> = {
  APPROVED: "success",
  REJECTED: "error",
  PENDING: "warning",
  REQUESTED: "warning",
  NOT_REQUIRED: "default",
  CANCELLED: "default",
  NO_CONSENSUS: "error",
};

export function approvalStatusLabel(status?: string | null): string {
  if (!status) return "—";
  return APPROVAL_STATUS_LABEL[status.toUpperCase()] ?? humanize(status.toLowerCase());
}

export function approvalStatusColor(status?: string | null): ChipColor {
  if (!status) return "default";
  return APPROVAL_STATUS_COLOR[status.toUpperCase()] ?? "default";
}

export interface ChangeRequestFilters {
  search: string;
  states: BeChangeRequestState[];
  impacts: BeChangeRequestImpact[];
  /** YYYY-MM-DD local date string, or empty. */
  closedStartDate: string;
  /** YYYY-MM-DD local date string, or empty. */
  closedEndDate: string;
}

export const DEFAULT_CR_FILTERS: ChangeRequestFilters = {
  search: "",
  states: [],
  impacts: [],
  closedStartDate: "",
  closedEndDate: "",
};

/** Count non-search active filters (used for the badge on the Filters button). */
export function countActiveCRFilters(filters: ChangeRequestFilters): number {
  return (
    (filters.states.length > 0 ? 1 : 0) +
    (filters.impacts.length > 0 ? 1 : 0) +
    (filters.closedStartDate ? 1 : 0) +
    (filters.closedEndDate ? 1 : 0)
  );
}
