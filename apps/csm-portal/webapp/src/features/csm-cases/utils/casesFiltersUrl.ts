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
  CaseState,
  DashboardScope,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import type {
  CasesFilters,
  OwnerFilter,
  SlaFilter,
} from "@features/csm-cases/components/CasesFilterBar";

export const DEFAULT_CASES_FILTERS: CasesFilters = {
  scope: "my_abt",
  search: "",
  severities: [],
  states: [],
  sla: "any",
  owner: "anyone",
};

const VALID_SCOPES: DashboardScope[] = ["my_abt", "all_customers"];
const VALID_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const VALID_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "solution_proposed",
  "awaiting_info",
  "waiting_on_wso2",
  "reopen",
  "closed",
];
const VALID_SLA: SlaFilter[] = ["any", "at_risk", "breached"];
const VALID_OWNERS: OwnerFilter[] = ["anyone", "me", "unassigned"];

function parseCsv<T extends string>(raw: string | null, allowed: T[]): T[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s): s is T => (allowed as string[]).includes(s));
}

function parseEnum<T extends string>(
  raw: string | null,
  allowed: T[],
  fallback: T,
): T {
  return raw && (allowed as string[]).includes(raw) ? (raw as T) : fallback;
}

export function readCasesFiltersFromUrl(
  params: URLSearchParams,
): CasesFilters {
  return {
    scope: parseEnum(params.get("scope"), VALID_SCOPES, "my_abt"),
    search: params.get("q") ?? "",
    severities: parseCsv(params.get("severities"), VALID_SEVERITIES),
    states: parseCsv(params.get("states"), VALID_STATES),
    sla: parseEnum(params.get("sla"), VALID_SLA, "any"),
    owner: parseEnum(params.get("owner"), VALID_OWNERS, "anyone"),
  };
}

/**
 * Build the search-params object representing these filters. Default values
 * are omitted so the URL stays clean.
 */
export function writeCasesFiltersToUrl(f: CasesFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.scope !== "my_abt") out.set("scope", f.scope);
  if (f.search) out.set("q", f.search);
  if (f.severities.length) out.set("severities", f.severities.join(","));
  if (f.states.length) out.set("states", f.states.join(","));
  if (f.sla !== "any") out.set("sla", f.sla);
  if (f.owner !== "anyone") out.set("owner", f.owner);
  return out;
}

/**
 * Convenience: build a `/cases?...` href from a partial filter override.
 * Anything not specified falls back to the defaults.
 */
export function casesHref(overrides: Partial<CasesFilters>): string {
  const full: CasesFilters = { ...DEFAULT_CASES_FILTERS, ...overrides };
  const qs = writeCasesFiltersToUrl(full).toString();
  return qs ? `/cases?${qs}` : "/cases";
}
