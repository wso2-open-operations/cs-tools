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
import { useNavigate, useSearchParams } from "react-router-dom";
import { Badge, Chip, Fab, IconButton, Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { Plus, SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useQuery, useQueryErrorResetBoundary, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { currentUser } from "@src/services/currentUser";
import type { CaseState } from "@src/types";
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
import {
  FILTERABLE_STATES,
  SEVERITY_LABELS,
  STATE_LABELS,
  TAB_CONFIG,
  WORK_STATE_LABEL,
} from "@components/support/config";
import { useDebouncedValue } from "@utils/useDebouncedValue";

const ALL_STATES_TAB = "all" as const;
type StateTabValue = CaseState | typeof ALL_STATES_TAB;

// Cases only (mirrors AllCasesPage's former "View All" list, now the Support page itself): a
// single infinite-scrolled, most-recently-updated-first list with search/filters/create inline,
// no separate recent-preview + "View All" hop. search/filters are seeded once from the URL on
// mount — lets Home's composition donuts deep-link here with a filter pre-applied — then managed
// as plain local state from there (not kept in sync back to the URL).
export default function SupportPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [search, setSearch] = useState(() => filtersFromSearchParams(searchParams).search);
  const debouncedSearch = useDebouncedValue(search, 300);
  const [filters, setFilters] = useState<CaseFilters>(() => filtersFromSearchParams(searchParams).filters);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const activeFilterCount = countActiveFilters(filters);

  const { data: currentUserId } = useQuery(currentUser.id());

  // Single-select state tab. "All" clears the states filter entirely; picking a state resets
  // work state unless it's "Work in progress" (the only state work state is meaningful for),
  // same invariant the webapp's filter bar enforces.
  const stateTab: StateTabValue = filters.states[0] ?? ALL_STATES_TAB;
  const handleStateTabChange = (value: StateTabValue) => {
    const states = value === ALL_STATES_TAB ? [] : [value];
    const workStates = value === "work_in_progress" ? filters.workStates : [];
    setFilters({ ...filters, states, workStates });
  };

  return (
    <Stack gap={2}>
      <Stack direction="row" gap={1} alignItems="center">
        <SearchBar value={search} onChange={setSearch} />
        <Badge badgeContent={activeFilterCount} color="primary" invisible={activeFilterCount === 0}>
          <IconButton aria-label="Filters" onClick={() => setFiltersOpen(true)}>
            <SlidersHorizontal size={18} />
          </IconButton>
        </Badge>
      </Stack>

      {activeFilterCount > 0 && <ActiveFiltersRow filters={filters} onChange={setFilters} />}

      <Tabs variant="scrollable" value={stateTab} onChange={(_, value: StateTabValue) => handleStateTabChange(value)}>
        <Tab label="All" value={ALL_STATES_TAB} disableRipple />
        {FILTERABLE_STATES.map((state) => (
          <Tab key={state} label={STATE_LABELS[state]} value={state} disableRipple />
        ))}
      </Tabs>

      <CaseListErrorBoundary>
        <Suspense fallback={<CaseListSkeleton />}>
          <CaseListContent search={debouncedSearch} filters={filters} currentUserId={currentUserId ?? null} />
        </Suspense>
      </CaseListErrorBoundary>

      <FiltersSheet open={filtersOpen} onClose={() => setFiltersOpen(false)} filters={filters} onApply={setFilters} />

      {/* Floating rather than inline in the header — mirrors the customer-portal microapp's own
          Fab (components/core/Fab.tsx): fixed bottom-right, offset above the fixed TabBar by its
          live-measured height (--tab-bar-height) plus some breathing room. */}
      <Fab
        aria-label="Create case"
        size="medium"
        color="primary"
        onClick={() => navigate("/cases/new")}
        sx={{ position: "fixed", right: 10, bottom: "calc(var(--tab-bar-height) + 60px)" }}
      >
        <Plus size={20} />
      </Fab>
    </Stack>
  );
}

// One removable chip per active filter (severities, work states, assigned/created-by-me) — lets
// a filter picked up via the FiltersSheet, or deep-linked in from Home's composition donuts, be
// removed individually instead of only via the sheet's "Clear all". Deliberately excludes state
// (it has its own dedicated Tab row above, so a chip here would just duplicate that) — same
// exclusion countActiveFilters already makes for its badge count.
function ActiveFiltersRow({ filters, onChange }: { filters: CaseFilters; onChange: (filters: CaseFilters) => void }) {
  return (
    <Stack direction="row" gap={1} flexWrap="wrap">
      {filters.severities.map((severity) => (
        <Chip
          key={`severity-${severity}`}
          label={SEVERITY_LABELS[severity]}
          size="small"
          onDelete={() => onChange({ ...filters, severities: filters.severities.filter((s) => s !== severity) })}
        />
      ))}
      {filters.workStates.map((workState) => (
        <Chip
          key={`work-state-${workState}`}
          label={WORK_STATE_LABEL[workState]}
          size="small"
          onDelete={() => onChange({ ...filters, workStates: filters.workStates.filter((w) => w !== workState) })}
        />
      ))}
      {filters.assignedToMe && (
        <Chip label="Assigned to me" size="small" onDelete={() => onChange({ ...filters, assignedToMe: false })} />
      )}
      {filters.createdByMe && (
        <Chip label="Created by me" size="small" onDelete={() => onChange({ ...filters, createdByMe: false })} />
      )}
    </Stack>
  );
}

function CaseListContent({
  search,
  filters,
  currentUserId,
}: {
  search: string;
  filters: CaseFilters;
  currentUserId: string | null;
}) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useSuspenseInfiniteQuery(
    cases.infinite(toCaseSearchFilters("case", search, filters, currentUserId)),
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

  if (items.length === 0) return <EmptyState message={TAB_CONFIG.case.emptyMessage} />;

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

function CaseListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 6 }).map((_, index) => (
        <CaseCardSkeleton key={index} />
      ))}
    </Stack>
  );
}

function CaseListErrorBoundary({ children }: { children: ReactNode }) {
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
