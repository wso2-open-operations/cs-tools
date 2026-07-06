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
import type { BeChangeRequestDetail } from "@api/backend/types";

/**
 * Look up a single change request by id via `GET /change-requests/{id}`
 * (ServiceNow data source only). Returns `null` when the id is unknown so the
 * detail page can render a not-found state rather than an error.
 */
export function useGetChangeRequest(
  id: string | undefined,
): UseQueryResult<BeChangeRequestDetail | null, Error> {
  const api = useBackendApi();

  return useQuery<BeChangeRequestDetail | null, Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, id ?? ""],
    queryFn: (): Promise<BeChangeRequestDetail | null> =>
      api.get<BeChangeRequestDetail>(
        `/change-requests/${encodeURIComponent(id as string)}`,
      ),
    enabled: !!id,
    staleTime: 30_000,
  });
}
