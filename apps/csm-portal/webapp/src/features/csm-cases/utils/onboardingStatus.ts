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

/**
 * The full `projectOnboardingStatus` vocabulary — entity-service's real
 * 7-value `onboarding_status_enum` (`onboardingStatusLabels` in that repo's
 * `case_repo.go`), not just the 4 a project is usually in. Shared between
 * the "Onboarding status" bar control (`CasesFilterBar.tsx`, which offers
 * all of these as options) and `translateCaseDashboardFilters`
 * (`widgetResourceConfig.ts`, which needs the full set to turn a dashboard
 * widget's `notIn` filter into this field's complement — see that file's
 * doc comment for why).
 *
 * This list used to stop at 4 ("the fixed ServiceNow choice list"),
 * omitting `On-Hold`/`Expired`/`Cancelled` — found live: a dashboard
 * widget's `excludeInProgressOnboardingStatus` preset
 * (`{"field":"projectOnboardingStatus","op":"notIn","values":["In-Progress"]}`,
 * a true exclusion of only "In-Progress") drilled through to the Advanced
 * Filter Bar as `in: ["Not-Started","Completed","Not-Applicable"]` — this
 * list's own complement of `["In-Progress"]` — silently re-excluding every
 * case whose project was `On-Hold`/`Expired`/`Cancelled` even though the
 * real preset never meant to touch them. A case on an Expired-onboarding
 * project reportedly disappeared from an "In Progress Cases" dashboard
 * widget for exactly this reason.
 */
export const ALL_ONBOARDING_STATUSES = [
  "In-Progress",
  "Not-Started",
  "Completed",
  "Not-Applicable",
  "On-Hold",
  "Expired",
  "Cancelled",
] as const;

export const ONBOARDING_STATUS_LABEL: Record<string, string> = {
  "In-Progress": "In progress",
  "Not-Started": "Not started",
  Completed: "Completed",
  "Not-Applicable": "Not applicable",
  "On-Hold": "On hold",
  Expired: "Expired",
  Cancelled: "Cancelled",
};

/**
 * A value no real case's `projectOnboardingStatus` ever has. Used by
 * `translateCaseDashboardFilters` (`widgetResourceConfig.ts`) when a
 * dashboard widget's `notIn` filter excludes every one of the known
 * values (or an `in`/`notIn` pair is disjoint) — the resulting complement is
 * genuinely empty, meaning the widget's own filter can never match any
 * case. `CasesFilters.onboardingStatuses` has no way to distinguish that
 * from "unfiltered" (both are `[]` — the same convention every other array
 * filter in this app uses, see `caseSearchPayload.ts`), and widening the
 * type to carry that distinction everywhere (URL codec, search payload, the
 * bar control) is a much larger change than this edge case warrants. Using
 * this sentinel as the sole `in` value keeps the field's shape unchanged
 * while still resolving to zero matching cases — the correct result for a
 * filter that excludes everything — instead of silently falling back to
 * "unfiltered" and showing every case, which is exactly the sign-flip bug
 * this whole field's design exists to prevent.
 */
export const ONBOARDING_STATUS_NO_MATCH = "__no_onboarding_status_matches__";
