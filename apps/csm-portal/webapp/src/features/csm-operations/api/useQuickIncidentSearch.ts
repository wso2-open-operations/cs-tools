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
  BeIncidentSearchPayload,
  BeIncidentSearchResponse,
} from "@api/backend/types";

/** Don't fire a search until the user has typed something searchable — mirrors {@link
 * "@features/csm-cases/api/useQuickCaseSearch".QUICK_CASE_MIN_QUERY_LEN}. */
export const QUICK_INCIDENT_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_INCIDENT_LIMIT = 5;

/** An incident number, e.g. "INC0090472" — always "INC" plus exactly 7 digits. */
const INCIDENT_NUMBER_RE = /^INC\d{7}$/;

/**
 * What kind of lookup a typed quick-search string should run as — mirrors
 * {@link "@features/csm-cases/api/useQuickCaseSearch".QuickCaseSearchScope},
 * just without the internalId variant cases have (incidents have only one
 * exact-match identifier).
 */
export type QuickIncidentSearchScope = "number" | "text";

/**
 * Classifies a typed (already-trimmed) quick-search string into the scope
 * {@link useQuickIncidentSearch} should route it through.
 */
export function classifyQuickIncidentQuery(
  query: string,
): QuickIncidentSearchScope {
  return INCIDENT_NUMBER_RE.test(query) ? "number" : "text";
}

/** One hit from the global-search incident lookup, enough for the palette's result row. */
export interface QuickIncidentHit {
  id: string;
  number?: string | null;
  subject: string;
  state?: string | null;
  priority?: string | null;
  assigneeName?: string;
  updatedOn?: string;
}

/**
 * Incident lookup for the global quick-nav palette. Calls
 * `POST /incidents/search` (ServiceNow data source only) — same endpoint
 * `useSearchIncidents` uses for the Operations tab's listing, just capped to
 * a handful of hits and shaped for the palette instead of a paginated table.
 * Routes the typed text one of two ways (see {@link classifyQuickIncidentQuery}):
 * an `INC\d{7}` incident number goes through as an exact-match, first-class
 * field filter (an indexed lookup); anything else falls back to the same
 * free-text `searchQuery` search the incidents list uses. Pass
 * `forceFreeText` to opt back into the free-text path even when the query
 * matches the exact pattern — the quick-nav palette's "search in subject and description too" affordance uses this to widen a scoped result.
 *
 * Disabled until the trimmed text reaches {@link QUICK_INCIDENT_MIN_QUERY_LEN},
 * so opening the palette costs no network.
 */
export function useQuickIncidentSearch(
  query: string,
  options?: { forceFreeText?: boolean },
): UseQueryResult<QuickIncidentHit[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  const scope = options?.forceFreeText ? "text" : classifyQuickIncidentQuery(q);

  return useQuery<QuickIncidentHit[], Error>({
    queryKey: [ApiQueryKeys.INCIDENTS, "quick-search", q, scope],
    queryFn: async (): Promise<QuickIncidentHit[]> => {
      const res = await postSkippingTotal<BeIncidentSearchPayload, BeIncidentSearchResponse>(
        api,
        "/incidents/search",
        {
          pagination: { offset: 0, limit: QUICK_INCIDENT_LIMIT },
          filters: scope === "number" ? { number: q } : { searchQuery: q },
        },
      );
      return (res.incidents ?? [])
        .filter((i): i is typeof i & { id: string } => !!i.id)
        .map((i) => ({
          id: i.id,
          number: i.number,
          subject: i.subject ?? "(no subject)",
          state: i.state,
          priority: i.priority,
          assigneeName: i.assignedTo?.name,
          updatedOn: i.updatedOn,
        }));
    },
    enabled: q.length >= QUICK_INCIDENT_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
