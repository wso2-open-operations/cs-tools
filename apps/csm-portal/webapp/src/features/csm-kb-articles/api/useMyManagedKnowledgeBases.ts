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

interface MyManagedKnowledgeBasesResponse {
  knowledgeBaseIds: string[];
}

/** Which knowledge bases the current user may review articles for --
 * drives the "To Review" tab so it only ever shows articles the caller can
 * actually act on, rather than every pending article with buttons hidden
 * per row. */
export function useMyManagedKnowledgeBases(): UseQueryResult<MyManagedKnowledgeBasesResponse | null, Error> {
  const api = useBackendApi();
  return useQuery<MyManagedKnowledgeBasesResponse | null, Error>({
    queryKey: [ApiQueryKeys.CSM_KB_ARTICLES, "my-managed-knowledge-bases"],
    queryFn: () => api.get<MyManagedKnowledgeBasesResponse>("/kb-managers/my-knowledge-bases"),
    staleTime: 60_000,
  });
}
