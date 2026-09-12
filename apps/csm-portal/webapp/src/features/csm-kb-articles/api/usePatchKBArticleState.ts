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
  UpdateKBArticleStateRequest,
  UpdateKBArticleStateResponse,
} from "@features/csm-kb-articles/types/csmKbArticles";

/**
 * Transition a KB article's state via `PATCH /kb-articles/{id}/state` —
 * submit, approve, reject, edit-published, or retire. Who is actually
 * allowed to call this for a given transition is enforced server-side (see
 * the backend's PatchKBArticleState handler); a 403 here means the caller
 * isn't the article's author or a manager of its knowledge base, as
 * appropriate for the requested transition.
 *
 * Invalidates both the detail query (fresh `state`/`rejectionComment`) and
 * the list query (the item's position may have moved between list views,
 * e.g. out of the review queue once approved).
 */
export function usePatchKBArticleState(
  articleId: string | undefined,
): UseMutationResult<UpdateKBArticleStateResponse, Error, UpdateKBArticleStateRequest> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<UpdateKBArticleStateResponse, Error, UpdateKBArticleStateRequest>({
    mutationFn: (input) => {
      if (!articleId) {
        throw new Error("Cannot update a KB article without an id.");
      }
      return api.patch<UpdateKBArticleStateRequest, UpdateKBArticleStateResponse>(
        `/kb-articles/${encodeURIComponent(articleId)}/state`,
        input,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_KB_ARTICLE_DETAIL, articleId ?? ""],
      });
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_KB_ARTICLES],
      });
    },
  });
}
