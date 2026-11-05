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

import type { BeCaseType } from "@api/backend/types";

// Single source of truth for case-type ordering and labels. Lives in its own
// (backend-client-free) module so both the filter bar and the pure URL
// serializer can share it without dragging the API client into a unit test or
// tripping the react-refresh "components-only export" rule.

/** All case types, in the order they appear in the type dropdown. */
export const ALL_CASE_TYPES: BeCaseType[] = [
  "case",
  "service_request",
  "security_report_analysis",
  "announcement",
  "engagement",
];

/** Human-readable label per case type. */
export const CASE_TYPE_LABEL: Record<BeCaseType, string> = {
  case: "Case",
  service_request: "Service request",
  security_report_analysis: "Security report",
  announcement: "Announcement",
  engagement: "Engagement",
};

type ChipColor = "default" | "info" | "warning" | "success" | "error";

/** Chip color per case type — `case` (the default/majority type) stays
 * neutral, the others get a distinguishing color. */
export const CASE_TYPE_COLOR: Record<BeCaseType, ChipColor> = {
  case: "default",
  service_request: "info",
  security_report_analysis: "warning",
  announcement: "success",
  engagement: "default",
};

/**
 * Whether severity is a meaningful concept for this case type. Plain support
 * cases (`case`) and service requests carry one that someone sets and acts
 * on; every other type still carries a `severity` in the search response
 * (the field isn't type-gated upstream), but it's not something anyone sets
 * for those, so the UI hides it there rather than show a value nobody put
 * any thought into. A service request's severity is internal (S1-S4, never
 * S0, and always present). A missing `caseType` (legacy rows) is treated as
 * `case`.
 */
export function caseTypeHasSeverity(caseType: BeCaseType | undefined): boolean {
  return caseType === undefined || caseType === "case" || caseType === "service_request";
}

/**
 * {@link ALL_CASE_TYPES}, narrowed to the types a caller may actually see.
 * The backend's `POST /cases/search` denies the WHOLE request with a 403
 * when the type filter names `security_report_analysis` and the caller
 * lacks `PermViewSecurityCenter` (`access.go`) — it does not silently drop
 * just that type and return the rest. Every unlocked, multi-type search
 * (the project Work items tab, the quick-nav case search) must build its
 * "every type" request from this instead of {@link ALL_CASE_TYPES}
 * directly, or a caller who can't see security reports gets a 403 on a
 * search that also asks for ordinary cases, service requests, etc. —
 * reported live.
 *
 * `canSeeSecurityReports` must mirror `PermViewSecurityCenter` exactly
 * (`cs_engineer || admin || comment_updater` — see that permission's own
 * doc comment in `access.go`), not just `canUseSecurityCenter`
 * (`cs_engineer || admin`): a `comment_updater`-only caller holds the
 * backend permission too, and passing the narrower flag here would 403 them
 * on exactly the search this function exists to prevent a 403 on.
 */
export function visibleCaseTypes(canSeeSecurityReports: boolean): BeCaseType[] {
  return canSeeSecurityReports
    ? ALL_CASE_TYPES
    : ALL_CASE_TYPES.filter((t) => t !== "security_report_analysis");
}

/** Where a case's own detail page lives, keyed by its type — each
 * non-`case` type has its own dedicated route/detail page (different
 * fields/actions), not just a filtered view of `/cases`. */
export function caseTypeDetailBasePath(caseType: BeCaseType | undefined): string {
  switch (caseType) {
    case "service_request":
      return "/operations/service-requests";
    case "security_report_analysis":
      return "/security-center/security-reports";
    case "engagement":
      return "/engagements";
    case "announcement":
      return "/announcements";
    case "case":
    default:
      return "/cases";
  }
}
