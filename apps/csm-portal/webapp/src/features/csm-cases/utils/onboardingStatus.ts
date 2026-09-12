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
 * The fixed ServiceNow choice list for `projectOnboardingStatus`. Shared
 * between the "Onboarding status" bar control (`CasesFilterBar.tsx`, which
 * offers exactly these 4 as options) and `translateCaseDashboardFilters`
 * (`widgetResourceConfig.ts`, which needs the full set to turn a dashboard
 * widget's `notIn` filter into this field's complement — see that file's
 * doc comment for why).
 */
export const ALL_ONBOARDING_STATUSES = [
  "In-Progress",
  "Not-Started",
  "Completed",
  "Not-Applicable",
] as const;

export const ONBOARDING_STATUS_LABEL: Record<string, string> = {
  "In-Progress": "In progress",
  "Not-Started": "Not started",
  Completed: "Completed",
  "Not-Applicable": "Not applicable",
};

/**
 * A value no real case's `projectOnboardingStatus` ever has. Used by
 * `translateCaseDashboardFilters` (`widgetResourceConfig.ts`) when a
 * dashboard widget's `notIn` filter excludes every one of the 4 known
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
