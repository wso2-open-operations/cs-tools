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
  BeAccount,
  BeAccountSearchPayload,
  BeAccountSearchResponse,
  BeProjectSearchPayload,
  BeProjectSearchResponse,
} from "@api/backend/types";
import {
  getMockCsmAccountById,
  getMockCsmAccounts,
  getMockProjectsForAccount,
} from "@features/csm-accounts/api/mocks/accountsMocks";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
import type {
  CsmAccountRow,
  CsmAccountsListResponse,
} from "@features/csm-accounts/types/csmAccounts";
import type { CsmProjectRow } from "@features/csm-projects/types/csmProjects";

const MOCK_LATENCY_MS = 180;
/** Page size when listing accounts. BE caps at 100. */
const ACCOUNTS_PAGE_LIMIT = 100;
const PROJECTS_PAGE_LIMIT = 100;

/**
 * Map a `BeAccount` to the UI row shape. Stat counts (projects, open cases,
 * S0/S1, breached) aren't exposed by the backend so they default to zero;
 * the Account Manager / Technical Owner names aren't either, because the
 * BE only returns user ids — a future pass should hydrate via /users/search.
 */
function rowFromBeAccount(a: BeAccount): CsmAccountRow {
  return {
    id: a.id,
    name: a.name ?? "(unnamed)",
    tier: a.tier === "enterprise" ? "Platinum" : "Silver",
    status: a.deactivationDate ? "Suspended" : "Active",
    accountManager: a.ownerId,
    technicalOwner: a.technicalOwnerId ?? undefined,
    projectCount: 0,
    openCaseCount: 0,
    s0s1Count: 0,
    breachedCount: 0,
    lastActivityAt: a.updatedAt ?? a.createdAt ?? "",
  };
}

/** Cross-customer accounts list. Backed by `POST /accounts/search` in LIVE. */
export function useGetCsmAccounts(
  scope: DashboardScope,
): UseQueryResult<CsmAccountsListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmAccountsListResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNTS, scope],
    queryFn: async (): Promise<CsmAccountsListResponse> => {
      if (isMockMode()) {
        logger.debug(`[useGetCsmAccounts] mock for scope=${scope}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmAccounts(scope);
      }
      const response = await api.post<
        BeAccountSearchPayload,
        BeAccountSearchResponse
      >("/accounts/search", {
        pagination: { offset: 0, limit: ACCOUNTS_PAGE_LIMIT },
      });
      return {
        scope,
        accounts: (response.accounts ?? []).map(rowFromBeAccount),
      };
    },
    staleTime: 30_000,
  });
}

/**
 * Single-account lookup. The backend has no `/accounts/{id}`; this falls
 * through to `/accounts/search` and filters client-side. Acceptable while
 * account counts are small (< 100); a dedicated endpoint should land before
 * this scales.
 */
export function useGetCsmAccountDetail(
  accountId: string | undefined,
): UseQueryResult<CsmAccountRow | null, Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmAccountRow | null, Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNT_DETAIL, accountId ?? ""],
    queryFn: async (): Promise<CsmAccountRow | null> => {
      if (!accountId) return null;
      if (isMockMode()) {
        logger.debug(`[useGetCsmAccountDetail] mock for ${accountId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmAccountById(accountId) ?? null;
      }
      const response = await api.post<
        BeAccountSearchPayload,
        BeAccountSearchResponse
      >("/accounts/search", {
        pagination: { offset: 0, limit: ACCOUNTS_PAGE_LIMIT },
      });
      const match = (response.accounts ?? []).find((a) => a.id === accountId);
      return match ? rowFromBeAccount(match) : null;
    },
    enabled: !!accountId,
    staleTime: 30_000,
  });
}

/**
 * Projects belonging to a single account. The backend has no project filter
 * by accountId; this fetches `/projects/search` and filters client-side.
 * Same scalability caveat as `useGetCsmAccountDetail`.
 */
export function useGetProjectsForAccount(
  accountId: string | undefined,
): UseQueryResult<CsmProjectRow[], Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmProjectRow[], Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNT_PROJECTS, accountId ?? ""],
    queryFn: async (): Promise<CsmProjectRow[]> => {
      if (!accountId) return [];
      if (isMockMode()) {
        logger.debug(`[useGetProjectsForAccount] mock for ${accountId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockProjectsForAccount(accountId);
      }
      const response = await api.post<
        BeProjectSearchPayload,
        BeProjectSearchResponse
      >("/projects/search", {
        pagination: { offset: 0, limit: PROJECTS_PAGE_LIMIT },
      });
      return (response.projects ?? [])
        .filter((p) => p.accountId === accountId)
        .map((p) => ({
          id: p.id,
          name: p.name ?? p.projectKey ?? p.id,
          customer: "—",
          accountId: p.accountId ?? "",
          tier: "Silver",
          productType: "—",
          status: "Active",
          updateLevel: "—",
          openCaseCount: 0,
          s0s1Count: 0,
          breachedCount: 0,
          lastActivityAt: p.updatedAt ?? p.createdAt ?? "",
        })) as CsmProjectRow[];
    },
    enabled: !!accountId,
    staleTime: 30_000,
  });
}
