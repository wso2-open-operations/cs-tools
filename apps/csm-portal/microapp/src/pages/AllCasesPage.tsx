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
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { Badge, IconButton, Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { Plus, SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useQuery, useQueryErrorResetBoundary, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { currentUser } from "@src/services/currentUser";
import type { CaseState, CaseType } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { SearchBar } from "@components/support/SearchBar";
import { FiltersSheet } from "@components/support/FiltersSheet";
import {
  countActiveFilters,
  filtersFromSearchParams,
  toCaseSearchFilters,
  type CaseFilters,
} from "@components/support/filters";
import { FILTERABLE_STATES, STATE_LABELS, TAB_CONFIG } from "@components/support/config";
import { useDebouncedValue } from "@utils/useDebouncedValue";

const ALL_STATES_TAB = "all" as const;
type StateTabValue = CaseState | typeof ALL_STATES_TAB;

// The full, infinite-scrolled list behind a tab's "View All" link (mirrors the customer-portal
// microapp's AllItemsPage.tsx). Search/filters are seeded once from the URL the recent-5 view
// handed off, then managed locally here so the user can keep refining without round-tripping
// through the recent view.
export default function AllCasesPage() {
  const { type } = useParams<{ type: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const caseType = (TAB_CONFIG[type as CaseType] ? type : "case") as CaseType;

  const initial = filtersFromSearchParams(searchParams);
  const [search, setSearch] = useState(initial.search);
  const debouncedSearch = useDebouncedValue(search, 300);
  const [filters, setFilters] = useState<CaseFilters>(initial.filters);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const activeFilterCount = countActiveFilters(filters);

  const { data: currentUserId } = useQuery(currentUser.id());

  // Single-select state tab, mirroring the type-tab pattern on the Support page. "All" clears the
  // states filter entirely; picking a state resets work state unless it's "Work in progress" (the
  // only state work state is meaningful for), same invariant the webapp's filter bar enforces.
  const stateTab: StateTabValue = filters.states[0] ?? ALL_STATES_TAB;
  const handleStateTabChange = (value: StateTabValue) => {
    const states = value === ALL_STATES_TAB ? [] : [value];
    const workStates = value === "work_in_progress" ? filters.workStates : [];
    setFilters({ ...filters, states, workStates });
  };

  return (
    <Stack gap={2}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Typography variant="h6">{TAB_CONFIG[caseType].title}</Typography>
        <IconButton
          aria-label="Create case"
          onClick={() => navigate("/cases/new")}
          sx={{
            bgcolor: "primary.main",
            color: "primary.contrastText",
            "&:hover": { bgcolor: "primary.dark" },
          }}
        >
          <Plus size={20} />
        </IconButton>
      </Stack>

      <Stack direction="row" gap={1} alignItems="center">
        <SearchBar value={search} onChange={setSearch} />
        <Badge badgeContent={activeFilterCount} color="primary" invisible={activeFilterCount === 0}>
          <IconButton aria-label="Filters" onClick={() => setFiltersOpen(true)}>
            <SlidersHorizontal size={18} />
          </IconButton>
        </Badge>
      </Stack>

      <Tabs variant="scrollable" value={stateTab} onChange={(_, value: StateTabValue) => handleStateTabChange(value)}>
        <Tab label="All" value={ALL_STATES_TAB} disableRipple />
        {FILTERABLE_STATES.map((state) => (
          <Tab key={state} label={STATE_LABELS[state]} value={state} disableRipple />
        ))}
      </Tabs>

      <AllCasesErrorBoundary>
        <Suspense fallback={<AllCasesSkeleton />}>
          <AllCasesListContent
            type={caseType}
            search={debouncedSearch}
            filters={filters}
            currentUserId={currentUserId ?? null}
          />
        </Suspense>
      </AllCasesErrorBoundary>

      <FiltersSheet open={filtersOpen} onClose={() => setFiltersOpen(false)} filters={filters} onApply={setFilters} />
    </Stack>
  );
}

function AllCasesListContent({
  type,
  search,
  filters,
  currentUserId,
}: {
  type: CaseType;
  search: string;
  filters: CaseFilters;
  currentUserId: string | null;
}) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useSuspenseInfiniteQuery(
    cases.infinite(toCaseSearchFilters(type, search, filters, currentUserId)),
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
  const total = data.pages[0]?.total ?? items.length;

  if (items.length === 0) return <EmptyState message={TAB_CONFIG[type].emptyMessage} />;

  return (
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
        <Typography variant="body2" color="text.secondary" textAlign="center" py={2}>
          You're all caught up!
        </Typography>
      )}
    </Stack>
  );
}

function AllCasesSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 6 }).map((_, index) => (
        <CaseCardSkeleton key={index} />
      ))}
    </Stack>
  );
}

function AllCasesErrorBoundary({ children }: { children: ReactNode }) {
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
