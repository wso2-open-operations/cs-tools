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
import { useBackendApi, type BackendApi } from "@api/backend/client";
import type {
  BeProject,
  BeProjectSearchPayload,
  BeProjectSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = 100; // backend caps pagination limit at 100
// Cap the scan so a large project catalog can't fire unbounded sequential
// requests (~2000 projects covered).
const MAX_PAGES = 20;

/**
 * Query options for the full project directory, via `POST /projects/search`.
 * Exported (rather than only the hook) so non-component code — e.g. the
 * `useGetCsmCases` queryFn — can resolve the same cached data through
 * `queryClient.fetchQuery` instead of re-fetching on every cases query.
 */
export function projectOptionsQueryOptions(api: BackendApi) {
  return {
    queryKey: [ApiQueryKeys.CSM_PROJECTS, "options"],
    queryFn: async (): Promise<BeProject[]> => {
      // Page through so projects beyond the first page are still selectable.
      const all: BeProject[] = [];
      let offset = 0;
      for (let page = 0; page < MAX_PAGES; page += 1) {
        const res = await api.post<
          BeProjectSearchPayload,
          BeProjectSearchResponse
        >("/projects/search", { pagination: { offset, limit: PAGE_LIMIT } });
        const projects = res.projects ?? [];
        all.push(...projects);
        if (projects.length < PAGE_LIMIT) break;
        offset += PAGE_LIMIT;
      }
      return all;
    },
    staleTime: 60_000,
  } as const;
}

/** Projects for the case-create / case-filter project selectors. */
export function useProjectOptions(): UseQueryResult<BeProject[], Error> {
  const api = useBackendApi();

  return useQuery<BeProject[], Error>(projectOptionsQueryOptions(api));
}
