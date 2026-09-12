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
import { ApiQueryKeys } from "@constants/apiConstants";
import type { SearchKBManagersRequest, SearchKBManagersResponse } from "@features/csm-kb-articles/types/csmKbArticles";

export function useSearchKBManagers(knowledgeBaseId: string | undefined): UseQueryResult<SearchKBManagersResponse | null, Error> {
  const api = useBackendApi();
  return useQuery<SearchKBManagersResponse | null, Error>({
    queryKey: [ApiQueryKeys.CSM_KB_ARTICLES, "kb-managers", knowledgeBaseId ?? ""],
    queryFn: () =>
      api.post<SearchKBManagersRequest, SearchKBManagersResponse>("/kb-managers/search", { knowledgeBaseId }),
    enabled: Boolean(knowledgeBaseId),
  });
}
