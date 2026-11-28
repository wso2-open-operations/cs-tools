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
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCommentTemplate,
  BeCommentTemplateList,
  BeCommentTemplatePayload,
} from "@api/backend/types";
import { ApiQueryKeys } from "@constants/apiConstants";

const PERSONAL_PATH = "/users/me/case-comment-templates";
const SHARED_PATH = "/case-comment-templates";

function templatePath(shared: boolean, id?: string): string {
  const base = shared ? SHARED_PATH : PERSONAL_PATH;
  return id ? `${base}/${encodeURIComponent(id)}` : base;
}

/**
 * The caller's own comment templates followed by the shared ones. Only
 * fetched while `enabled` (the comment box opens the list on demand).
 */
export function useGetCommentTemplates(
  enabled = true,
): UseQueryResult<BeCommentTemplate[], Error> {
  const api = useBackendApi();
  return useQuery<BeCommentTemplate[], Error>({
    queryKey: [ApiQueryKeys.COMMENT_TEMPLATES],
    queryFn: async () => {
      const res = await api.get<BeCommentTemplateList>(PERSONAL_PATH);
      return res?.templates ?? [];
    },
    enabled,
    staleTime: 5 * 60 * 1000,
  });
}

export interface SaveCommentTemplateInput {
  /** Set to update an existing template; unset creates one. */
  id?: string;
  /** Which collection: the caller's own, or the shared one (admins only). */
  shared: boolean;
  payload: BeCommentTemplatePayload;
}

/** Creates or updates a template, then refreshes the list. */
export function useSaveCommentTemplate(): UseMutationResult<
  BeCommentTemplate,
  Error,
  SaveCommentTemplateInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<BeCommentTemplate, Error, SaveCommentTemplateInput>({
    mutationFn: ({ id, shared, payload }) =>
      id
        ? api.patch<BeCommentTemplatePayload, BeCommentTemplate>(templatePath(shared, id), payload)
        : api.post<BeCommentTemplatePayload, BeCommentTemplate>(templatePath(shared), payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.COMMENT_TEMPLATES] });
    },
  });
}

/** Deletes a template, then refreshes the list. */
export function useDeleteCommentTemplate(): UseMutationResult<
  void,
  Error,
  { id: string; shared: boolean }
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<void, Error, { id: string; shared: boolean }>({
    mutationFn: ({ id, shared }) => api.delete(templatePath(shared, id)),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.COMMENT_TEMPLATES] });
    },
  });
}
