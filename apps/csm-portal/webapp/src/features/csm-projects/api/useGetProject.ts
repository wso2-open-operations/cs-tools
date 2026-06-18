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
import { apiConfig } from "@config/apiConfig";
import { ApiQueryKeys } from "@constants/apiConstants";
import { ApiError, parseApiResponseMessage } from "@utils/ApiError";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";

/**
 * Look up a single project by id via `GET /projects/{id}` (BFF passthrough to
 * the entity service). The response is the enriched {@link ProjectDetails} shape
 * with the parent `account` embedded. Returns `null` (not an error) on 404 so
 * the page can render a not-found state instead of an error banner. The query
 * stays disabled until an id is present so a bare `/projects/` never fires.
 */
export function useGetProject(
  id: string | undefined,
): UseQueryResult<ProjectDetails | null, Error> {
  const authFetch = useAuthApiClient();

  return useQuery<ProjectDetails | null, Error>({
    queryKey: [ApiQueryKeys.CSM_PROJECT_DETAIL, id ?? ""],
    queryFn: async (): Promise<ProjectDetails | null> => {
      if (!id) return null;

      const res = await authFetch(
        `${apiConfig.backendUrl}/projects/${encodeURIComponent(id)}`,
      );
      if (res.status === 404) return null;
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as ProjectDetails;
    },
    enabled: !!id,
    staleTime: 30_000,
  });
}
