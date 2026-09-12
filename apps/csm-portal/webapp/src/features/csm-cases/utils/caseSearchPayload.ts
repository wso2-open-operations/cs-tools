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

import type { BackendApi } from "@api/backend/client";
import {
  beStateFromUi,
  priorityFromSeverity,
  severityFromBe,
  uiStateFromBe,
} from "@api/backend/mappers";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/utils/assignee";
import { classifyCaseQuery } from "@features/csm-cases/utils/caseQueryScope";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import type {
  BeCaseFieldFilter,
  BeCaseSearchFilters,
  BeCaseSearchView,
  BeUserSearchResponse,
} from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";
import {
  advancedFilterRowToFieldFilter,
  isCompleteAdvancedFilterRow,
} from "@features/csm-cases/utils/advancedFilters";
import { anyOfBranchToPayload } from "@features/csm-cases/utils/anyOfFilters";

/**
 * Builds the `/cases/search` `filters` object for the given UI filter state
 * — shared by `useGetCsmCases` (the listing) and the cases/service-requests
 * "Export CSV" action, so the export is provably searching with the exact
 * same filter payload the listing itself sent, not a hand-rolled
 * approximation that can silently drift from it.
 *
 * `assignedUserIds` must already be resolved (see
 * {@link resolveAssignedUserIds}) — this function does no async work.
 *
 * Emits the generic `field`/`op`/`values` filter array (see
 * `BeCaseFieldFilter`) rather than the old named fields.
 */
export function buildCaseSearchFilters(
  filters: CasesFilters,
  search: string,
  assignedUserIds: string[] | undefined,
  options?: { forceFreeText?: boolean; alsoFreeText?: boolean },
): BeCaseSearchFilters {
  const fieldFilters: BeCaseFieldFilter[] = [];
  if (filters.severities.length > 0) {
    fieldFilters.push({
      field: "severity",
      op: "in",
      values: filters.severities.map(priorityFromSeverity),
    });
  }
  if (filters.states.length > 0) {
    fieldFilters.push({
      field: "state",
      op: "in",
      values: filters.states.map(beStateFromUi),
    });
  }
  // `states`/`excludeStates` both target the `state` field but with
  // different ops (`in`/`notIn`) — two independent entries, same reasoning
  // as `tags`/`excludeTags` below.
  if (filters.excludeStates.length > 0) {
    fieldFilters.push({
      field: "state",
      op: "notIn",
      values: filters.excludeStates.map(beStateFromUi),
    });
  }
  if (filters.caseTypes.length > 0) {
    fieldFilters.push({ field: "type", op: "in", values: filters.caseTypes });
  }
  // Work state can only be applied server-side when "work_in_progress" is
  // the sole selected state — enforced here (not just in the filter bar's
  // own onChange) so a stale workStates value reaching this builder any
  // other way (a saved view, a pinned/dashboard URL, a future caller) can
  // never silently narrow the results to just in-progress/ongoing-paused
  // cases when other states are also selected.
  if (
    filters.workStates.length > 0 &&
    filters.states.length === 1 &&
    filters.states[0] === "work_in_progress"
  ) {
    fieldFilters.push({
      field: "workState",
      op: "in",
      values: filters.workStates,
    });
  }
  if (filters.engagementTypes.length > 0) {
    fieldFilters.push({
      field: "engagementType",
      op: "in",
      values: filters.engagementTypes,
    });
  }
  if (filters.projects.length > 0) {
    fieldFilters.push({ field: "projectId", op: "in", values: filters.projects });
  }
  if (assignedUserIds && assignedUserIds.length > 0) {
    fieldFilters.push({
      field: "assignedUserId",
      op: "in",
      values: assignedUserIds,
    });
  }
  if (filters.productNames.length > 0) {
    fieldFilters.push({
      field: "product",
      op: "in",
      values: filters.productNames,
    });
  }
  if (filters.csTeams.length > 0) {
    fieldFilters.push({
      field: "creTeam",
      op: "in",
      values: filters.csTeams,
    });
  }
  if (filters.sreTeams.length > 0) {
    fieldFilters.push({
      field: "sreTeam",
      op: "in",
      values: filters.sreTeams,
    });
  }
  // `tags`/`excludeTags` both target the `tag` field but with different ops
  // (`in`/`notIn`) — two independent entries, not a single one an op flag
  // toggles, so both may be active at once (the backend ANDs the array).
  if (filters.tags.length > 0) {
    fieldFilters.push({ field: "tag", op: "in", values: filters.tags });
  }
  if (filters.excludeTags.length > 0) {
    fieldFilters.push({ field: "tag", op: "notIn", values: filters.excludeTags });
  }
  // No `notIn` counterpart here (unlike `tags`/`excludeTags`): the domain is
  // the 4 fixed values in `onboardingStatus.ts`, so a dashboard widget's
  // `projectOnboardingStatus notIn` filter is folded into this same `in`
  // list as its complement at the translation boundary
  // (`translateCaseDashboardFilters`), never carried through as a second op.
  if (filters.onboardingStatuses.length > 0) {
    fieldFilters.push({
      field: "projectOnboardingStatus",
      op: "in",
      values: filters.onboardingStatuses,
    });
  }
  if (filters.slaElapsedPctGte !== null) {
    fieldFilters.push({
      field: "taskSLABusinessElapsedPercent",
      op: "gte",
      values: [String(filters.slaElapsedPctGte)],
    });
  }
  if (filters.slaElapsedPctLte !== null) {
    fieldFilters.push({
      field: "taskSLABusinessElapsedPercent",
      op: "lte",
      values: [String(filters.slaElapsedPctLte)],
    });
  }
  // Value-less predicate: `escalation isEmpty`/`isNotEmpty` carry no
  // `values` at all — the op alone is the whole filter (see
  // `case_filters.go`'s `escalation` case). Must still be emitted whenever
  // `hasEscalation` is non-null, exactly the class of entry
  // `writeWidgetPreviewHref` used to silently drop for having nothing in
  // `values` to serialize.
  if (filters.hasEscalation === true) {
    fieldFilters.push({ field: "escalation", op: "isNotEmpty" });
  } else if (filters.hasEscalation === false) {
    fieldFilters.push({ field: "escalation", op: "isEmpty" });
  }
  if (filters.escalationLevels.length > 0) {
    fieldFilters.push({
      field: "escalationLevel",
      op: "in",
      values: filters.escalationLevels,
    });
  }
  if (filters.projectTypes.length > 0) {
    fieldFilters.push({
      field: "projectType",
      op: "in",
      values: filters.projectTypes,
    });
  }
  if (filters.createdOnGte !== null) {
    fieldFilters.push({ field: "createdOn", op: "gte", values: [filters.createdOnGte] });
  }
  if (filters.createdOnLte !== null) {
    fieldFilters.push({ field: "createdOn", op: "lte", values: [filters.createdOnLte] });
  }
  if (filters.updatedOnGte !== null) {
    fieldFilters.push({ field: "updatedOn", op: "gte", values: [filters.updatedOnGte] });
  }
  if (filters.updatedOnLte !== null) {
    fieldFilters.push({ field: "updatedOn", op: "lte", values: [filters.updatedOnLte] });
  }
  if (filters.closedOnGte !== null) {
    fieldFilters.push({ field: "closedOn", op: "gte", values: [filters.closedOnGte] });
  }
  if (filters.closedOnLte !== null) {
    fieldFilters.push({ field: "closedOn", op: "lte", values: [filters.closedOnLte] });
  }

  // Ad-hoc rows from the "Advanced filters" builder (`AdvancedFiltersBuilder`)
  // — each becomes one extra `BeCaseFieldFilter` entry. Only complete rows
  // (see `isCompleteAdvancedFilterRow`) are emitted; an incomplete one (a
  // field/op picked but no value where the op requires one) is silently
  // skipped rather than sent as an empty predicate. `filters` here is the
  // already relative-date-resolved copy the caller (`useGetCsmCases`) passes
  // in — this function does no date resolution of its own, same as the
  // dedicated `createdOnGte`/`createdOnLte` fields above.
  for (const row of filters.advancedFilters) {
    if (!isCompleteAdvancedFilterRow(row)) continue;
    const ff = advancedFilterRowToFieldFilter(row);
    if (ff) fieldFilters.push(ff);
  }

  // A typed case number / WSO2 case id goes through as an exact-match field
  // filter rather than the free-text `searchQuery` scan, mirroring the global
  // quick-nav palette (see `classifyCaseQuery`, shared by both).
  //
  // `searchQuery` is a CONTAINS/OR scan across number, WSO2 id, short
  // description AND description upstream — so searching an exact case number
  // also matched every *other* case that merely mentions that number in its
  // description, and those could outrank (or crowd out) the case actually
  // being looked up. Reported as such: searching one case number surfaced a
  // different case entirely. An indexed exact match also avoids that scan.
  //
  // `forceFreeText` opts back into the `searchQuery` scan even for a query
  // that looks like an identifier — the cases list runs both legs in parallel
  // and merges them (see `useGetCsmCases`), so that a case mentioning the
  // number in its description is still findable, just below the exact hit.
  //
  // `alsoFreeText` keeps the exact filter *and* adds the scan. The backend ANDs
  // `searchQuery` with the `filters` array (the quick-nav palette relies on the
  // same thing to constrain a free-text search to a set of case types), so this
  // resolves "how many exact hits the scan already covers" — which is what lets
  // the merged total be computed exactly instead of guessed. See
  // `useGetCsmCases`.
  const scope =
    search.length > 0 && !options?.forceFreeText ? classifyCaseQuery(search) : "text";
  if (scope !== "text") {
    fieldFilters.push({ field: scope, op: "eq", values: [search] });
  }

  const withFreeText = scope === "text" || !!options?.alsoFreeText;

  // "OR groups" (`filters.anyOf`) — each branch with at least one complete
  // condition becomes one `{filters: [...]}` entry; an empty branch (no
  // complete conditions at all) is dropped rather than emitted, since the
  // backend 400s on `anyOf: [{}]` (see `anyOfBranchToPayload`).
  const anyOf = filters.anyOfBranches
    .map(anyOfBranchToPayload)
    .filter((b): b is { filters: BeCaseFieldFilter[] } => b !== undefined);

  return {
    ...(withFreeText && search.length > 0 && { searchQuery: search }),
    ...(fieldFilters.length > 0 && { filters: fieldFilters }),
    ...(anyOf.length > 0 && { anyOf }),
  };
}

/** Sentinel: the assignee filter is active (one or more engineers/`@me`
 * selected) but resolved to no ids at all — the caller must treat this as a
 * definitionally-empty result, never as "no assignee filter" (which would
 * silently broaden the search back to every case). See the resolution note
 * in `useGetCsmCases` for why. */
export const ASSIGNEE_FILTER_RESOLVED_EMPTY = Symbol("assignee-filter-resolved-empty");

/**
 * Resolves the assignee filter (engineer emails + the `@me` sentinel) to the
 * user UUIDs `/cases/search` expects. Shared between `useGetCsmCases` and the
 * export action so both apply the exact same assignee filter to the exact
 * same set of cases.
 */
export async function resolveAssignedUserIds(
  api: BackendApi,
  assignees: string[],
  currentUserId: string | undefined,
): Promise<string[] | undefined | typeof ASSIGNEE_FILTER_RESOLVED_EMPTY> {
  if (assignees.length === 0) return undefined;

  const wantsMe = assignees.includes(ASSIGNEE_ME_TOKEN);
  const emails = assignees.filter((a) => a !== ASSIGNEE_ME_TOKEN);
  let byEmail: BeUserSearchResponse | null = null;
  if (emails.length > 0) {
    byEmail = await api.post<
      { filters: { emails: string[] }; pagination: { limit: number } },
      BeUserSearchResponse
    >("/users/search", {
      filters: { emails },
      pagination: { limit: BE_MAX_PAGE_LIMIT },
    });
  }
  const ids = new Set<string>();
  (byEmail?.users ?? []).forEach((u) => {
    if (u.id) ids.add(u.id);
  });
  if (wantsMe && currentUserId) ids.add(currentUserId);

  if (ids.size === 0) return ASSIGNEE_FILTER_RESOLVED_EMPTY;
  return [...ids];
}

/**
 * Maps one `/cases/search` result item to the UI's `CsmCaseRow` shape —
 * shared by `useGetCsmCases` (the listing) and the export action's own
 * paging, so an exported row is provably the same transform the table itself
 * applies, not a second, driftable copy of it.
 */
export function mapCaseSearchViewToRow(
  c: BeCaseSearchView,
  currentUserEmail: string | undefined,
): CsmCaseRow {
  const projectId = c.project?.id ?? "";
  const assigneeEmail = c.assignedEngineer?.email;
  const assignee = c.assignedEngineer?.name?.trim() || assigneeEmail || "Unassigned";
  const myEmail = currentUserEmail?.toLowerCase();
  const assigneeIsMe =
    !!assigneeEmail && !!myEmail && assigneeEmail.toLowerCase() === myEmail;
  const createdBy = c.createdBy?.name?.trim() || c.createdBy?.email || "Unknown";
  return {
    id: c.id,
    caseNumber: c.number,
    wso2CaseId: c.internalId,
    subject: c.subject ?? "(no subject)",
    customer: c.account?.name ?? "-",
    accountId: c.account?.id ?? "",
    projectId,
    projectName: c.project?.name ?? "-",
    product: c.deployedProduct?.name ?? c.product?.name ?? "-",
    severity: severityFromBe(c.severity),
    state: uiStateFromBe(c.state),
    caseType: c.type,
    issueType: c.issueType,
    workState: c.workState ?? null,
    assignee,
    assigneeIsMe,
    createdBy,
    slaClockType: "ack",
    minutesToBreach: 0,
    hasSla: false,
    createdAt: c.createdOn ?? "",
    updatedAt: c.updatedOn ?? c.createdOn ?? "",
    escalationLevel: c.escalationLevel ?? null,
  };
}
