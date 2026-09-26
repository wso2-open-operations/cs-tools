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

import { useSearchParams } from "react-router";
import type { GlobalFilters, OverviewPriority, OverviewProject, SlaState } from "@api/types";

// The `priority` filter's sentinel value for "issue has no priority"
// (i.priority IS NULL on the backend). Matches the backend's noPriorityValue
// (internal/handler/issues_query.go) one-for-one.
export const NO_PRIORITY_VALUE = "__none__";

export const GLOBAL_FILTER_KEYS = ["repo", "priority", "abtTeam"] as const;
export type GlobalFilterKey = (typeof GLOBAL_FILTER_KEYS)[number];

/** Reads the global filters from the URL and returns a setter that also resets pagination. */
export function useGlobalFilters(): GlobalFilters & { setFilter: (key: GlobalFilterKey, value: string) => void } {
  const [params, setParams] = useSearchParams();
  const setFilter = (key: GlobalFilterKey, value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    next.delete("page"); // a filter change invalidates whatever page the previous filter's results were on
    setParams(next, { replace: true });
  };
  return {
    repo: params.get("repo") ?? undefined,
    priority: params.get("priority") ?? undefined,
    abtTeam: params.get("abtTeam") ?? undefined,
    setFilter,
  };
}

export const ISSUE_LIST_FILTER_KEYS = ["repo", "priority", "abtTeam", "status", "slaState"] as const;
export type IssueListFilterKey = (typeof ISSUE_LIST_FILTER_KEYS)[number];

export interface IssueListFilters {
  repo: string[];
  priority: string[];
  abtTeam: string[];
  status: string[];
  slaState: SlaState[];
}

/**
 * Reads /issues's five multi-select filter keys from repeated URL params and
 * returns a setter that replaces all occurrences of one key, resets
 * pagination, and replaces (rather than pushes) history — matching
 * useGlobalFilters's setFilter.
 */
export function useIssueListFilters(): IssueListFilters & { setFilter: (key: IssueListFilterKey, values: string[]) => void } {
  const [params, setParams] = useSearchParams();
  const setFilter = (key: IssueListFilterKey, values: string[]) => {
    const next = new URLSearchParams(params);
    next.delete(key);
    for (const v of values) next.append(key, v);
    next.delete("page");
    setParams(next, { replace: true });
  };
  return {
    repo: params.getAll("repo"),
    priority: params.getAll("priority"),
    abtTeam: params.getAll("abtTeam"),
    status: params.getAll("status"),
    slaState: params.getAll("slaState") as SlaState[],
    setFilter,
  };
}

/** Wraps a single-valued filter into the one-element array /issues's multi-valued filters expect, or undefined when unset. */
export function toFilterList(value: string | undefined): string[] | undefined {
  return value ? [value] : undefined;
}

/** Friendly project name for "owner/name", falling back to the repo's name part. */
export function projectNameFor(projects: OverviewProject[] | undefined, repo: string | null): string {
  return projects?.find((p) => p.repo === repo)?.name ?? repo?.split("/")[1] ?? "—";
}

/** Priority filter options derived from the overview's own priority tiers (value = key, label = "<Label> · <Code>"), so the dropdown always matches sla-config.yaml's configured tiers. */
export function priorityOptionsFrom(priorities: OverviewPriority[] | undefined): { value: string; label: string }[] {
  return (priorities ?? []).map((p) => ({ value: p.key, label: `${p.label} · ${p.code}` }));
}
