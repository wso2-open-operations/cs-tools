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

import type { SearchProjectsRequest } from "@features/csm-projects/types/csmProjects";

/** The onboarding status labels `PATCH /projects/{id}` accepts; the search
 *  endpoint matches the same set ignoring case and `-`/`_`/space separators. */
export const PROJECT_ONBOARDING_STATUSES = [
  "Not-Started",
  "In-Progress",
  "On-Hold",
  "Completed",
  "Cancelled",
  "Expired",
  "Not-Applicable",
] as const;

/** The overall closure states `POST /projects/search` filters on. */
export const PROJECT_CLOSURE_STATES = ["Open", "Suspended", "Restricted"] as const;

/** The date columns the list filters by, each as an inclusive `From`/`To`
 *  pair of `yyyy-MM-dd` request keys (`<key>From`, `<key>To`). */
export const PROJECT_DATE_FILTER_KEYS = [
  "endDate",
  "onboardingStartDate",
  "goLivePlanDate",
  "goLiveDate",
  "onboardingExpiryDate",
  "productionReadinessCompletionDate",
] as const;

export type ProjectDateFilterKey = (typeof PROJECT_DATE_FILTER_KEYS)[number];

export const PROJECT_DATE_FILTER_LABELS: Record<ProjectDateFilterKey, string> = {
  endDate: "Project end date",
  onboardingStartDate: "Onboarding start date",
  goLivePlanDate: "Go-live plan date",
  goLiveDate: "Go-live date",
  onboardingExpiryDate: "Onboarding expiry date",
  productionReadinessCompletionDate: "Production readiness completion date",
};

type DateRangeFilters = {
  [K in ProjectDateFilterKey as `${K}From` | `${K}To`]: string;
};

/**
 * The projects list's filter state. It lives in the URL, under the same names
 * the search request and a dashboard widget's query use, so a widget
 * click-through, a shared link and the request all read one vocabulary:
 * `?onboardingStatus=In-Progress,Completed&closureStates=Open&goLiveDateFrom=2026-01-01`.
 * Lists are comma-separated; a date bound is `yyyy-MM-dd`; `""` means unset.
 */
export interface ProjectFilters extends DateRangeFilters {
  onboardingStatus: string[];
  /** Include list of overall closure states. */
  closureStates: string[];
  /** A single closure state; only arrives from a widget click-through. */
  closureStatus: string;
  /** Exclude list of overall closure states; only arrives from a widget click-through. */
  excludeClosureStates: string[];
}

export const EMPTY_PROJECT_FILTERS: ProjectFilters = {
  onboardingStatus: [],
  closureStates: [],
  closureStatus: "",
  excludeClosureStates: [],
  endDateFrom: "",
  endDateTo: "",
  onboardingStartDateFrom: "",
  onboardingStartDateTo: "",
  goLivePlanDateFrom: "",
  goLivePlanDateTo: "",
  goLiveDateFrom: "",
  goLiveDateTo: "",
  onboardingExpiryDateFrom: "",
  onboardingExpiryDateTo: "",
  productionReadinessCompletionDateFrom: "",
  productionReadinessCompletionDateTo: "",
};

/** The date bound keys of {@link ProjectFilters}, e.g. `goLiveDateFrom`. */
const DATE_BOUND_KEYS = PROJECT_DATE_FILTER_KEYS.flatMap(
  (k) => [`${k}From`, `${k}To`] as const,
);

/** Every URL param this filter state owns. */
export const PROJECT_FILTER_PARAM_KEYS: readonly string[] = [
  "onboardingStatus",
  "closureStates",
  "closureStatus",
  "excludeClosureStates",
  ...DATE_BOUND_KEYS,
];

const DATE_ONLY_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

function parseCsv(raw: string | null): string[] {
  return raw
    ? raw
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean)
    : [];
}

/** Only the known closure states: a stale or hand-edited value is dropped
 *  rather than sent to be rejected. */
function parseClosureStates(raw: string | null): string[] {
  return parseCsv(raw).filter((s) => (PROJECT_CLOSURE_STATES as readonly string[]).includes(s));
}

/** Case, hyphen, underscore and space are ignored by the search endpoint's
 *  onboarding status match (`OnHold` and `On-Hold` are both accepted). */
const normalizeStatus = (s: string): string => s.toLowerCase().replace(/[-_\s]/g, "");
const KNOWN_ONBOARDING_STATUSES = new Set<string>(PROJECT_ONBOARDING_STATUSES.map(normalizeStatus));

/** Only onboarding statuses the search accepts: an unknown or hand-edited value
 *  is dropped (the endpoint answers 400 for it). Valid values keep their
 *  spelling so they still match the filter's option values. */
function parseOnboardingStatuses(raw: string | null): string[] {
  return parseCsv(raw).filter((s) => KNOWN_ONBOARDING_STATUSES.has(normalizeStatus(s)));
}

/** A real `yyyy-MM-dd` calendar date, else `""`. */
function parseDateOnly(raw: string | null): string {
  const m = raw ? DATE_ONLY_RE.exec(raw) : null;
  if (!m) return "";
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  const real =
    d.getFullYear() === Number(m[1]) && d.getMonth() === Number(m[2]) - 1 && d.getDate() === Number(m[3]);
  return real ? raw! : "";
}

/** Read the filter state from the URL. Unknown or malformed values are
 *  dropped; an end date before its start is dropped too (it could only match
 *  nothing, and the backend answers 400). */
export function readProjectFilters(params: URLSearchParams): ProjectFilters {
  const out: ProjectFilters = {
    ...EMPTY_PROJECT_FILTERS,
    onboardingStatus: parseOnboardingStatuses(params.get("onboardingStatus")),
    closureStates: parseClosureStates(params.get("closureStates")),
    excludeClosureStates: parseClosureStates(params.get("excludeClosureStates")),
  };
  const single = parseClosureStates(params.get("closureStatus"));
  out.closureStatus = single.length === 1 ? single[0] : "";
  for (const k of PROJECT_DATE_FILTER_KEYS) {
    const from = parseDateOnly(params.get(`${k}From`));
    const to = parseDateOnly(params.get(`${k}To`));
    out[`${k}From`] = from;
    out[`${k}To`] = from && to && from > to ? "" : to;
  }
  return out;
}

/** Write the filter state onto `params` (a copy is returned): every owned key
 *  is removed first, then each non-empty one set, so unrelated params survive. */
export function writeProjectFilters(
  params: URLSearchParams,
  filters: ProjectFilters,
): URLSearchParams {
  const next = new URLSearchParams(params);
  for (const k of PROJECT_FILTER_PARAM_KEYS) next.delete(k);
  if (filters.onboardingStatus.length > 0) next.set("onboardingStatus", filters.onboardingStatus.join(","));
  if (filters.closureStates.length > 0) next.set("closureStates", filters.closureStates.join(","));
  if (filters.closureStatus) next.set("closureStatus", filters.closureStatus);
  if (filters.excludeClosureStates.length > 0) {
    next.set("excludeClosureStates", filters.excludeClosureStates.join(","));
  }
  for (const k of DATE_BOUND_KEYS) {
    if (filters[k]) next.set(k, filters[k]);
  }
  return next;
}

/** The request fields for these filters; unset ones are omitted. */
export function projectFiltersToRequest(
  filters: ProjectFilters,
): Partial<SearchProjectsRequest> {
  const out: Partial<SearchProjectsRequest> = {};
  if (filters.onboardingStatus.length > 0) out.onboardingStatus = filters.onboardingStatus;
  if (filters.closureStates.length > 0) out.closureStates = filters.closureStates;
  if (filters.closureStatus) out.closureStatus = filters.closureStatus;
  if (filters.excludeClosureStates.length > 0) out.excludeClosureStates = filters.excludeClosureStates;
  for (const k of DATE_BOUND_KEYS) {
    if (filters[k]) out[k] = filters[k];
  }
  return out;
}

/** How many filters are active: each list counts once, and so does each date
 *  column with either bound set. */
export function countActiveProjectFilters(filters: ProjectFilters): number {
  let n = 0;
  if (filters.onboardingStatus.length > 0) n++;
  if (filters.closureStates.length > 0) n++;
  if (filters.closureStatus) n++;
  if (filters.excludeClosureStates.length > 0) n++;
  for (const k of PROJECT_DATE_FILTER_KEYS) {
    if (filters[`${k}From`] || filters[`${k}To`]) n++;
  }
  return n;
}

export interface ProjectFilterChip {
  key: string;
  label: string;
  onRemove: (f: ProjectFilters) => ProjectFilters;
}

function dateRangeLabel(label: string, from: string, to: string): string {
  if (from && to) return `${label}: ${from} to ${to}`;
  return from ? `${label}: from ${from}` : `${label}: until ${to}`;
}

/** One removable chip per active filter. `withBuilderRows` false
 *  limits it to the two filters the builder has no row for, since with the
 *  builder open a chip would duplicate its row. */
export function buildProjectFilterChips(filters: ProjectFilters, withBuilderRows: boolean): ProjectFilterChip[] {
  const chips: ProjectFilterChip[] = [];
  if (filters.closureStatus) {
    chips.push({
      key: "closureStatus",
      label: `Project state: ${filters.closureStatus}`,
      onRemove: (f) => ({ ...f, closureStatus: "" }),
    });
  }
  if (filters.excludeClosureStates.length > 0) {
    chips.push({
      key: "excludeClosureStates",
      label: `Excluding state: ${filters.excludeClosureStates.join(", ")}`,
      onRemove: (f) => ({ ...f, excludeClosureStates: [] }),
    });
  }
  if (!withBuilderRows) return chips;
  if (filters.onboardingStatus.length > 0) {
    chips.push({
      key: "onboardingStatus",
      label: `Onboarding status: ${filters.onboardingStatus.join(", ")}`,
      onRemove: (f) => ({ ...f, onboardingStatus: [] }),
    });
  }
  if (filters.closureStates.length > 0) {
    chips.push({
      key: "closureStates",
      label: `Project state: ${filters.closureStates.join(", ")}`,
      onRemove: (f) => ({ ...f, closureStates: [] }),
    });
  }
  for (const k of PROJECT_DATE_FILTER_KEYS) {
    const from = filters[`${k}From`];
    const to = filters[`${k}To`];
    if (!from && !to) continue;
    chips.push({
      key: k,
      label: dateRangeLabel(PROJECT_DATE_FILTER_LABELS[k], from, to),
      onRemove: (f) => ({ ...f, [`${k}From`]: "", [`${k}To`]: "" }),
    });
  }
  return chips;
}
