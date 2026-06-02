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
  CsmEngagementDeliveryMode,
  CsmEngagementDeliverableStatus,
  CsmEngagementHealth,
  CsmEngagementPaymentType,
  CsmEngagementStage,
  CsmEngagementState,
  CsmEngagementTaskState,
  CsmEngagementType,
} from "@features/csm-engagements/types/csmEngagements";

export const ENGAGEMENT_TYPE_LABEL: Record<CsmEngagementType, string> = {
  customer_onboarding: "Customer Onboarding",
  migration: "Migration",
  qsp: "QSP",
  enterprise_csm_tam: "Enterprise CSM / TAM",
  consultancy: "Consultancy",
  training: "Training",
  architecture_review: "Architecture Review",
  firefighting: "Firefighting",
};

export const ENGAGEMENT_STATE_LABEL: Record<CsmEngagementState, string> = {
  new: "New",
  requested: "Requested",
  in_progress: "In progress",
  on_hold: "On hold",
  completed: "Completed",
  cancelled: "Cancelled",
};

export const ENGAGEMENT_STAGE_LABEL: Record<CsmEngagementStage, string> = {
  pre_engagement: "Pre-engagement",
  planning: "Planning & Procurement",
  execution: "Execution",
  warranty: "Warranty",
  post_engagement: "Post-engagement",
  closure: "Closure",
};

export const ENGAGEMENT_DELIVERY_LABEL: Record<CsmEngagementDeliveryMode, string> = {
  onsite: "Onsite",
  remote: "Remote",
  hybrid: "Hybrid",
};

export const ENGAGEMENT_PAYMENT_TYPE_LABEL: Record<CsmEngagementPaymentType, string> = {
  paid: "Paid",
  foc: "Free of charge",
};

export const ENGAGEMENT_HEALTH_LABEL: Record<CsmEngagementHealth, string> = {
  green: "On track",
  amber: "At risk",
  red: "Off track",
};

export const ENGAGEMENT_TASK_STATE_LABEL: Record<CsmEngagementTaskState, string> = {
  not_started: "Not started",
  in_progress: "In progress",
  blocked: "Blocked",
  completed: "Completed",
  cancelled: "Cancelled",
};

export const ENGAGEMENT_DELIVERABLE_STATUS_LABEL: Record<CsmEngagementDeliverableStatus, string> = {
  pending: "Pending",
  in_review: "In review",
  accepted: "Accepted",
  waived: "Waived",
};

/** MUI Chip color mappings — keep in sync with oxygen-ui palette tokens. */
export const ENGAGEMENT_STATE_COLOR: Record<
  CsmEngagementState,
  "default" | "primary" | "warning" | "success" | "error" | "info"
> = {
  new: "default",
  requested: "info",
  in_progress: "primary",
  on_hold: "warning",
  completed: "success",
  cancelled: "error",
};

export const ENGAGEMENT_HEALTH_COLOR: Record<
  CsmEngagementHealth,
  "success" | "warning" | "error"
> = {
  green: "success",
  amber: "warning",
  red: "error",
};

export const ENGAGEMENT_TASK_STATE_COLOR: Record<
  CsmEngagementTaskState,
  "default" | "primary" | "warning" | "success" | "error"
> = {
  not_started: "default",
  in_progress: "primary",
  blocked: "error",
  completed: "success",
  cancelled: "default",
};

export const ENGAGEMENT_DELIVERABLE_STATUS_COLOR: Record<
  CsmEngagementDeliverableStatus,
  "default" | "primary" | "warning" | "success" | "info"
> = {
  pending: "default",
  in_review: "info",
  accepted: "success",
  waived: "warning",
};

/** Stage order for timelines and progress derivation. */
export const ENGAGEMENT_STAGE_ORDER: CsmEngagementStage[] = [
  "pre_engagement",
  "planning",
  "execution",
  "warranty",
  "post_engagement",
  "closure",
];

export const ENGAGEMENT_TYPES_ALL: CsmEngagementType[] = [
  "customer_onboarding",
  "migration",
  "qsp",
  "enterprise_csm_tam",
  "consultancy",
  "training",
  "architecture_review",
  "firefighting",
];

export const ENGAGEMENT_STATES_ALL: CsmEngagementState[] = [
  "new",
  "requested",
  "in_progress",
  "on_hold",
  "completed",
  "cancelled",
];

/** Returns the stage index (0–5) for sorting / progress calculations. */
export function stageIndex(stage: CsmEngagementStage): number {
  return ENGAGEMENT_STAGE_ORDER.indexOf(stage);
}

/** Whether the engagement is considered active (counted in active KPIs). */
export function isActiveState(state: CsmEngagementState): boolean {
  return state === "in_progress" || state === "on_hold";
}

/** Whether the engagement is closed/terminal (excluded from active KPIs). */
export function isClosedState(state: CsmEngagementState): boolean {
  return state === "completed" || state === "cancelled";
}

/** ISO date string formatter — returns "" for missing input. */
export function formatDateOnly(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toISOString().slice(0, 10);
}

/**
 * Derive overall % complete from stage status + task completion.
 * Stage weight is 60%, tasks contribute 40%.
 */
export function deriveProgressPct(input: {
  stageStatus: Partial<Record<CsmEngagementStage, "not_started" | "in_progress" | "completed">>;
  tasksTotal: number;
  tasksCompleted: number;
}): number {
  const stageScore = ENGAGEMENT_STAGE_ORDER.reduce((acc, s) => {
    const v = input.stageStatus[s];
    if (v === "completed") return acc + 1;
    if (v === "in_progress") return acc + 0.5;
    return acc;
  }, 0);
  const stagePct = (stageScore / ENGAGEMENT_STAGE_ORDER.length) * 60;
  const taskPct =
    input.tasksTotal === 0 ? 40 : (input.tasksCompleted / input.tasksTotal) * 40;
  return Math.round(stagePct + taskPct);
}
