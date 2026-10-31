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
import { formatDateOnlyForDisplay } from "@utils/dateTime";

/** Limits the API enforces on a posted update. */
export const MAX_SUBJECT_LENGTH = 512;
export const MAX_CONTENT_LENGTH = 8000;

/** What the post form says about who can see what it posts. */
export const NOT_VISIBLE_NOTE =
  "These updates are currently not visible to the customer and do not trigger any emails internally.";

/** Allocation states the API knows, in the order the filter lists them. */
export const ALLOCATION_STATES = [
  "TENTATIVE",
  "READY_FOR_CLEARANCE",
  "CLEARANCE_IN_PROGRESS",
  "CONFIRMED",
  "CONFIRMED_VISA_PENDING",
  "ACCEPTED_BY_CONSULTANT",
  "REJECTED_BY_CONSULTANT",
  "REJECTED_OTHER",
  "REJECTED_VISA_ISSUES",
  "ALLOCATION_CANCELLED",
] as const;

const STATE_LABELS: Record<string, string> = {
  TENTATIVE: "Tentative",
  READY_FOR_CLEARANCE: "Ready for clearance",
  CLEARANCE_IN_PROGRESS: "Clearance in progress",
  CONFIRMED: "Confirmed",
  CONFIRMED_VISA_PENDING: "Confirmed, visa pending",
  ACCEPTED_BY_CONSULTANT: "Accepted by consultant",
  REJECTED_BY_CONSULTANT: "Rejected by consultant",
  REJECTED_OTHER: "Rejected (other)",
  REJECTED_VISA_ISSUES: "Rejected (visa issues)",
  ALLOCATION_CANCELLED: "Cancelled",
};

/** Title-cases an unknown enum value ("SOME_VALUE" -> "Some value"). */
export function humanizeEnum(value: string | null | undefined): string {
  if (!value) return "";
  const words = value.toLowerCase().replace(/_/g, " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

export function allocationStateLabel(state: string | null | undefined): string {
  if (!state) return "";
  return STATE_LABELS[state] ?? humanizeEnum(state);
}

export const ALLOCATION_STATE_OPTIONS = ALLOCATION_STATES.map((value) => ({
  value,
  label: allocationStateLabel(value),
}));

export function allocationStateRole(state: string | null | undefined): SemanticRole {
  switch (state) {
    case "CONFIRMED":
      return "success";
    case "CONFIRMED_VISA_PENDING":
    case "ACCEPTED_BY_CONSULTANT":
      return "info";
    case "TENTATIVE":
    case "READY_FOR_CLEARANCE":
    case "CLEARANCE_IN_PROGRESS":
      return "warning";
    case "REJECTED_BY_CONSULTANT":
    case "REJECTED_OTHER":
    case "REJECTED_VISA_ISSUES":
    case "ALLOCATION_CANCELLED":
      return "error";
    default:
      return "default";
  }
}

export function updateStateLabel(state: string | null | undefined): string {
  return humanizeEnum(state);
}

export function updateStateRole(state: string | null | undefined): SemanticRole {
  switch (state) {
    case "PUBLISHED":
      return "success";
    case "CANCELLED":
      return "error";
    default:
      return "default";
  }
}

/** "Aug 1, 2026 - Aug 5, 2026", with either end missing shown as a dash. */
export function formatDateSpan(start?: string, end?: string): string {
  const s = formatDateOnlyForDisplay(start);
  const e = formatDateOnlyForDisplay(end);
  if (!s && !e) return "—";
  return `${s ?? "—"} - ${e ?? "—"}`;
}

export function formatUtilization(value?: number | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) return "—";
  return `${Number.isInteger(value) ? value : value.toFixed(1)}%`;
}

const HTML_TAG = /<\/?[a-z][^>]*>/i;

/** True when the text carries markup, so it must go through the sanitizer. */
export function looksLikeHtml(text: string): boolean {
  return HTML_TAG.test(text);
}
