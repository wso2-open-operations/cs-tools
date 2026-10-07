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
  BeProblemSearchPayload,
  BeProblemSearchResponse,
} from "@api/backend/types";

/** Don't fire a search until the user has typed something searchable. */
export const QUICK_PROBLEM_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_PROBLEM_LIMIT = 5;

/** A problem number, e.g. "PRB0040192" — always "PRB" plus exactly 7 digits. */
const PROBLEM_NUMBER_RE = /^PRB\d{7}$/;

/**
 * What kind of lookup a typed quick-search string should run as — mirrors
 * {@link "@features/csm-cases/api/useQuickCaseSearch".QuickCaseSearchScope},
 * just without the internalId variant cases have (problems have only one
 * exact-match identifier).
 */
export type QuickProblemSearchScope = "number" | "text";

/**
 * Classifies a typed (already-trimmed) quick-search string into the scope
 * {@link useQuickProblemSearch} should route it through.
 */
export function classifyQuickProblemQuery(
  query: string,
): QuickProblemSearchScope {
  return PROBLEM_NUMBER_RE.test(query) ? "number" : "text";
}

/** One hit from the global-search problem lookup, enough for the palette's result row. */
export interface QuickProblemHit {
  id: string;
  number?: string;
  subject: string;
  state?: string;
  assigneeName?: string;
}

/**
 * Problem lookup for the global quick-nav palette. Calls
 * `POST /problems/search` (ServiceNow data source only) — same endpoint
 * `useSearchProblems` uses for the Operations tab's listing, just capped to
 * a handful of hits and shaped for the palette instead of a paginated table.
 * Routes the typed text one of two ways (see {@link classifyQuickProblemQuery}):
 * a `PRB\d{7}` problem number goes through as an exact-match, first-class
 * field filter (an indexed lookup); anything else falls back to the same
 * free-text `searchQuery` search the problems list uses. Pass
 * `forceFreeText` to opt back into the free-text path even when the query
 * matches the exact pattern — the quick-nav palette's "search in subject and description too" affordance uses this to widen a scoped result.
 *
 * Disabled until the trimmed text reaches {@link QUICK_PROBLEM_MIN_QUERY_LEN},
 * so opening the palette costs no network.
 */
export function useQuickProblemSearch(
  query: string,
  options?: { forceFreeText?: boolean },
): UseQueryResult<QuickProblemHit[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  const scope = options?.forceFreeText ? "text" : classifyQuickProblemQuery(q);

  return useQuery<QuickProblemHit[], Error>({
    queryKey: [ApiQueryKeys.PROBLEMS, "quick-search", q, scope],
    queryFn: async (): Promise<QuickProblemHit[]> => {
      const res = await postSkippingTotal<BeProblemSearchPayload, BeProblemSearchResponse>(
        api,
        "/problems/search",
        {
          pagination: { offset: 0, limit: QUICK_PROBLEM_LIMIT },
          filters: scope === "number" ? { number: q } : { searchQuery: q },
        },
      );
      return (res.problems ?? []).map((p) => ({
        id: p.id,
        number: p.number,
        subject: p.subject ?? "(no subject)",
        state: p.state,
        assigneeName: p.assignedTo?.name,
      }));
    },
    enabled: q.length >= QUICK_PROBLEM_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
