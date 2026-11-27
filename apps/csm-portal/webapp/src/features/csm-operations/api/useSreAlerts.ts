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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeSreAlertDetail,
  BeSreAlertFacets,
  BeSreAlertSearchPayload,
  BeSreAlertSearchResponse,
} from "@api/backend/types";

/**
 * One page of SRE alerts (`POST /sre-alerts/search`). `keepPreviousData` keeps
 * the table populated while the next page or filter loads.
 */
export function useSearchSreAlerts(
  payload: BeSreAlertSearchPayload,
): UseQueryResult<BeSreAlertSearchResponse, Error> {
  const api = useBackendApi();
  return useQuery<BeSreAlertSearchResponse, Error>({
    queryKey: [ApiQueryKeys.SRE_ALERTS, payload],
    queryFn: () => api.post<BeSreAlertSearchPayload, BeSreAlertSearchResponse>("/sre-alerts/search", payload),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}

/**
 * One SRE alert by its UUID or ALT number (`GET /sre-alerts/{id}`). Resolves to
 * `null` when no alert has that id, so the page can say so instead of erroring.
 */
export function useGetSreAlert(id: string | undefined): UseQueryResult<BeSreAlertDetail | null, Error> {
  const api = useBackendApi();
  return useQuery<BeSreAlertDetail | null, Error>({
    queryKey: [ApiQueryKeys.SRE_ALERT_DETAILS, id ?? ""],
    queryFn: () => api.get<BeSreAlertDetail>(`/sre-alerts/${encodeURIComponent(id ?? "")}`),
    enabled: !!id,
    staleTime: 30_000,
  });
}

/** The filter bar's choices: distinct values of the alerts received in the last 30 days (`GET /sre-alerts/facets`). */
export function useSreAlertFacets(): UseQueryResult<BeSreAlertFacets, Error> {
  const api = useBackendApi();
  return useQuery<BeSreAlertFacets, Error>({
    queryKey: [ApiQueryKeys.SRE_ALERT_FACETS],
    queryFn: async () =>
      (await api.get<BeSreAlertFacets>("/sre-alerts/facets")) ?? {
        environments: [],
        services: [],
        severities: [],
        sources: [],
      },
    staleTime: 5 * 60_000,
  });
}
