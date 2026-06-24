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
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
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

// The case-search API has no assignee or work-state filter, so we narrow by
// state on the server and match assignee + ongoing on the client. The
// work_in_progress set is cross-customer, so page through it rather than
// inspecting only the first page — otherwise the caller's ongoing case could
// sit beyond the first page and the single-active-case guard would miss it.
const SEARCH_LIMIT = 100;
// Safety bound on the scan (pages * limit) so a pathological dataset can't spin
// this on-demand check forever.
const MAX_PAGES = 20;

/**
 * Returns a function that finds the **other** cases the signed-in engineer is
 * actively working — `work_in_progress`, `workState` `ongoing`, and assigned to
 * them — excluding `excludeCaseId`. Used to enforce the single-active-case rule
 * when starting work on a case.
 *
 * Because the search endpoint exposes neither an assignee nor a work-state
 * filter, this narrows by `states: [work_in_progress]` server-side and then
 * matches `assignedEngineer.email` against the JWT email and `workState ===
 * "ongoing"` client-side (a `null` workState is never ongoing).
 */
export function useFindMyOngoingCases(): (
  excludeCaseId: string,
) => Promise<MyOngoingCase[]> {
  const api = useBackendApi();
  const myEmail = useIdTokenClaims()?.email?.toLowerCase();

  return useCallback(
    async (excludeCaseId: string): Promise<MyOngoingCase[]> => {
      // Without our email we can't tell which cases are ours; skip the prompt.
      if (!myEmail) return [];

      const matches: MyOngoingCase[] = [];
      for (let page = 0; page < MAX_PAGES; page += 1) {
        const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
          "/cases/search",
          {
            filters: { states: ["work_in_progress"] },
            pagination: { offset: page * SEARCH_LIMIT, limit: SEARCH_LIMIT },
          },
        );
        const rows = res.cases ?? [];
        for (const c of rows) {
          if (
            c.id !== excludeCaseId &&
            // A null/absent workState is never "ongoing".
            c.workState === "ongoing" &&
            c.assignedEngineer?.email?.toLowerCase() === myEmail
          ) {
            matches.push({
              id: c.id,
              label: c.internalId || c.number || c.subject || c.id,
            });
          }
        }
        if (rows.length < SEARCH_LIMIT || !(res.hasMore ?? false)) break;
      }
      return matches;
    },
    [api, myEmail],
  );
}
