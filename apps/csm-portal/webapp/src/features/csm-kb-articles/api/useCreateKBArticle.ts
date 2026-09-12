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
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  CreateKBArticleRequest,
  CreateKBArticleResponse,
} from "@features/csm-kb-articles/types/csmKbArticles";

/**
 * Create a new KB article via `POST /kb-articles`. Always lands in the
 * `draft` state — submitting for review is a separate action
 * (`usePatchKBArticleState`). Invalidates the list query on success so the
 * new draft appears immediately without a manual refetch.
 */
export function useCreateKBArticle(): UseMutationResult<CreateKBArticleResponse, Error, CreateKBArticleRequest> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CreateKBArticleResponse, Error, CreateKBArticleRequest>({
    mutationFn: (input) =>
      api.post<CreateKBArticleRequest, CreateKBArticleResponse>(
        "/kb-articles",
        input,
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_KB_ARTICLES],
      });
    },
  });
}
