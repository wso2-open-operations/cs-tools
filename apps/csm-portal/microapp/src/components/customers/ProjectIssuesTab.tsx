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

import { useState } from "react";
import { Stack, Typography } from "@wso2/oxygen-ui";
import { useInfiniteQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { useInfiniteScrollSentinel } from "@utils/useInfiniteScrollSentinel";
import { SearchBar } from "@components/support/SearchBar";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";

// A project's issues across every case type (support cases, service requests, engagements,
// security reports, announcements) — the mobile equivalent of the webapp's CsmIssuesView locked
// to `projects: [id]`. Search-only for this pass; no filter sheet (the webapp's full
// severity/state/assignee/product filter set isn't warranted for a single project's issue list,
// same reasoning Announcements/Engagements started from before Engagements later grew filters).
export function ProjectIssuesTab({ projectId }: { projectId: string }) {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);

  const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    cases.infinite({
      projectIds: [projectId],
      ...(debouncedSearch.trim() && { searchQuery: debouncedSearch.trim() }),
    }),
  );

  const items = data?.pages.flatMap((p) => p.items) ?? [];

  const sentinelRef = useInfiniteScrollSentinel({ hasNextPage, isFetchingNextPage, fetchNextPage });

  return (
    <Stack gap={2}>
      <SearchBar value={search} onChange={setSearch} placeholder="Search by number, subject…" />

      {isLoading ? (
        <Stack gap={1.5}>
          {[0, 1, 2].map((i) => (
            <CaseCardSkeleton key={i} />
          ))}
        </Stack>
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : items.length === 0 ? (
        <EmptyState message="No issues for this project." />
      ) : (
        <Stack gap={1.5}>
          {items.map((item) => (
            <CaseCard key={item.id} item={item} />
          ))}

          <div ref={sentinelRef} style={{ height: 1 }} />

          {isFetchingNextPage && <CaseCardSkeleton />}
          {!hasNextPage && (
            <Typography variant="body2" color="text.secondary" textAlign="center" py={1}>
              You're all caught up!
            </Typography>
          )}
        </Stack>
      )}
    </Stack>
  );
}
