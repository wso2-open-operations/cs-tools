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

import { Suspense, useEffect, useRef, useState, type ReactNode } from "react";
import { Badge, Chip, IconButton, Stack } from "@wso2/oxygen-ui";
import { SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useQueryErrorResetBoundary, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { changeRequests } from "@src/services/changeRequests";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { SearchBar } from "@components/support/SearchBar";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { formatDate } from "@utils/dateTime";
import { ChangeRequestCard, ChangeRequestCardSkeleton } from "./ChangeRequestCard";
import { ChangeRequestsFiltersSheet } from "./ChangeRequestsFiltersSheet";
import { CHANGE_REQUEST_IMPACT_LABELS, CHANGE_REQUEST_STATE_LABELS } from "./config";
import {
  countActiveCRFilters,
  EMPTY_CR_FILTERS,
  toChangeRequestSearchFilters,
  type ChangeRequestFilters,
} from "./changeRequestFilters";

// Mirrors the shape of SupportPage's own list content (search + filter sheet + infinite scroll),
// scoped to change requests instead of cases.
export function ChangeRequestsTab() {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);
  const [filters, setFilters] = useState<ChangeRequestFilters>(EMPTY_CR_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const activeFilterCount = countActiveCRFilters(filters);

  return (
    <Stack gap={2}>
      <Stack direction="row" gap={1} alignItems="center">
        <SearchBar value={search} onChange={setSearch} placeholder="Search by CR #, subject…" />
        <Badge badgeContent={activeFilterCount} color="primary" invisible={activeFilterCount === 0}>
          <IconButton aria-label="Filters" onClick={() => setFiltersOpen(true)}>
            <SlidersHorizontal size={18} />
          </IconButton>
        </Badge>
      </Stack>

      {activeFilterCount > 0 && <ActiveFiltersRow filters={filters} onChange={setFilters} />}

      <ChangeRequestListErrorBoundary>
        <Suspense fallback={<ChangeRequestListSkeleton />}>
          <ChangeRequestListContent search={debouncedSearch} filters={filters} />
        </Suspense>
      </ChangeRequestListErrorBoundary>

      <ChangeRequestsFiltersSheet
        open={filtersOpen}
        onClose={() => setFiltersOpen(false)}
        filters={filters}
        onApply={setFilters}
      />
    </Stack>
  );
}

// One removable chip per active filter — mirrors SupportPage's own ActiveFiltersRow, letting a
// filter picked in the sheet be removed individually instead of only via "Clear all".
function ActiveFiltersRow({
  filters,
  onChange,
}: {
  filters: ChangeRequestFilters;
  onChange: (filters: ChangeRequestFilters) => void;
}) {
  return (
    <Stack direction="row" gap={1} flexWrap="wrap">
      {filters.states.map((state) => (
        <Chip
          key={`state-${state}`}
          label={CHANGE_REQUEST_STATE_LABELS[state]}
          size="small"
          onDelete={() => onChange({ ...filters, states: filters.states.filter((s) => s !== state) })}
        />
      ))}
      {filters.impacts.map((impact) => (
        <Chip
          key={`impact-${impact}`}
          label={CHANGE_REQUEST_IMPACT_LABELS[impact]}
          size="small"
          onDelete={() => onChange({ ...filters, impacts: filters.impacts.filter((i) => i !== impact) })}
        />
      ))}
      {filters.closedStartDate && (
        <Chip
          label={`From ${formatDate(new Date(filters.closedStartDate))}`}
          size="small"
          onDelete={() => onChange({ ...filters, closedStartDate: "" })}
        />
      )}
      {filters.closedEndDate && (
        <Chip
          label={`To ${formatDate(new Date(filters.closedEndDate))}`}
          size="small"
          onDelete={() => onChange({ ...filters, closedEndDate: "" })}
        />
      )}
    </Stack>
  );
}

function ChangeRequestListContent({ search, filters }: { search: string; filters: ChangeRequestFilters }) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useSuspenseInfiniteQuery(
    changeRequests.infinite({ filters: toChangeRequestSearchFilters(search, filters) }),
  );

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

  const items = data.pages.flatMap((page) => page.items);

  if (items.length === 0) return <EmptyState message="No change requests found." />;

  return (
    <Stack gap={1.5}>
      {items.map((item) => (
        <ChangeRequestCard key={item.id} item={item} />
      ))}

      <div ref={sentinelRef} style={{ height: 1 }} />

      {isFetchingNextPage && <ChangeRequestCardSkeleton />}
    </Stack>
  );
}

function ChangeRequestListSkeleton() {
  return (
    <Stack gap={1.5}>
      {[0, 1, 2].map((i) => (
        <ChangeRequestCardSkeleton key={i} />
      ))}
    </Stack>
  );
}

function ChangeRequestListErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
