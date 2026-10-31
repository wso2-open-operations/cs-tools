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

import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";

/** One row of GET /sla-status/manage -- entity-service's raw SLAStatus shape, passed through unchanged. */
export interface ActiveSlaStatus {
  caseId: string;
  clockType: string;
  businessElapsedPercent: number;
  hasBreached: boolean;
  isPaused: boolean;
  startedOn: string | null;
  pausedOn: string | null;
}

interface SearchActiveSlaStatusResponse {
  statuses: ActiveSlaStatus[];
  total: number;
}

const ACTIVE_SLA_STATUSES_KEY = ["admin", "sla-status", "manage"] as const;

/**
 * Every currently-active SLA clock, for the Recalculate SLA admin page.
 * Capped at 200 rows -- this page is for a quick sanity check after a
 * recalculation, not a full operational view of every clock in the system.
 */
export function useActiveSlaStatuses(): UseQueryResult<SearchActiveSlaStatusResponse, Error> {
  const api = useBackendApi();
  return useQuery<SearchActiveSlaStatusResponse, Error>({
    queryKey: ACTIVE_SLA_STATUSES_KEY,
    queryFn: async () =>
      (await api.get<SearchActiveSlaStatusResponse>("/sla-status/manage?limit=200&source=csm")) ?? { statuses: [], total: 0 },
  });
}

/**
 * The "Recalculate SLA" admin action (POST /sla-engine/recalculate). A 204
 * means the request was published, not that the rebuild on
 * csm-notification-service's side has finished -- the caller should refetch
 * useActiveSlaStatuses after a short delay to show progress, not assume it's
 * already done.
 */
export function useRequestSlaRecalculation(): UseMutationResult<void, Error, void> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<void, Error, void>({
    mutationFn: async () => {
      await api.post<Record<string, never>, null>("/sla-engine/recalculate", {});
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ACTIVE_SLA_STATUSES_KEY });
    },
  });
}
