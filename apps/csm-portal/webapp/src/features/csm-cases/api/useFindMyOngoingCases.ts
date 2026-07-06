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

import { useCallback } from "react";
import { useBackendApi } from "@api/backend/client";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";

/** A case the current user is actively working (in-progress + ongoing). */
export interface MyOngoingCase {
  id: string;
  /** Human label for the confirm dialog (WSO2 id / case number / subject). */
  label: string;
}

// Page size = the BE's max (requests above it are rejected). When we filter by
// assignee server-side the result set is tiny (an engineer's own ongoing cases,
// at most a handful), so the loop below exits after the first page. The email
// fallback path is NOT assignee-filtered server-side — it sees every engineer's
// ongoing in-progress case — so it must page until results are exhausted, or
// the caller's ongoing case could sit beyond page one and the guard would miss
// it. MAX_PAGES bounds a pathological set so the on-demand check can't spin.
const SEARCH_LIMIT = BE_MAX_PAGE_LIMIT;
const MAX_PAGES = 20;

/**
 * Returns a function that finds the **other** cases the signed-in engineer is
 * actively working — `work_in_progress`, `workState` `ongoing`, and assigned to
 * them — excluding `excludeCaseId`. Used to enforce the single-active-case rule
 * when starting work on a case.
 *
 * The search is filtered **server-side** by assignee (`assignedUserIds`), state
 * (`work_in_progress`) and work-state (`ongoing`), so we no longer scan the
 * whole cross-customer in-progress set and match on the client. A thin
 * client-side re-check is kept only as a correctness backstop: the search
 * response currently returns the raw ServiceNow work-state label (e.g.
 * `"Ongoing"`) rather than the lowercased enum the detail endpoint returns, so
 * we compare case-insensitively; and if `/users/me` couldn't supply our id the
 * assignee filter is omitted, so we fall back to matching the JWT email. The
 * assignee-filtered path returns a tiny set (one page); the email fallback is
 * unfiltered by assignee and so pages until the results are exhausted.
 */
export function useFindMyOngoingCases(): (
  excludeCaseId: string,
) => Promise<MyOngoingCase[]> {
  const api = useBackendApi();
  const myEmail = useIdTokenClaims()?.email?.toLowerCase();
  const myUserId = useCurrentUser().user?.id;

  return useCallback(
    async (excludeCaseId: string): Promise<MyOngoingCase[]> => {
      // Without our id or email we can't tell which cases are ours; skip.
      if (!myUserId && !myEmail) return [];

      const matches: MyOngoingCase[] = [];
      for (let page = 0; page < MAX_PAGES; page += 1) {
        const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
          "/cases/search",
          {
            filters: {
              states: ["work_in_progress"],
              workStates: ["ongoing"],
              // Filter by assignee server-side when we know our platform id;
              // otherwise (entity service down → `/users/me` omits it) omit the
              // filter and rely on the email backstop below.
              ...(myUserId && { assignedUserIds: [myUserId] }),
            },
            pagination: { offset: page * SEARCH_LIMIT, limit: SEARCH_LIMIT },
          },
        );
        const rows = res.cases ?? [];
        for (const c of rows) {
          if (c.id === excludeCaseId) continue;
          // Backstop against the search endpoint returning the raw SN label
          // ("Ongoing") instead of the lowercased enum: compare case-insensitively.
          if (c.workState?.toLowerCase() !== "ongoing") continue;
          // If we couldn't filter by assignee server-side, match on email here.
          if (
            !myUserId &&
            c.assignedEngineer?.email?.toLowerCase() !== myEmail
          ) {
            continue;
          }
          matches.push({
            id: c.id,
            label: c.internalId || c.number || c.subject || c.id,
          });
        }
        // Assignee-filtered path: this tiny set fits one page and breaks here.
        // Email fallback: keep paging until the search results are exhausted.
        if (rows.length < SEARCH_LIMIT || !(res.hasMore ?? false)) break;
      }
      return matches;
    },
    [api, myEmail, myUserId],
  );
}
