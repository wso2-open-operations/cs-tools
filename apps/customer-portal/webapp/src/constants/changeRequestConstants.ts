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

import {
  Activity,
  Bell,
  CalendarCheck,
  CheckCircle,
  CircleCheck,
  CircleQuestionMark,
  Eye,
  FileText,
  PlayCircle,
  RotateCcw,
  ShieldCheck,
  UserCheck,
  XCircle,
} from "@wso2/oxygen-ui-icons-react";
import { alpha, colors } from "@wso2/oxygen-ui";
import { type ComponentType } from "react";
import type { CaseMetadataResponse } from "@/types/cases";
import type { ChangeRequestFilterValues } from "@/types/changeRequests";
import type { SupportStatConfig } from "@constants/supportConstants";

/**
 * Marketing bullet points for the Change Request card on Support / Operations.
 */
export const CHANGE_REQUEST_BULLET_ITEMS = [
  "Formal approval process",
  "Scheduled implementation",
  "Customer review and approval",
  "Rollback capabilities",
  "Calendar visualization",
  "Complete audit trail",
  "Impact and risk assessment",
  "Post implementation verification",
] as const;

/**
 * Change Request Status types.
 */
export const ChangeRequestStatus = {
  SCHEDULED: "Scheduled",
  IN_PROGRESS: "In Progress",
  COMPLETED: "Completed",
  CANCELLED: "Cancelled",
  PENDING_APPROVAL: "Pending Approval",
} as const;

export type ChangeRequestStatus =
  (typeof ChangeRequestStatus)[keyof typeof ChangeRequestStatus];

/**
 * Change Request Impact types.
 */
export const ChangeRequestImpact = {
  HIGH: "High",
  MEDIUM: "Medium",
  LOW: "Low",
} as const;

export type ChangeRequestImpact =
  (typeof ChangeRequestImpact)[keyof typeof ChangeRequestImpact];

/**
 * Change Request State labels.
 */
export const ChangeRequestStates = {
  NEW: "New",
  ASSESS: "Assess",
  AUTHORIZE: "Authorize",
  CUSTOMER_APPROVAL: "Customer Approval",
  SCHEDULED: "Scheduled",
  IMPLEMENT: "Implement",
  REVIEW: "Review",
  CUSTOMER_REVIEW: "Customer Review",
  ROLLBACK: "Rollback",
  CLOSED: "Closed",
  CANCELED: "Canceled",
} as const;

export type ChangeRequestState =
  (typeof ChangeRequestStates)[keyof typeof ChangeRequestStates];

/**
 * Canonical workflow order (used by progress UI and PDF).
 */
export const CHANGE_REQUEST_STATE_ORDER: ChangeRequestState[] = [
  ChangeRequestStates.NEW,
  ChangeRequestStates.ASSESS,
  ChangeRequestStates.AUTHORIZE,
  ChangeRequestStates.CUSTOMER_APPROVAL,
  ChangeRequestStates.SCHEDULED,
  ChangeRequestStates.IMPLEMENT,
  ChangeRequestStates.REVIEW,
  ChangeRequestStates.CUSTOMER_REVIEW,
  ChangeRequestStates.ROLLBACK,
  ChangeRequestStates.CLOSED,
  ChangeRequestStates.CANCELED,
];

/**
 * Maps stable API state ids to canonical change request state labels.
 */
export const CHANGE_REQUEST_STATE_API_ID_TO_LABEL: Record<
  string,
  ChangeRequestState
> = {
  "-5": ChangeRequestStates.NEW,
  "-4": ChangeRequestStates.ASSESS,
  "-3": ChangeRequestStates.AUTHORIZE,
  "5": ChangeRequestStates.CUSTOMER_APPROVAL,
  "-2": ChangeRequestStates.SCHEDULED,
  "-1": ChangeRequestStates.IMPLEMENT,
  "0": ChangeRequestStates.REVIEW,
  "1": ChangeRequestStates.CUSTOMER_REVIEW,
  "2": ChangeRequestStates.ROLLBACK,
  "3": ChangeRequestStates.CLOSED,
  "4": ChangeRequestStates.CANCELED,
};

function normalizeChangeRequestStateLabel(
  label: string | null | undefined,
): ChangeRequestState | undefined {
  if (!label) return undefined;
  const t = label.trim();
  if (t === "Cancelled") {
    return ChangeRequestStates.CANCELED;
  }
  const values = Object.values(ChangeRequestStates) as string[];
  if (values.includes(t)) {
    return t as ChangeRequestState;
  }
  return undefined;
}

/**
 * Resolves API state (id preferred, then label) to a canonical workflow label.
 *
 * @param state - Optional id/label from the API.
 * @returns {ChangeRequestState | undefined} Canonical state or undefined.
 */
export function resolveChangeRequestCanonicalState(
  state?: { id?: string; label?: string } | null,
): ChangeRequestState | undefined {
  if (!state) return undefined;
  const id = state.id?.trim();
  if (id && id in CHANGE_REQUEST_STATE_API_ID_TO_LABEL) {
    return CHANGE_REQUEST_STATE_API_ID_TO_LABEL[id];
  }
  return normalizeChangeRequestStateLabel(state.label ?? undefined);
}

function getStateColorPaletteForCanonical(
  state: ChangeRequestState | undefined,
): (typeof colors)[keyof typeof colors] {
  if (!state) return colors.grey;

  switch (state) {
    case ChangeRequestStates.NEW:
      return colors.blue;
    case ChangeRequestStates.ASSESS:
      return colors.purple;
    case ChangeRequestStates.AUTHORIZE:
      return colors.pink;
    case ChangeRequestStates.CUSTOMER_APPROVAL:
      return colors.amber;
    case ChangeRequestStates.SCHEDULED:
      return colors.cyan;
    case ChangeRequestStates.IMPLEMENT:
      return colors.purple;
    case ChangeRequestStates.REVIEW:
      return colors.indigo;
    case ChangeRequestStates.CUSTOMER_REVIEW:
      return colors.lightGreen;
    case ChangeRequestStates.ROLLBACK:
      return colors.red;
    case ChangeRequestStates.CLOSED:
      return colors.green;
    case ChangeRequestStates.CANCELED:
      return colors.brown;
    default:
      return colors.grey;
  }
}

function getChangeRequestStateColorForCanonical(
  state: ChangeRequestState | undefined,
): string {
  if (!state) return colors.grey[400];

  switch (state) {
    case ChangeRequestStates.NEW:
      return colors.blue[900];
    case ChangeRequestStates.ASSESS:
      return colors.purple[200];
    case ChangeRequestStates.AUTHORIZE:
      return colors.pink[700];
    case ChangeRequestStates.CUSTOMER_APPROVAL:
      return colors.amber[800];
    case ChangeRequestStates.SCHEDULED:
      return colors.cyan[600];
    case ChangeRequestStates.IMPLEMENT:
      return colors.purple[500];
    case ChangeRequestStates.REVIEW:
      return colors.indigo[300];
    case ChangeRequestStates.CUSTOMER_REVIEW:
      return colors.lightGreen[600];
    case ChangeRequestStates.ROLLBACK:
      return colors.red[700];
    case ChangeRequestStates.CLOSED:
      return colors.green[800];
    case ChangeRequestStates.CANCELED:
      return colors.brown[500];
    default:
      return colors.grey[500];
  }
}

/**
 * Accent color for change request state (string label or `{ id, label }` from API).
 */
export function getChangeRequestStateColor(
  stateOrLabel?: string | { id?: string; label?: string } | null,
): string {
  const canonical =
    typeof stateOrLabel === "object" && stateOrLabel !== null
      ? resolveChangeRequestCanonicalState(stateOrLabel)
      : normalizeChangeRequestStateLabel(stateOrLabel);
  return getChangeRequestStateColorForCanonical(canonical);
}

/**
 * Alpha-tinted bg/text/border for change request state chips (list / details).
 */
export function getChangeRequestStateColorShades(
  stateOrLabel?: string | { id?: string; label?: string } | null,
): { bg: string; text: string; border: string } {
  const canonical =
    typeof stateOrLabel === "object" && stateOrLabel !== null
      ? resolveChangeRequestCanonicalState(stateOrLabel)
      : normalizeChangeRequestStateLabel(stateOrLabel);
  const accent = getChangeRequestStateColorForCanonical(canonical);
  const palette = getStateColorPaletteForCanonical(canonical);
  const row = palette as unknown as Record<number, string>;

  return {
    bg: alpha(accent, 0.1),
    text: row[800] ?? row[500],
    border: alpha(accent, 0.25),
  };
}

/**
 * Get icon component for change request state.
 */
export function getChangeRequestStateIcon(
  stateOrLabel?: string | { id?: string; label?: string } | null,
): ComponentType<{ size?: number }> {
  const canonical =
    typeof stateOrLabel === "object" && stateOrLabel !== null
      ? resolveChangeRequestCanonicalState(stateOrLabel)
      : normalizeChangeRequestStateLabel(stateOrLabel);
  if (!canonical) return CircleQuestionMark;

  switch (canonical) {
    case ChangeRequestStates.NEW:
      return FileText;
    case ChangeRequestStates.ASSESS:
      return Activity;
    case ChangeRequestStates.AUTHORIZE:
      return ShieldCheck;
    case ChangeRequestStates.CUSTOMER_APPROVAL:
      return UserCheck;
    case ChangeRequestStates.SCHEDULED:
      return CalendarCheck;
    case ChangeRequestStates.IMPLEMENT:
      return PlayCircle;
    case ChangeRequestStates.REVIEW:
      return Eye;
    case ChangeRequestStates.CUSTOMER_REVIEW:
      return UserCheck;
    case ChangeRequestStates.ROLLBACK:
      return RotateCcw;
    case ChangeRequestStates.CLOSED:
      return CheckCircle;
    case ChangeRequestStates.CANCELED:
      return XCircle;
    default:
      return CircleQuestionMark;
  }
}

/**
 * Change Request Impact labels.
 */
export const ChangeRequestImpactLabels = {
  HIGH: "1 - High",
  MEDIUM: "2 - Medium",
  LOW: "3 - Low",
} as const;

function getImpactColorPalette(impactLabel: string | undefined) {
  if (!impactLabel) return colors.grey;

  switch (impactLabel) {
    case ChangeRequestImpactLabels.HIGH:
      return colors.red;
    case ChangeRequestImpactLabels.MEDIUM:
      return colors.orange;
    case ChangeRequestImpactLabels.LOW:
      return colors.green;
    default:
      return colors.grey;
  }
}

/**
 * Get color for change request impact level.
 */
export function getChangeRequestImpactColor(
  impactLabel: string | undefined,
): string {
  const palette = getImpactColorPalette(impactLabel);
  return !impactLabel ? palette[400] : palette[500];
}

/**
 * Get color shades for change request impact level.
 */
export function getChangeRequestImpactColorShades(
  impactLabel: string | undefined,
): { bg: string; text: string; border: string } {
  const colorShades = getImpactColorPalette(impactLabel);

  return {
    bg: alpha(colorShades[500], 0.1),
    text: colorShades[800],
    border: alpha(colorShades[500], 0.2),
  };
}

/**
 * Format impact label by removing the numeric prefix.
 * "1 - High" -> "High"
 */
export function formatImpactLabel(impactLabel: string | undefined): string {
  if (!impactLabel) return "Not Available";

  return impactLabel.replace(/^\d+\s*-\s*/, "");
}

/**
 * Valid keys for change requests statistics.
 */
export type ChangeRequestStatKey =
  | "totalRequests"
  | "awaitingYourAction"
  | "ongoing"
  | "completed";

/**
 * Configuration for the change requests statistics cards.
 */
export const CHANGE_REQUEST_STAT_CONFIGS: SupportStatConfig<ChangeRequestStatKey>[] =
  [
    {
      icon: FileText,
      iconColor: "info",
      key: "totalRequests",
      label: "Total Requests",
    },
    {
      icon: Bell,
      iconColor: "warning",
      key: "awaitingYourAction",
      label: "Awaiting Your Action",
    },
    {
      icon: Activity,
      iconColor: "primary",
      key: "ongoing",
      label: "Ongoing",
    },
    {
      icon: CircleCheck,
      iconColor: "success",
      key: "completed",
      label: "Completed",
    },
  ];

/**
 * Change request filter definitions.
 */
export const CHANGE_REQUEST_FILTER_DEFINITIONS: Array<{
  filterKey: keyof ChangeRequestFilterValues;
  id: string;
  metadataKey: keyof CaseMetadataResponse;
}> = [
  {
    filterKey: "stateId",
    id: "state",
    metadataKey: "changeRequestStates",
  },
  {
    filterKey: "impactId",
    id: "impact",
    metadataKey: "changeRequestImpacts",
  },
];
