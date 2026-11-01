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

import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";

/** One row of GET /sla-status/manage -- entity-service's raw SLAStatus shape, passed through unchanged. */
export interface ActiveSlaStatus {
  caseId: string;
  /** Omitted from the wire (no `omitempty` survivor) when the case has none. */
  caseNumber?: string;
  wso2CaseId?: string;
  caseTitle?: string;
  caseType?: string;
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
  limit: number;
  offset: number;
}

interface ActiveSlaStatusesRequest {
  limit: number;
  offset: number;
}

const ACTIVE_SLA_STATUSES_KEY = ["admin", "sla-status", "manage"] as const;

/**
 * Every currently-active SLA clock, for the Recalculate SLA admin page.
 * entity-service's GET /sla-status already supports limit+offset+total -- see
 * that endpoint's own doc comment -- this just exercises it instead of
 * fetching a single, uncapped first page.
 */
export function useActiveSlaStatuses(
  request: ActiveSlaStatusesRequest,
): UseQueryResult<SearchActiveSlaStatusResponse, Error> {
  const api = useBackendApi();
  return useQuery<SearchActiveSlaStatusResponse, Error>({
    queryKey: [...ACTIVE_SLA_STATUSES_KEY, request],
    queryFn: async () =>
      (await api.get<SearchActiveSlaStatusResponse>(
        `/sla-status/manage?limit=${request.limit}&offset=${request.offset}`,
      )) ?? { statuses: [], total: 0, limit: request.limit, offset: request.offset },
    placeholderData: keepPreviousData,
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
