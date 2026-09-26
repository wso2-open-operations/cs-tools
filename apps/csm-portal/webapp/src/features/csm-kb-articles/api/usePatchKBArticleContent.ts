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

import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { KBArticle, UpdateKBArticleContentRequest } from "@features/csm-kb-articles/types/csmKbArticles";

/** Edit an existing draft's title/body via `PATCH /kb-articles/{id}`.
 * Only valid while the article is still a draft and the caller is its
 * author -- enforced server-side. */
export function usePatchKBArticleContent(articleId: string | undefined): UseMutationResult<KBArticle, Error, UpdateKBArticleContentRequest> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<KBArticle, Error, UpdateKBArticleContentRequest>({
    mutationFn: (input) => {
      if (!articleId) throw new Error("Cannot update a KB article without an id.");
      return api.patch<UpdateKBArticleContentRequest, KBArticle>(`/kb-articles/${encodeURIComponent(articleId)}`, input);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_KB_ARTICLE_DETAIL, articleId ?? ""] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_KB_ARTICLES] });
    },
  });
}
