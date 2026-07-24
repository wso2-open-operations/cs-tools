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

import { useEffect, useRef, useState } from "react";
import { Badge, Box, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { announcements } from "@src/services/announcements";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import {
  countActiveAnnouncementFilters,
  EMPTY_ANNOUNCEMENT_FILTERS,
  type AnnouncementFilters,
} from "@utils/announcements";
import { SearchBar } from "@components/support/SearchBar";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { AnnouncementCard, AnnouncementCardSkeleton } from "@components/announcements/AnnouncementCard";
import { AnnouncementFiltersSheet } from "@components/announcements/AnnouncementFiltersSheet";

// Read-only announcements list (announcements are cases of type "announcement").
// Search + State + Project filters, infinite-scrolled newest-updated first.
export default function AnnouncementsPage() {
  const [filters, setFilters] = useState<AnnouncementFilters>(EMPTY_ANNOUNCEMENT_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);
  const activeFilterCount = countActiveAnnouncementFilters(filters);

  // Search is debounced into the query so typing doesn't refetch every keystroke.
  const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    announcements.infinite({ ...filters, search: debouncedSearch }),
  );

  const items = data?.pages.flatMap((p) => p.items) ?? [];
  const total = data?.pages[0]?.total ?? items.length;

  const sentinelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting && hasNextPage && !isFetchingNextPage) {
          void fetchNextPage();
        }
      },
      { threshold: 0.1 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  return (
    <Stack gap={2}>
      <Box>
        <Typography variant="h5">Announcements</Typography>
        <Typography variant="body2" color="text.secondary">
          Customer-facing announcements published across projects.
        </Typography>
      </Box>

      <Stack direction="row" gap={1} alignItems="center">
        <SearchBar
          value={filters.search}
          onChange={(value) => setFilters((prev) => ({ ...prev, search: value }))}
          placeholder="Search by number or subject…"
        />
        <Badge badgeContent={activeFilterCount} color="primary" invisible={activeFilterCount === 0}>
          <IconButton aria-label="Filters" onClick={() => setFiltersOpen(true)}>
            <SlidersHorizontal size={18} />
          </IconButton>
        </Badge>
      </Stack>

      {isLoading ? (
        <ListSkeleton />
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : items.length === 0 ? (
        <EmptyState message="No announcements found." />
      ) : (
        <Stack gap={1.5}>
          <Typography variant="caption" color="text.secondary">
            {items.length} of {total}
          </Typography>

          {items.map((item) => (
            <AnnouncementCard key={item.id} item={item} />
          ))}

          {/* IntersectionObserver can miss a zero-height target, so give the sentinel 1px to observe. */}
          <div ref={sentinelRef} style={{ height: 1 }} />

          {isFetchingNextPage && <AnnouncementCardSkeleton />}
          {!hasNextPage && (
            <Typography variant="body2" color="text.secondary" textAlign="center" py={1}>
              You're all caught up!
            </Typography>
          )}
        </Stack>
      )}

      <AnnouncementFiltersSheet
        open={filtersOpen}
        onClose={() => setFiltersOpen(false)}
        filters={filters}
        onApply={setFilters}
      />
    </Stack>
  );
}

function ListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 5 }).map((_, i) => (
        <AnnouncementCardSkeleton key={i} />
      ))}
    </Stack>
  );
}
