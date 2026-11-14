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

import type { InfiniteData, QueryClient, QueryKey } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ILogger } from "@hooks/logger";
import type {
  CaseActivitiesResponse,
  CaseComment,
  CaseCommentsResponse,
} from "@features/support/types/cases";

type AuthFetch = (input: RequestInfo | URL, options?: RequestInit) => Promise<Response>;

/** Page size used by the case comments infinite query and by incremental refreshes. */
export const CASE_COMMENTS_PAGE_SIZE = 10;
/**
 * An incremental refresh walks newest-first pages until it meets a comment the
 * cache already has. If it has not met one after this many pages it gives up
 * and invalidates the query instead, so the cache is never silently wrong.
 */
export const MAX_INCREMENTAL_REFRESH_PAGES = 5;

/** Maps a raw activity entry to the comment shape the UI renders. */
export function mapActivityToComment(activity: CaseActivitiesResponse["activities"][number]): CaseComment {
  return {
    id: activity.id,
    content: activity.content,
    createdOn: activity.createdOn,
    createdBy: activity.createdBy,
    createdByFirstName: activity.createdByFirstName,
    createdByLastName: activity.createdByLastName,
    createdByFullName: activity.createdByFullName,
    createdByIsInternal: activity.createdByIsInternal,
    type: activity.type,
    fileName: activity.fileName,
    contentType: activity.contentType,
    sizeBytes: activity.sizeBytes,
    downloadUrl: activity.downloadUrl,
    isEscalated: false,
  };
}

/**
 * Fetches one page of a case's activity feed (POST /cases/:caseId/activities/search),
 * newest first, mapped to comments.
 */
export async function fetchCaseCommentsPage(
  authFetch: AuthFetch,
  caseId: string,
  offset: number,
  limit: number = CASE_COMMENTS_PAGE_SIZE,
): Promise<CaseCommentsResponse> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const response = await authFetch(`${baseUrl}/cases/${caseId}/activities/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pagination: { offset, limit } }),
  });

  if (!response.ok) {
    throw new Error(`Error fetching case comments: ${response.status} ${response.statusText}`);
  }

  const data: CaseActivitiesResponse = await response.json();
  return {
    comments: (data.activities ?? []).map(mapActivityToComment),
    totalRecords: data.totalRecords,
    offset: data.offset,
    limit: data.limit,
  };
}

/** Distinct comment ids across all loaded pages. */
function collectIds(pages: CaseCommentsResponse[]): Set<string> {
  const ids = new Set<string>();
  for (const page of pages) {
    for (const comment of page.comments) ids.add(comment.id);
  }
  return ids;
}

/** Number of distinct comments loaded so far; the offset of the next older page. */
export function countLoadedComments(pages: CaseCommentsResponse[]): number {
  return collectIds(pages).size;
}

/**
 * Drops comments that appear on more than one page. Offsets shift when new
 * comments arrive, so an older page can overlap one already loaded.
 */
export function dedupeCommentPages(
  data: InfiniteData<CaseCommentsResponse>,
): InfiniteData<CaseCommentsResponse> {
  const seen = new Set<string>();
  let changed = false;
  const pages = data.pages.map((page) => {
    const comments = page.comments.filter((comment) => {
      if (seen.has(comment.id)) return false;
      seen.add(comment.id);
      return true;
    });
    if (comments.length === page.comments.length) return page;
    changed = true;
    return { ...page, comments };
  });
  return changed ? { ...data, pages } : data;
}

/** Cached infinite comments queries for a case (the key also carries a project id). */
function findCachedQueries(
  queryClient: QueryClient,
  caseId: string,
): QueryKey[] {
  return queryClient
    .getQueriesData<InfiniteData<CaseCommentsResponse>>({
      queryKey: [ApiQueryKeys.CASE_COMMENTS],
      predicate: (query) =>
        query.queryKey[2] === caseId &&
        query.queryKey[3] === "infinite" &&
        ((query.state.data as InfiniteData<CaseCommentsResponse> | undefined)?.pages.length ?? 0) > 0,
    })
    .map(([key]) => key);
}

async function refreshOnce(
  queryClient: QueryClient,
  authFetch: AuthFetch,
  caseId: string,
  logger?: Pick<ILogger, "debug">,
): Promise<void> {
  const keys = findCachedQueries(queryClient, caseId);
  if (keys.length === 0) return;
  const queryKey = keys[0];

  try {
    const known = collectIds(
      queryClient.getQueryData<InfiniteData<CaseCommentsResponse>>(queryKey)?.pages ?? [],
    );
    const fresh: CaseComment[] = [];
    let totalRecords: number | undefined;
    let reachedKnown = false;
    let reachedEnd = false;

    for (let page = 0; page < MAX_INCREMENTAL_REFRESH_PAGES; page += 1) {
      const offset = page * CASE_COMMENTS_PAGE_SIZE;
      const result = await fetchCaseCommentsPage(authFetch, caseId, offset);
      totalRecords ??= result.totalRecords;
      for (const comment of result.comments) {
        if (known.has(comment.id)) {
          reachedKnown = true;
        } else {
          fresh.push(comment);
        }
      }
      if (reachedKnown) break;
      if (
        result.comments.length < CASE_COMMENTS_PAGE_SIZE ||
        offset + CASE_COMMENTS_PAGE_SIZE >= result.totalRecords
      ) {
        reachedEnd = true;
        break;
      }
    }

    if (!reachedKnown && !reachedEnd) {
      // Too many new comments to splice in safely: reload the query.
      void queryClient.invalidateQueries({ queryKey });
      return;
    }

    const latestTotal = totalRecords ?? 0;
    queryClient.setQueryData<InfiniteData<CaseCommentsResponse>>(queryKey, (old) => {
      if (!old || old.pages.length === 0) return old;
      // Re-read ids now: the cache may have changed while requests were in flight.
      const current = collectIds(old.pages);
      const additions = fresh.filter((comment) => !current.has(comment.id));
      const [first, ...rest] = old.pages;
      const pages = [
        { ...first, comments: [...additions, ...first.comments], totalRecords: latestTotal },
        ...rest.map((page) => ({ ...page, totalRecords: latestTotal })),
      ];
      return { ...old, pages };
    });
  } catch (error) {
    logger?.debug(
      "[case-comments-refresh] incremental refresh failed",
      error instanceof Error ? error.message : "Unknown error",
    );
  }
}

interface InFlight {
  promise: Promise<void>;
  again: boolean;
}
const inFlightByClient = new WeakMap<QueryClient, Map<string, InFlight>>();

/**
 * Brings a case's cached comments up to date without refetching every loaded
 * page: fetches from the newest page and stops at the first page that contains
 * a comment already in the cache (usually one request for one new comment),
 * then splices the new comments in front. Does nothing if the case's comments
 * are not cached (they load normally when the panel mounts). Never rejects.
 *
 * Concurrent calls for one case are coalesced: at most one run is active and
 * one more is queued, so a comment that lands mid-run is still picked up.
 */
export function refreshCaseComments(
  queryClient: QueryClient,
  authFetch: AuthFetch,
  caseId: string,
  logger?: Pick<ILogger, "debug">,
): Promise<void> {
  let byCase = inFlightByClient.get(queryClient);
  if (!byCase) {
    byCase = new Map();
    inFlightByClient.set(queryClient, byCase);
  }
  const existing = byCase.get(caseId);
  if (existing) {
    existing.again = true;
    return existing.promise;
  }

  const entry: InFlight = { promise: Promise.resolve(), again: false };
  const registry = byCase;
  entry.promise = (async () => {
    try {
      do {
        entry.again = false;
        await refreshOnce(queryClient, authFetch, caseId, logger);
      } while (entry.again);
    } finally {
      registry.delete(caseId);
    }
  })();
  registry.set(caseId, entry);
  return entry.promise;
}
