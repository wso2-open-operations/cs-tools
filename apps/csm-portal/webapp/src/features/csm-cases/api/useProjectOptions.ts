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
import { useBackendApi } from "@api/backend/client";
import type {
  BeProject,
  BeProjectSearchPayload,
  BeProjectSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = 100; // backend caps pagination limit at 100

/** Projects for the case-create project selector, via `POST /projects/search`. */
export function useProjectOptions(): UseQueryResult<BeProject[], Error> {
  const api = useBackendApi();

  return useQuery<BeProject[], Error>({
    queryKey: ["csm-case-create-projects"],
    queryFn: async (): Promise<BeProject[]> => {
      const res = await api.post<
        BeProjectSearchPayload,
        BeProjectSearchResponse
      >("/projects/search", { pagination: { offset: 0, limit: PAGE_LIMIT } });
      return res.projects ?? [];
    },
    staleTime: 60_000,
  });
}
