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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  ProductAddCommentResponse,
  ProductCommandResponse,
  ProductGitHubCommand,
  ProductIssueDetail,
} from "@features/csm-cases/types/productWorkItems";

function path(workItemId: string, suffix: string): string {
  return `/product-work-items/${encodeURIComponent(workItemId)}${suffix}`;
}

function keyHeaders(key: string | undefined): { headers: Record<string, string> } | undefined {
  return key ? { headers: { "Idempotency-Key": key } } : undefined;
}

/** Shared by the four actions: refetch the detail and the lists that show this issue. */
function useRefreshAfterAction(workItemId: string): () => void {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries({
      queryKey: [ApiQueryKeys.CSM_PRODUCT_WORK_ITEM, workItemId],
    });
    void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASE_PRODUCT_WORK_ITEMS] });
    void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_MY_WOC_PRODUCT_ISSUES] });
  };
}

/** Add the 202 response to the cached detail so it shows before the refetch lands. */
function useMergeIntoDetail(workItemId: string) {
  const queryClient = useQueryClient();
  return (comment: ProductAddCommentResponse["comment"] | null, command: ProductGitHubCommand) => {
    queryClient.setQueryData<ProductIssueDetail | null>(
      [ApiQueryKeys.CSM_PRODUCT_WORK_ITEM, workItemId],
      (old) => {
        if (!old) return old;
        const commands = [command, ...(old.commands ?? []).filter((c) => c.id !== command.id)];
        if (!comment) return { ...old, commands };
        const conversation = old.conversation ?? [];
        return {
          ...old,
          commands,
          conversation: conversation.some((c) => c.id === comment.id)
            ? conversation
            : [...conversation, comment],
        };
      },
    );
  };
}

export function useAddProductIssueComment(
  workItemId: string,
): UseMutationResult<ProductAddCommentResponse, Error, { body: string; idempotencyKey?: string }> {
  const api = useBackendApi();
  const refresh = useRefreshAfterAction(workItemId);
  const merge = useMergeIntoDetail(workItemId);
  return useMutation({
    mutationFn: ({ body, idempotencyKey }) =>
      api.post<{ body: string }, ProductAddCommentResponse>(
        path(workItemId, "/comments"),
        { body },
        keyHeaders(idempotencyKey),
      ),
    onSuccess: (res) => {
      merge(res.comment, res.command);
      refresh();
    },
    // A 409 means the server state moved on; show it and pick up the new state.
    onError: refresh,
  });
}

export function useMoveProductIssueToProductTeam(
  workItemId: string,
): UseMutationResult<ProductCommandResponse, Error, { idempotencyKey?: string }> {
  const api = useBackendApi();
  const refresh = useRefreshAfterAction(workItemId);
  const merge = useMergeIntoDetail(workItemId);
  return useMutation({
    mutationFn: ({ idempotencyKey }) =>
      api.post<{ status: string }, ProductCommandResponse>(
        path(workItemId, "/board-status"),
        { status: "WAITING_ON_PRODUCT_TEAM" },
        keyHeaders(idempotencyKey),
      ),
    onSuccess: (res) => {
      merge(null, res.command);
      refresh();
    },
    onError: refresh,
  });
}

export function useCloseProductIssue(
  workItemId: string,
): UseMutationResult<ProductCommandResponse, Error, { idempotencyKey?: string }> {
  const api = useBackendApi();
  const refresh = useRefreshAfterAction(workItemId);
  const merge = useMergeIntoDetail(workItemId);
  return useMutation({
    mutationFn: ({ idempotencyKey }) =>
      api.post<Record<string, never>, ProductCommandResponse>(
        path(workItemId, "/close"),
        {},
        keyHeaders(idempotencyKey),
      ),
    onSuccess: (res) => {
      merge(null, res.command);
      refresh();
    },
    onError: refresh,
  });
}

export function useRetryProductIssueCommand(
  workItemId: string,
): UseMutationResult<ProductCommandResponse, Error, { commandId: string }> {
  const api = useBackendApi();
  const refresh = useRefreshAfterAction(workItemId);
  const merge = useMergeIntoDetail(workItemId);
  return useMutation({
    mutationFn: ({ commandId }) =>
      api.post<Record<string, never>, ProductCommandResponse>(
        path(workItemId, `/commands/${encodeURIComponent(commandId)}/retry`),
        {},
      ),
    onSuccess: (res) => {
      merge(null, res.command);
      refresh();
    },
    onError: refresh,
  });
}
