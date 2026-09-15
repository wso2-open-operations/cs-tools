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

import {
  keepPreviousData,
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  SearchKBArticlesRequest,
  SearchKBArticlesResponse,
} from "@features/csm-kb-articles/types/csmKbArticles";

/**
 * Search KB articles via `POST /kb-articles/search`. Used by both the
 * engineer's own-article list (filtered by authorId) and the manager's
 * review queue (filtered by states: ["pending_review"]) — same endpoint,
 * different filter shape.
 */
export function useSearchKBArticles(
  request: SearchKBArticlesRequest,
): UseQueryResult<SearchKBArticlesResponse, Error> {
  const api = useBackendApi();

  return useQuery<SearchKBArticlesResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_KB_ARTICLES, request],
    queryFn: () =>
      api.post<SearchKBArticlesRequest, SearchKBArticlesResponse>(
        "/kb-articles/search",
        request,
      ),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}
