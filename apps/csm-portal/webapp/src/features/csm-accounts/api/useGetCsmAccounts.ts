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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
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

export function useGetCsmAccounts(
  scope: DashboardScope,
): UseQueryResult<CsmAccountsListResponse, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmAccountsListResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNTS, scope],
    queryFn: async (): Promise<CsmAccountsListResponse> => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(`[useGetCsmAccounts] mock for scope=${scope}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmAccounts(scope);
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/accounts?scope=${encodeURIComponent(scope)}`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(`Error fetching accounts: ${response.statusText}`);
      }
      return (await response.json()) as CsmAccountsListResponse;
    },
    staleTime: 30_000,
  });
}

export function useGetCsmAccountDetail(
  accountId: string | undefined,
): UseQueryResult<CsmAccountRow | null, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmAccountRow | null, Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNT_DETAIL, accountId ?? ""],
    queryFn: async (): Promise<CsmAccountRow | null> => {
      if (!accountId) return null;
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(`[useGetCsmAccountDetail] mock for ${accountId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmAccountById(accountId) ?? null;
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/accounts/${encodeURIComponent(accountId)}`;
      const response = await authFetch(url, { method: "GET" });
      if (response.status === 404) return null;
      if (!response.ok) {
        throw new Error(`Error fetching account: ${response.statusText}`);
      }
      return (await response.json()) as CsmAccountRow;
    },
    enabled: !!accountId,
    staleTime: 30_000,
  });
}

export function useGetProjectsForAccount(
  accountId: string | undefined,
): UseQueryResult<CsmProjectRow[], Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmProjectRow[], Error>({
    queryKey: [ApiQueryKeys.CSM_ACCOUNT_PROJECTS, accountId ?? ""],
    queryFn: async (): Promise<CsmProjectRow[]> => {
      if (!accountId) return [];
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(`[useGetProjectsForAccount] mock for ${accountId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockProjectsForAccount(accountId);
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/accounts/${encodeURIComponent(accountId)}/projects`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(
          `Error fetching projects for account ${accountId}: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmProjectRow[];
    },
    enabled: !!accountId,
    staleTime: 30_000,
  });
}
