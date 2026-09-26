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
  type QueryKey,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeComment,
  BeCaseCommentCreatePayload,
  BeCaseCommentSearchPayload,
  BeCommentPatchPayload,
  BeCommentSearchResponse,
} from "@api/backend/types";
import {
  commentTypeFromInternal,
  uiCommentFromBe,
} from "@api/backend/mappers";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

/** Page size per request. Capped by the BE; see BE_MAX_PAGE_LIMIT. */
const COMMENTS_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;
/** Safety bound on how many pages a single case's comment trail can page
 * through — 200 pages * BE_MAX_PAGE_LIMIT is far beyond any real case, this
 * only guards against an unbounded loop if the BE's `hasMore` were ever
 * wrong. */
const MAX_COMMENT_PAGES = 200;

/**
 * Load *every* comment on a case. In LIVE mode calls
 * `POST /cases/{id}/comments/search`, paging through `BE_MAX_PAGE_LIMIT`-sized
 * requests until the BE reports no more (`hasMore`) — a case with more
 * comments than one page used to silently drop everything past the first
 * BE_MAX_PAGE_LIMIT (reported live: printing/exporting a heavily-commented
 * case was missing most of its comment trail, independent of and in addition
 * to the print-CSS pagination fix in `print.css`). This is the comments
 * *lane* specifically (work notes/public comments) — the audit/field-change
 * lane (`useGetCsmCaseActivities`) needed and got the identical fix.
 */
export function useGetCsmCaseComments(
  caseId: string | undefined,
): UseQueryResult<CsmCaseComment[], Error> {
  const api = useBackendApi();

  return useQuery<CsmCaseComment[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, caseId ?? ""],
    queryFn: async (): Promise<CsmCaseComment[]> => {
      if (!caseId) return [];

      const allComments: BeComment[] = [];
      for (let page = 0; page < MAX_COMMENT_PAGES; page += 1) {
        const payload: BeCaseCommentSearchPayload = {
          pagination: { offset: page * COMMENTS_PAGE_LIMIT, limit: COMMENTS_PAGE_LIMIT },
        };
        const response = await api.post<
          BeCaseCommentSearchPayload,
          BeCommentSearchResponse
        >(`/cases/${encodeURIComponent(caseId)}/comments/search`, payload);
        const rows = response.comments ?? [];
        allComments.push(...rows);
        if (rows.length < COMMENTS_PAGE_LIMIT || !response.hasMore) break;
      }
      return allComments.map((comment) =>
        uiCommentFromBe(comment, { context: "case" }),
      );
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}

export interface PostCsmCaseCommentInput {
  caseId: string;
  /** Plain-text body. HTML is escaped on render. */
  bodyHtml: string;
  /** Display name of the logged-in engineer. */
  authorName: string;
  /** If true, the entry is an internal work note (not customer-visible). */
  internal?: boolean;
}

/**
 * Create a comment on a case. The backend accepts a plain-text body; the FE
 * still passes a `bodyHtml` field for parity with the mock, but it's stored
 * as plain text and re-rendered on read.
 */
export function usePostCsmCaseComment(): UseMutationResult<
  CsmCaseComment,
  Error,
  PostCsmCaseCommentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CsmCaseComment, Error, PostCsmCaseCommentInput>({
    mutationFn: async (input): Promise<CsmCaseComment> => {
      const payload: BeCaseCommentCreatePayload = {
        type: commentTypeFromInternal(input.internal ?? false),
        // BE stores rich-text HTML; send the editor output as-is.
        content: input.bodyHtml,
      };
      const created = await api.post<
        BeCaseCommentCreatePayload,
        BeComment
      >(`/cases/${encodeURIComponent(input.caseId)}/comments`, payload);
      return uiCommentFromBe(created, { context: "case" });
    },
    onSuccess: (_newComment, variables) => {
      // The BE create response is a thin ack — it echoes only {id, createdOn,
      // createdBy(email string)}, not the author's display name, the rendered
      // content, or the comment type. Appending that partial object renders the
      // new entry as "Unknown WSO2 —" with no body. Until the BE echoes the full
      // comment on POST, refetch the list so the new comment is hydrated from
      // `comments/search`, which returns the full shape. One extra round-trip
      // per post, but the entry renders correctly.
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, variables.caseId],
      });
    },
  });
}

// ---------------------------------------------------------------------------
// Edit / soft-delete — generic across case/change-request/incident comments
// ---------------------------------------------------------------------------
//
// `PATCH /comments/{id}` and `DELETE /comments/{id}` are keyed purely by the
// comment's own id, not by which aggregate (case/change request/incident) it
// belongs to — one route pair serves all three. These two hooks live here
// (alongside the case-comment query/create hooks, and the `CsmCaseComment`
// type/`uiCommentFromBe` mapper they already share) rather than being
// duplicated per aggregate; `useCsmChangeRequestComments.ts`/
// `useCsmIncidentComments.ts` import them directly instead of redefining
// their own. Each caller supplies its own `invalidateQueryKey` (that
// aggregate's own comments list query key) since only the caller knows which
// list the edited/deleted comment belongs to.

export interface PatchCommentInput {
  commentId: string;
  /** New plain-text/rich-text body — same shape `usePostCsmCaseComment`
   * already sends on create. */
  content: string;
  /** Query key to invalidate on success — that aggregate's own comments list
   * (e.g. `[ApiQueryKeys.CSM_CASE_COMMENTS, caseId]`). */
  invalidateQueryKey: QueryKey;
}

/** Edit a comment's content via `PATCH /comments/{id}`. Server-side: only the
 * comment's own author or a caller holding the `admin` role may do this; a
 * prior edit's body is preserved server-side for audit. */
export function usePatchComment(): UseMutationResult<
  CsmCaseComment,
  Error,
  PatchCommentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CsmCaseComment, Error, PatchCommentInput>({
    mutationFn: async (input): Promise<CsmCaseComment> => {
      const payload: BeCommentPatchPayload = { content: input.content };
      // PATCH /comments/{id} returns { message, comment }, not a bare
      // comment — the BFF forwards the entity service's response unchanged.
      const updated = await api.patch<
        BeCommentPatchPayload,
        { message: string; comment: BeComment }
      >(`/comments/${encodeURIComponent(input.commentId)}`, payload);
      return uiCommentFromBe(updated.comment, { context: "case" });
    },
    onSuccess: (_updated, variables) => {
      void queryClient.invalidateQueries({
        queryKey: variables.invalidateQueryKey,
      });
    },
  });
}

export interface DeleteCommentInput {
  commentId: string;
  /** Query key to invalidate on success — see {@link PatchCommentInput}. */
  invalidateQueryKey: QueryKey;
}

/** Soft-delete a comment via `DELETE /comments/{id}` (204 on success; the
 * real content is never destroyed server-side). Server-side: only the
 * comment's own author or a caller holding the `admin` role may do this. */
export function useDeleteComment(): UseMutationResult<
  void,
  Error,
  DeleteCommentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<void, Error, DeleteCommentInput>({
    mutationFn: async (input): Promise<void> => {
      await api.del<null>(`/comments/${encodeURIComponent(input.commentId)}`);
    },
    onSuccess: (_void, variables) => {
      void queryClient.invalidateQueries({
        queryKey: variables.invalidateQueryKey,
      });
    },
  });
}
