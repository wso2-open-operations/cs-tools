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
import type { BeProjectMetadata } from "@api/backend/types";

/**
 * Look up a project's reference/feature metadata via `GET /projects/{id}/metadata`
 * (BFF passthrough to the entity service). Returns `null` when the id is unknown
 * so callers that only need an optional feature flag (e.g.
 * `features.srProductCategories`) can fail open rather than treating "not
 * found" as an error. The query stays disabled until an id is present.
 */
export function useProjectMetadata(
  id: string | undefined,
): UseQueryResult<BeProjectMetadata | null, Error> {
  const api = useBackendApi();

  return useQuery<BeProjectMetadata | null, Error>({
    queryKey: [ApiQueryKeys.CSM_PROJECT_METADATA, id ?? ""],
    queryFn: (): Promise<BeProjectMetadata | null> =>
      api.get<BeProjectMetadata>(`/projects/${encodeURIComponent(id as string)}/metadata`),
    enabled: !!id,
    staleTime: 60_000,
  });
}
