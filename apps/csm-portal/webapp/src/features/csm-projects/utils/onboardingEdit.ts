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

import { utcDateOnlyValue } from "@utils/dateTime";
import {
  PROJECT_ONBOARDING_STATUSES,
  type ProjectOnboardingPatch,
  type ProjectOnboardingStatusValue,
} from "@features/csm-projects/api/usePatchProjectOnboarding";

export const ONBOARDING_STATUS_LABELS: Record<ProjectOnboardingStatusValue, string> = {
  "Not-Started": "Not Started",
  "In-Progress": "In Progress",
  "On-Hold": "On Hold",
  Completed: "Completed",
  Cancelled: "Cancelled",
  Expired: "Expired",
  "Not-Applicable": "Not Applicable",
};

/** The backend matches ignoring case, hyphens, underscores and spaces, and an
 *  older source spells On Hold `OnHold`; normalize the same way so the picker
 *  opens on the project's real current value. `""` when absent or unknown. */
export function matchCurrentStatus(current?: string | null): ProjectOnboardingStatusValue | "" {
  const norm = (v: string): string => v.toLowerCase().replace(/[-_\s]/g, "");
  const target = current ? norm(current) : "";
  return PROJECT_ONBOARDING_STATUSES.find((s) => norm(s) === target) ?? "";
}

/** Current onboarding dates as the read API returns them (midnight-UTC
 *  timestamps, or null). Read names differ from the PATCH names for plan,
 *  go-live and readiness; `DATE_FIELDS` maps between them. */
export interface ProjectOnboardingDates {
  onboardingStartDate?: string | null;
  goLivePlanDate?: string | null;
  goLiveDate?: string | null;
  onboardingExpiryDate?: string | null;
  productionReadinessCompletionDate?: string | null;
}

export type OnboardingDateField = "start" | "plan" | "goLive" | "expiry" | "readiness";

export const DATE_FIELDS: ReadonlyArray<{
  key: OnboardingDateField;
  label: string;
  read: keyof ProjectOnboardingDates;
  write: keyof ProjectOnboardingPatch;
}> = [
  { key: "start", label: "Onboarding start date", read: "onboardingStartDate", write: "onboardingStartDate" },
  { key: "plan", label: "Go-live plan date", read: "goLivePlanDate", write: "onboardingGoLivePlanDate" },
  { key: "goLive", label: "Go-live date", read: "goLiveDate", write: "onboardingGoLiveDate" },
  { key: "expiry", label: "Onboarding expiry date", read: "onboardingExpiryDate", write: "onboardingExpiryDate" },
  {
    key: "readiness",
    label: "Production readiness completion date",
    read: "productionReadinessCompletionDate",
    write: "onboardingProductionReadinessCompletionDate",
  },
];

export interface OnboardingFormValues {
  status: ProjectOnboardingStatusValue | "";
  ownerId: string;
  dates: Record<OnboardingDateField, string>;
}

/** The form values for a project as stored: `YYYY-MM-DD` from the UTC day of
 *  each date, `""` when unset. Captured when an edit session starts and used
 *  as the baseline for "changed". */
export function initialFormValues(
  status: string | null | undefined,
  ownerId: string | null | undefined,
  dates: ProjectOnboardingDates,
): OnboardingFormValues {
  const out = {} as Record<OnboardingDateField, string>;
  for (const f of DATE_FIELDS) out[f.key] = utcDateOnlyValue(dates[f.read]) ?? "";
  return { status: matchCurrentStatus(status), ownerId: ownerId ?? "", dates: out };
}

/** A date counts as changed only when non-empty and different from the stored
 *  value; an emptied picker means "keep", since a date can't be cleared. */
export function isDateChanged(
  k: OnboardingDateField,
  values: OnboardingFormValues,
  baseline: OnboardingFormValues,
): boolean {
  return values.dates[k] !== "" && values.dates[k] !== baseline.dates[k];
}

/** Only the changed fields, under the request field names; never a null/empty
 *  value (the backend has no way to clear any of them). */
export function buildOnboardingPatch(
  values: OnboardingFormValues,
  baseline: OnboardingFormValues,
): ProjectOnboardingPatch {
  const next: ProjectOnboardingPatch = {};
  if (values.status && values.status !== baseline.status) next.onboardingStatus = values.status;
  if (values.ownerId && values.ownerId !== baseline.ownerId) next.onboardingOwner = values.ownerId;
  for (const f of DATE_FIELDS) {
    if (isDateChanged(f.key, values, baseline)) {
      (next as Record<string, string>)[f.write] = values.dates[f.key];
    }
  }
  return next;
}

/**
 * Client-side date-order check; the backend stays authoritative. The start
 * date the backend will compare against is the one being sent, else the stored
 * one. A later date is checked when it, or the start date, is being changed,
 * against its effective value (the edit, else the stored one), so moving the
 * start past an untouched stored date is caught here rather than rejected by
 * the backend. Unrelated edits are never blocked by stored data that already
 * violates the order. ISO dates compare correctly as strings.
 */
export function validateOnboardingDates(
  values: OnboardingFormValues,
  baseline: OnboardingFormValues,
): Partial<Record<OnboardingDateField, string>> {
  const errors: Partial<Record<OnboardingDateField, string>> = {};
  const effectiveStart = values.dates.start || baseline.dates.start;
  if (!effectiveStart) return errors;
  const startChanged = isDateChanged("start", values, baseline);
  for (const k of ["plan", "goLive", "expiry", "readiness"] as const) {
    const changed = isDateChanged(k, values, baseline);
    if (!changed && !startChanged) continue;
    const effective = changed ? values.dates[k] : baseline.dates[k];
    if (effective && effective < effectiveStart) {
      errors[k] = changed
        ? "Can't be earlier than the onboarding start date."
        : `The stored date (${effective}) is earlier than the new onboarding start date.`;
    }
  }
  return errors;
}
