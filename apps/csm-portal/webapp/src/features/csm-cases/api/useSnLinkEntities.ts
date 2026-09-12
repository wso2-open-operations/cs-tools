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
import type { BeAlertDetail, BeSmartAlertDetail } from "@api/backend/types";

/**
 * Look up a single alert by id via `GET /alerts/{id}`. Backs the "View alert"
 * marker `replaceSnLinks` leaves in place of an alert reference embedded in a
 * comment/work-note body (see `snLinkRegistry`). Returns `null` on a 404 (a
 * stale/unknown id) so the modal can render a not-found state instead of
 * throwing. `enabled` gates the fetch on a modal actually being open for this
 * id — a long comment thread can carry many alert references and none should
 * be prefetched.
 */
export function useGetAlert(
  id: string | undefined,
): UseQueryResult<BeAlertDetail | null, Error> {
  const api = useBackendApi();

  return useQuery<BeAlertDetail | null, Error>({
    queryKey: [ApiQueryKeys.ALERT_DETAILS, id ?? ""],
    queryFn: (): Promise<BeAlertDetail | null> =>
      api.get<BeAlertDetail>(`/alerts/${encodeURIComponent(id as string)}`),
    enabled: !!id,
    staleTime: 30_000,
  });
}

/**
 * Look up a single smart alert by id via `GET /smart-alerts/{id}`. Same
 * shape/rationale as {@link useGetAlert}, for the "View smart alert" marker.
 */
export function useGetSmartAlert(
  id: string | undefined,
): UseQueryResult<BeSmartAlertDetail | null, Error> {
  const api = useBackendApi();

  return useQuery<BeSmartAlertDetail | null, Error>({
    queryKey: [ApiQueryKeys.SMART_ALERT_DETAILS, id ?? ""],
    queryFn: (): Promise<BeSmartAlertDetail | null> =>
      api.get<BeSmartAlertDetail>(
        `/smart-alerts/${encodeURIComponent(id as string)}`,
      ),
    enabled: !!id,
    staleTime: 30_000,
  });
}
