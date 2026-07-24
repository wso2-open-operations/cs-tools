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
import { Badge, Chip, IconButton, Stack, Tab, Tabs } from "@wso2/oxygen-ui";
import { SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useQuery, useQueryErrorResetBoundary, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { currentUser } from "@src/services/currentUser";
import type { CaseState } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { SearchBar } from "@components/support/SearchBar";
import { FILTERABLE_STATES, STATE_LABELS, TAB_CONFIG, WORK_STATE_LABEL } from "@components/support/config";
import { countActiveFilters, EMPTY_FILTERS, toCaseSearchFilters, type CaseFilters } from "@components/support/filters";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { ServiceRequestsFiltersSheet } from "./ServiceRequestsFiltersSheet";

const ALL_STATES_TAB = "all" as const;
type StateTabValue = CaseState | typeof ALL_STATES_TAB;

// Reuses the exact same list/pagination infra as SupportPage's case list, just scoped to
// type: "service_request". Severity is dropped (case-type-only — the webapp's own CsmIssuesView
// hides its severity filter the same way when locked to a non-"case" type); work state stays,
// same as the webapp.
export function ServiceRequestsTab() {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);
  const [filters, setFilters] = useState<CaseFilters>(EMPTY_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const activeFilterCount = countActiveFilters(filters);
  const { data: currentUserId } = useQuery(currentUser.id());

  const stateTab: StateTabValue = filters.states[0] ?? ALL_STATES_TAB;
  const handleStateTabChange = (value: StateTabValue) => {
    const states = value === ALL_STATES_TAB ? [] : [value];
    const workStates = value === "work_in_progress" ? filters.workStates : [];
    setFilters({ ...filters, states, workStates });
  };

  return (
    <Stack gap={2}>
      <Stack direction="row" gap={1} alignItems="center">
        <SearchBar value={search} onChange={setSearch} placeholder="Search by request #, subject…" />
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

      <ServiceRequestListErrorBoundary>
        <Suspense fallback={<ServiceRequestListSkeleton />}>
          <ServiceRequestListContent search={debouncedSearch} filters={filters} currentUserId={currentUserId ?? null} />
        </Suspense>
      </ServiceRequestListErrorBoundary>

      <ServiceRequestsFiltersSheet
        open={filtersOpen}
        onClose={() => setFiltersOpen(false)}
        filters={filters}
        onApply={setFilters}
      />
    </Stack>
  );
}

// Mirrors SupportPage's own ActiveFiltersRow, minus severity (not applicable to service requests).
function ActiveFiltersRow({ filters, onChange }: { filters: CaseFilters; onChange: (filters: CaseFilters) => void }) {
  return (
    <Stack direction="row" gap={1} flexWrap="wrap">
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

function ServiceRequestListContent({
  search,
  filters,
  currentUserId,
}: {
  search: string;
  filters: CaseFilters;
  currentUserId: string | null;
}) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useSuspenseInfiniteQuery(
    cases.infinite(toCaseSearchFilters("service_request", search, filters, currentUserId)),
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

  if (items.length === 0) return <EmptyState message={TAB_CONFIG.service_request.emptyMessage} />;

  return (
    <Stack gap={1.5}>
      {items.map((item) => (
        <CaseCard key={item.id} item={item} />
      ))}

      <div ref={sentinelRef} style={{ height: 1 }} />

      {isFetchingNextPage && <CaseCardSkeleton />}
    </Stack>
  );
}

function ServiceRequestListSkeleton() {
  return (
    <Stack gap={1.5}>
      {[0, 1, 2].map((i) => (
        <CaseCardSkeleton key={i} />
      ))}
    </Stack>
  );
}

function ServiceRequestListErrorBoundary({ children }: { children: ReactNode }) {
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
