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
import type { BeIncidentCreateDefaults } from "@api/backend/types";

/**
 * `GET /incidents/create-defaults`: the Default service and its support group
 * (the "default team") — what an incident is assigned to when its Service has
 * no support group of its own. Configuration, not per-incident data, so it is
 * fetched once and kept for the session (`staleTime: Infinity`).
 */
export function useGetIncidentCreateDefaults(): UseQueryResult<
  BeIncidentCreateDefaults | null,
  Error
> {
  const api = useBackendApi();

  return useQuery<BeIncidentCreateDefaults | null, Error>({
    queryKey: [ApiQueryKeys.INCIDENT_CREATE_DEFAULTS],
    queryFn: (): Promise<BeIncidentCreateDefaults | null> =>
      api.get<BeIncidentCreateDefaults>("/incidents/create-defaults"),
    staleTime: Infinity,
  });
}
