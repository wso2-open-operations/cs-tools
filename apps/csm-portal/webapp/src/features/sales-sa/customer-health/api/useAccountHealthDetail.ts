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

import { useQuery, useMutation, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { BackendApiError } from "@api/backend/client";
import type { AccountDetail, ProjectHealthStatus } from "./customerHealthTypes";

// GET /customer-health/accounts/{accountId} — 404 (not the old
// Ballerina backend's 500) on an unknown account. See
// CustomerHealthDetailTable's own 404-check for why this matters.
export function useAccountHealthDetail(
  accountId: string | undefined,
): UseQueryResult<AccountDetail | null, Error> {
  const backendApi = useBackendApi();
  return useQuery<AccountDetail | null, Error>({
    queryKey: ["spl-customer-health-account-detail", accountId],
    enabled: Boolean(accountId),
    queryFn: () => backendApi.get<AccountDetail>(`/customer-health/accounts/${encodeURIComponent(accountId ?? "")}`),
  });
}

export function isNotFoundError(error: unknown): boolean {
  return error instanceof BackendApiError && error.status === 404;
}

// GET /customer-health/accounts/{accountSysId}/health-status
export function useAccountHealthStatus(
  accountId: string | undefined,
): UseQueryResult<ProjectHealthStatus[], Error> {
  const backendApi = useBackendApi();
  return useQuery<ProjectHealthStatus[], Error>({
    queryKey: ["spl-customer-health-health-status", accountId],
    enabled: Boolean(accountId),
    queryFn: async () =>
      (await backendApi.get<ProjectHealthStatus[]>(
        `/customer-health/accounts/${encodeURIComponent(accountId ?? "")}/health-status`,
      )) ?? [],
  });
}

// POST /customer-health/accounts/{accountSysId}/init-health-tracking —
// idempotent (existing rows untouched), fired once the project list is
// known. Confirmed against the Go handler (customer_health.go's
// InitHealthTracking): returns 202 Accepted with NO body at all — plain
// `backendApi.post()` would throw trying to `.json()` an empty response, so
// this calls `.post<..., unknown>()` and treats a JSON-parse failure on an
// otherwise-ok response as the expected empty-202 case rather than a real
// error (any other failure — network, 4xx/5xx — still throws normally,
// since `useBackendApi` throws `BackendApiError` for those before ever
// reaching `.json()`).
export function useInitHealthTracking() {
  const backendApi = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<void, Error, { accountId: string; projectSysIds: string[] }>({
    mutationFn: async ({ accountId, projectSysIds }) => {
      try {
        await backendApi.post<{ projectSysIds: string[] }, unknown>(
          `/customer-health/accounts/${encodeURIComponent(accountId)}/init-health-tracking`,
          { projectSysIds },
        );
      } catch (err) {
        if (err instanceof SyntaxError) return; // expected: empty 202 body
        throw err;
      }
    },
    onSuccess: (_data, { accountId }) => {
      void queryClient.invalidateQueries({ queryKey: ["spl-customer-health-health-status", accountId] });
    },
  });
}
