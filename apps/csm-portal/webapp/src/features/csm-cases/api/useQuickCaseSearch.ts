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
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { isMockMode, useBackendApi } from "@api/backend/client";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";
import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";

const MOCK_LATENCY_MS = 120;

/** Don't fire a search until the user has typed something searchable. */
export const QUICK_CASE_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_CASE_LIMIT = 8;

/**
 * One hit from the global-search case lookup. Carries only what the palette
 * needs: the UUID `id` for the `/cases/:id` link and the human-readable
 * identity/subject for display.
 */
export interface QuickCaseHit {
  id: string;
  caseNumber?: string;
  wso2CaseId?: string;
  subject: string;
}

/**
 * Free-text case lookup for the global quick-nav palette. Calls
 * `POST /cases/search` with the typed text as `searchQuery` (the same search the
 * cases list uses), so a CS/WSO2 case id — or any subject text — resolves to
 * matching cases the user can jump straight into.
 *
 * The query is disabled until the trimmed text reaches
 * {@link QUICK_CASE_MIN_QUERY_LEN}, so opening the palette costs no network. In
 * MOCK mode it filters the seeded set client-side on id/subject.
 */
export function useQuickCaseSearch(
  query: string,
): UseQueryResult<QuickCaseHit[], Error> {
  const logger = useLogger();
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<QuickCaseHit[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASES, "quick-search", q],
    queryFn: async (): Promise<QuickCaseHit[]> => {
      if (isMockMode()) {
        logger.debug(`[useQuickCaseSearch] mock case search for "${q}"`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        const needle = q.toLowerCase();
        return getMockCsmCases("all_customers")
          .filter((c) => {
            const id = caseIdLabel(c).toLowerCase();
            return (
              id.includes(needle) || c.subject.toLowerCase().includes(needle)
            );
          })
          .slice(0, QUICK_CASE_LIMIT)
          .map((c) => ({
            id: c.id,
            caseNumber: c.caseNumber,
            wso2CaseId: c.wso2CaseId,
            subject: c.subject,
          }));
      }

      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          pagination: { offset: 0, limit: QUICK_CASE_LIMIT },
          filters: { searchQuery: q },
        },
      );
      return (res.cases ?? []).map((c) => ({
        id: c.id,
        caseNumber: c.number,
        wso2CaseId: c.internalId,
        subject: c.title ?? "(no subject)",
      }));
    },
    enabled: q.length >= QUICK_CASE_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
