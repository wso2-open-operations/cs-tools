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

import type { QueryClient } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { CaseType } from "@features/support/constants/supportConstants";

type AuthFetch = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

type RefreshCaseType =
  | typeof CaseType.DEFAULT_CASE
  | typeof CaseType.SERVICE_REQUEST;

/**
 * Calls backend endpoints required after successful case/service request creation.
 *
 * @param {AuthFetch} authFetch - Authorized fetch function.
 * @param {string} projectId - Project id.
 * @param {RefreshCaseType} caseType - Created case type.
 * @returns {Promise<void>} Promise that resolves after best-effort API calls.
 */
export async function triggerPostCreationApiCalls(
  authFetch: AuthFetch,
  projectId: string,
  caseType: RefreshCaseType,
): Promise<void> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl || !projectId) return;

  const statsAllUrl = `${baseUrl}/projects/${projectId}/stats/cases?caseTypes=${CaseType.DEFAULT_CASE}&caseTypes=${CaseType.SERVICE_REQUEST}&caseTypes=${CaseType.ENGAGEMENT}`;
  const statsTypeUrl = `${baseUrl}/projects/${projectId}/stats/cases?caseTypes=${caseType}`;
  const casesSearchUrl = `${baseUrl}/projects/${projectId}/cases/search`;

  await Promise.allSettled([
    authFetch(statsAllUrl, { method: "GET" }),
    authFetch(statsTypeUrl, { method: "GET" }),
    authFetch(casesSearchUrl, {
      method: "POST",
      body: JSON.stringify({
        filters: { caseTypes: [caseType] },
        pagination: { limit: 10, offset: 0 },
      }),
    }),
  ]);
}

/**
 * Invalidates and refetches React Query caches affected by case/service-request creation.
 *
 * @param {QueryClient} queryClient - React Query client.
 * @param {string} projectId - Project id.
 * @param {RefreshCaseType} caseType - Created case type.
 * @returns {Promise<void>} Promise that resolves once refetch/invalidation requests are sent.
 */
export async function refreshCaseQueriesAfterCreation(
  queryClient: QueryClient,
  projectId: string,
  caseType: RefreshCaseType,
): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey as unknown[];
        if (key[0] !== ApiQueryKeys.CASES_STATS || key[1] !== projectId) {
          return false;
        }
        const caseTypes = key[4];
        if (!Array.isArray(caseTypes)) {
          return true;
        }
        const normalized = caseTypes.map((t) => String(t));
        return (
          normalized.includes(caseType) ||
          (normalized.includes(CaseType.DEFAULT_CASE) &&
            normalized.includes(CaseType.SERVICE_REQUEST) &&
            normalized.includes(CaseType.ENGAGEMENT))
        );
      },
    }),
    queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey as unknown[];
        if (key[0] !== ApiQueryKeys.PROJECT_CASES) return false;
        const isPageQuery = key[1] === "page";
        const projectIdIdx = isPageQuery ? 2 : 1;
        if (key[projectIdIdx] !== projectId) return false;
        const baseRequest = (isPageQuery ? key[3] : key[2]) as
          | { filters?: { caseTypes?: string[] } }
          | undefined;
        const types = baseRequest?.filters?.caseTypes ?? [];
        return types.includes(caseType);
      },
    }),
    queryClient.refetchQueries({
      queryKey: [ApiQueryKeys.CASES_STATS, projectId],
    }),
    queryClient.refetchQueries({
      queryKey: [ApiQueryKeys.PROJECT_CASES],
    }),
    queryClient.invalidateQueries({
      queryKey: [ApiQueryKeys.CASES_STATS, projectId],
    }),
    queryClient.invalidateQueries({
      queryKey: [ApiQueryKeys.PROJECT_CASES],
    }),
  ]);
}
