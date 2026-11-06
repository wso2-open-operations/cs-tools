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
  useInfiniteQuery,
  type InfiniteData,
  type UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { CaseCommentsResponse } from "@features/support/types/cases";
import {
  CASE_COMMENTS_PAGE_SIZE,
  countLoadedComments,
  dedupeCommentPages,
  fetchCaseCommentsPage,
} from "@features/support/api/caseCommentsRefresh";

/**
 * Infinite query for case comments, loading 10 at a time.
 * Call `fetchNextPage()` to load older comments.
 */
export default function useGetCaseCommentsInfinite(
  projectId: string,
  caseId: string,
): UseInfiniteQueryResult<InfiniteData<CaseCommentsResponse>, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<CaseCommentsResponse, Error, InfiniteData<CaseCommentsResponse>>({
    queryKey: [ApiQueryKeys.CASE_COMMENTS, projectId, caseId, "infinite"],
    initialPageParam: 0,
    // The next page starts after the comments already loaded, not at
    // lastPage.offset + limit: an incremental refresh (see
    // refreshCaseComments) puts new comments in front, which shifts every
    // older comment's offset.
    getNextPageParam: (lastPage, allPages) => {
      const loaded = countLoadedComments(allPages);
      return loaded < lastPage.totalRecords ? loaded : undefined;
    },
    select: dedupeCommentPages,
    queryFn: async ({ pageParam }): Promise<CaseCommentsResponse> => {
      const offset = pageParam as number;
      logger.debug(
        `Fetching case comments (infinite): projectId=${projectId}, caseId=${caseId}, offset=${offset}`,
      );
      return fetchCaseCommentsPage(authFetch, caseId, offset, CASE_COMMENTS_PAGE_SIZE);
    },
    enabled: !!projectId && !!caseId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
