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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import { postSkippingTotal } from "@api/backend/postSkippingTotal";
import type {
  BeChangeRequestSearchPayload,
  BeChangeRequestSearchResponse,
} from "@api/backend/types";

/** Don't fire a search until the user has typed something searchable. */
export const QUICK_CHANGE_REQUEST_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_CHANGE_REQUEST_LIMIT = 5;

/** A change request number, e.g. "CHG0038721" — always "CHG" plus exactly 7 digits. */
const CHANGE_REQUEST_NUMBER_RE = /^CHG\d{7}$/;

/**
 * What kind of lookup a typed quick-search string should run as — mirrors
 * {@link "@features/csm-cases/api/useQuickCaseSearch".QuickCaseSearchScope},
 * just without the internalId variant cases have (change requests have only
 * one exact-match identifier).
 */
export type QuickChangeRequestSearchScope = "number" | "text";

/**
 * Classifies a typed (already-trimmed) quick-search string into the scope
 * {@link useQuickChangeRequestSearch} should route it through.
 */
export function classifyQuickChangeRequestQuery(
  query: string,
): QuickChangeRequestSearchScope {
  return CHANGE_REQUEST_NUMBER_RE.test(query) ? "number" : "text";
}

/** One hit from the global-search change-request lookup, enough for the palette's result row. */
export interface QuickChangeRequestHit {
  id: string;
  number?: string;
  subject: string;
  state?: string | null;
  impact?: string | null;
  assigneeName?: string;
  updatedOn?: string;
}

/**
 * Change-request lookup for the global quick-nav palette. Calls
 * `POST /change-requests/search` (ServiceNow data source only) — same
 * endpoint `useSearchChangeRequests` uses for the Operations tab's listing,
 * just capped to a handful of hits and shaped for the palette instead of a
 * paginated table. Routes the typed text one of two ways (see
 * {@link classifyQuickChangeRequestQuery}): a `CHG\d{7}` change-request
 * number goes through as an exact-match, first-class field filter (an
 * indexed lookup); anything else falls back to the same free-text
 * `searchQuery` search the change-requests list uses. Pass `forceFreeText`
 * to opt back into the free-text path even when the query matches the exact
 * pattern — the quick-nav palette's "search in subject and description too" affordance
 * uses this to widen a scoped result.
 *
 * Disabled until the trimmed text reaches
 * {@link QUICK_CHANGE_REQUEST_MIN_QUERY_LEN}, so opening the palette costs no
 * network.
 */
export function useQuickChangeRequestSearch(
  query: string,
  options?: { forceFreeText?: boolean },
): UseQueryResult<QuickChangeRequestHit[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  const scope = options?.forceFreeText
    ? "text"
    : classifyQuickChangeRequestQuery(q);

  return useQuery<QuickChangeRequestHit[], Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUESTS, "quick-search", q, scope],
    queryFn: async (): Promise<QuickChangeRequestHit[]> => {
      const res = await postSkippingTotal<
        BeChangeRequestSearchPayload,
        BeChangeRequestSearchResponse
      >(api, "/change-requests/search", {
        pagination: { offset: 0, limit: QUICK_CHANGE_REQUEST_LIMIT },
        filters: scope === "number" ? { number: q } : { searchQuery: q },
      });
      return (res.changeRequests ?? []).map((cr) => ({
        id: cr.id,
        number: cr.number,
        subject: cr.subject ?? "(no subject)",
        state: cr.state,
        impact: cr.impact,
        assigneeName: cr.assignedEngineer?.name,
        updatedOn: cr.updatedOn,
      }));
    },
    enabled: q.length >= QUICK_CHANGE_REQUEST_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
