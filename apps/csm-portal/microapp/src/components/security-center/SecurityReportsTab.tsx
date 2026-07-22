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
import { Badge, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { securityReports } from "@src/services/securityReports";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { useInfiniteScrollSentinel } from "@utils/useInfiniteScrollSentinel";
import {
  countActiveSecurityReportFilters,
  EMPTY_SECURITY_REPORT_FILTERS,
  type SecurityReportFilters,
} from "@utils/securityReports";
import { SearchBar } from "@components/support/SearchBar";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";
import { SecurityReportFiltersSheet } from "@components/security-center/SecurityReportFiltersSheet";

// Cross-customer security reports list (security reports are cases of type
// "security_report_analysis"), mirroring the webapp's CsmSecurityCenterPage's
// Security reports tab — a CsmIssuesView locked to
// `caseTypes: ["security_report_analysis"]` with severity hidden. Search +
// State + Work state + Assignee + Project + Product filters, infinite-scrolled
// newest-updated first. Read-only for this pass — "New security report" is
// deliberately deferred, same call Engagements/Operations already made.
export function SecurityReportsTab() {
  const [filters, setFilters] = useState<SecurityReportFilters>(EMPTY_SECURITY_REPORT_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);
  const activeFilterCount = countActiveSecurityReportFilters(filters);

  // Search is debounced into the query so typing doesn't refetch every keystroke.
  const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    securityReports.infinite({ ...filters, search: debouncedSearch }),
  );

  const items = data?.pages.flatMap((p) => p.items) ?? [];
  const total = data?.pages[0]?.total ?? items.length;

  const sentinelRef = useInfiniteScrollSentinel({ hasNextPage, isFetchingNextPage, fetchNextPage });

  return (
    <Stack gap={2}>
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
        <EmptyState message="No security reports found." />
      ) : (
        <Stack gap={1.5}>
          <Typography variant="caption" color="text.secondary">
            {items.length} of {total}
          </Typography>

          {items.map((item) => (
            <CaseCard key={item.id} item={item} />
          ))}

          {/* IntersectionObserver can miss a zero-height target, so give the sentinel 1px to observe. */}
          <div ref={sentinelRef} style={{ height: 1 }} />

          {isFetchingNextPage && <CaseCardSkeleton />}
          {!hasNextPage && (
            <Typography variant="body2" color="text.secondary" textAlign="center" py={1}>
              You're all caught up!
            </Typography>
          )}
        </Stack>
      )}

      <SecurityReportFiltersSheet
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
        <CaseCardSkeleton key={i} />
      ))}
    </Stack>
  );
}
