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
  keepPreviousData,
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import { severityFromPriority, uiStateFromBe } from "@api/backend/mappers";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
  BeCaseState,
} from "@api/backend/types";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

/**
 * Every lifecycle state except the terminal `closed` — the widget tracks the
 * caller's active workload. `reopened` is included (it is a non-closed,
 * assignable state); the severity/state matrix omits it, but "all non-closed"
 * should not. `/cases/search` takes an inclusion list, so "non-closed" is
 * expressed by enumerating the non-closed states rather than excluding one.
 */
const NON_CLOSED_STATES: BeCaseState[] = [
  "open",
  "work_in_progress",
  "waiting_on_wso2",
  "awaiting_info",
  "reopened",
  "solution_proposed",
];

/** UI states for the "View all" deep-link (`/cases`), which filters on the UI
 * `CaseState` vocabulary — `reopened` has no filter option there, so it is
 * dropped from the link only (the widget itself still lists reopened cases). */
export const MY_OPEN_CASES_LINK_STATES = NON_CLOSED_STATES.filter(
  (s) => s !== "reopened",
).join(",");

export interface MyAssignedOpenCases {
  cases: CsmCaseRow[];
  /** Total non-closed cases assigned to the caller across all pages. */
  total: number;
  /** Whether more rows exist beyond the current page. */
  hasMore: boolean;
}

/** Default page size for the dashboard widget (kept small to stay compact). */
export const MY_OPEN_CASES_PAGE_SIZE = 5;

/**
 * Non-closed cases assigned to the signed-in engineer, for the dashboard
 * "Assigned to me" widget.
 *
 * A single `POST /cases/search` filtered server-side by `assignedUserIds` (the
 * caller's platform UUID from the app-wide current-user context) and the
 * non-closed `states` set — no client-side filtering of a superset. Sorted by
 * `updatedOn` desc (most recently touched first), matching the main cases list.
 * The widget paginates a small page (`page` / `pageSize`) rather than loading
 * everything; `total` / `hasMore` drive the pager and the "View all" link.
 *
 * The account/customer column is not resolved here (the case-search view does
 * not embed it and `CasesList` does not render it), so this deliberately skips
 * the account-directory lookup `useGetCsmCases` runs — keeping the dashboard
 * load to one request.
 *
 * Keyed under the `CSM_CASES` prefix so case create / assignment / close
 * mutations (which invalidate that prefix) refresh the widget. Disabled until
 * the caller's `id` is known — `/users/me` omits it only when the entity
 * service is down, in which case the widget shows an unavailable state rather
 * than silently broadening to everyone's cases.
 */
export function useGetMyAssignedOpenCases(
  page: number,
  pageSize: number,
): UseQueryResult<MyAssignedOpenCases, Error> {
  const api = useBackendApi();
  const userId = useCurrentUser().user?.id;
  const myEmail = useIdTokenClaims()?.email;
  const offset = page * pageSize;

  return useQuery<MyAssignedOpenCases, Error>({
    queryKey: [
      ApiQueryKeys.CSM_CASES,
      "my-assigned-open",
      userId ?? "",
      page,
      pageSize,
    ],
    enabled: !!userId,
    queryFn: async (): Promise<MyAssignedOpenCases> => {
      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          pagination: { offset, limit: pageSize },
          sortBy: { field: "updatedOn", order: "desc" },
          filters: {
            assignedUserIds: [userId as string],
            states: NON_CLOSED_STATES,
          },
        },
      );

      const myEmailLc = myEmail?.toLowerCase();
      const cases: CsmCaseRow[] = (res.cases ?? []).map((c) => {
        const assigneeEmail = c.assignedEngineer?.email;
        return {
          id: c.id,
          caseNumber: c.number,
          wso2CaseId: c.internalId,
          subject: c.subject ?? "(no subject)",
          // The search view does not embed the account; CasesList never renders
          // it, so leave it unresolved rather than scanning the account list.
          customer: "-",
          accountId: "",
          projectId: c.project?.id ?? "",
          projectName: c.project?.name ?? "-",
          product: c.deployedProduct?.name ?? "-",
          severity: severityFromPriority(c.severity),
          state: uiStateFromBe(c.state),
          caseType: c.type,
          workState: c.workState ?? null,
          assignee:
            c.assignedEngineer?.name?.trim() || assigneeEmail || "Unassigned",
          assigneeIsMe:
            !!assigneeEmail &&
            !!myEmailLc &&
            assigneeEmail.toLowerCase() === myEmailLc,
          slaClockType: "ack",
          minutesToBreach: 0,
          // No SLA data from the backend yet — keep the column neutral.
          hasSla: false,
          createdAt: c.createdOn ?? "",
          updatedAt: c.updatedOn ?? c.createdOn ?? "",
          updatedAtIsCreatedFallback: !c.updatedOn && !!c.createdOn,
        };
      });

      return {
        cases,
        total: res.total ?? cases.length,
        hasMore: res.hasMore ?? false,
      };
    },
    // Keep the previous page's rows/total while the next page loads, so the
    // pager and count stay stable instead of blinking out on page change.
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}
