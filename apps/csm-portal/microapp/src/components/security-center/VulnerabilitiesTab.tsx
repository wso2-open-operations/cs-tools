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
import { Card, Chip, Stack, Typography } from "@wso2/oxygen-ui";
import { Link } from "react-router-dom";
import { useInfiniteQuery } from "@tanstack/react-query";
import { EMPTY_VULNERABILITY_FILTERS, vulnerabilities, type VulnerabilityFilters } from "@src/services/vulnerabilities";
import type { Vulnerability, VulnerabilityPriority } from "@src/types";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { useInfiniteScrollSentinel } from "@utils/useInfiniteScrollSentinel";
import {
  ALL_VULNERABILITY_PRIORITIES,
  VULNERABILITY_PRIORITY_LABEL,
  vulnerabilityPriorityColor,
} from "@utils/vulnerabilities";
import { SearchBar } from "@components/support/SearchBar";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";

function VulnerabilityCard({ item }: { item: Vulnerability }) {
  return (
    <Card
      component={Link}
      to={`/more/security-center/vulnerabilities/${item.id}`}
      variant="outlined"
      sx={{ p: 2, textDecoration: "none", display: "block" }}
    >
      <Stack gap={1}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" gap={1}>
          <Typography variant="body2" noWrap sx={{ minWidth: 0, fontFamily: "monospace" }}>
            {item.cveId || item.vulnerabilityId || "—"}
          </Typography>
          {item.priority && (
            <Chip
              size="small"
              variant="outlined"
              color={vulnerabilityPriorityColor(item.priority)}
              label={item.priority}
            />
          )}
        </Stack>
        <Typography variant="body2" noWrap>
          {item.componentName || "—"}
          {item.componentVersion ? ` ${item.componentVersion}` : ""}
        </Typography>
        <Typography variant="caption" color="text.secondary" noWrap>
          {item.productName || "—"}
          {item.productVersion ? ` ${item.productVersion}` : ""}
        </Typography>
      </Stack>
    </Card>
  );
}

function VulnerabilityCardSkeleton() {
  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Stack gap={1}>
        <Typography variant="body2" color="text.secondary">
          &nbsp;
        </Typography>
      </Stack>
    </Card>
  );
}

// Read-only product vulnerabilities list (search + single-select priority filter),
// infinite-scrolled — mirrors the webapp's ProductVulnerabilitiesTab, minus its
// Table/TablePagination (this app's list pages are all infinite-scrolled).
export function VulnerabilitiesTab() {
  const [filters, setFilters] = useState<VulnerabilityFilters>(EMPTY_VULNERABILITY_FILTERS);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    vulnerabilities.infinite({ ...filters, search: debouncedSearch }),
  );

  const items = data?.pages.flatMap((p) => p.items) ?? [];
  const total = data?.pages[0]?.total ?? items.length;

  const sentinelRef = useInfiniteScrollSentinel({ hasNextPage, isFetchingNextPage, fetchNextPage });

  const togglePriority = (priority: VulnerabilityPriority) => {
    setFilters((prev) => ({ ...prev, priority: prev.priority === priority ? null : priority }));
  };

  return (
    <Stack gap={2}>
      <SearchBar
        value={filters.search}
        onChange={(value) => setFilters((prev) => ({ ...prev, search: value }))}
        placeholder="Search vulnerabilities…"
      />

      <Stack direction="row" gap={1} flexWrap="wrap">
        {ALL_VULNERABILITY_PRIORITIES.map((priority) => {
          const isSelected = filters.priority === priority;
          return (
            <Chip
              key={priority}
              label={VULNERABILITY_PRIORITY_LABEL[priority]}
              size="small"
              aria-pressed={isSelected}
              variant={isSelected ? "filled" : "outlined"}
              color={isSelected ? "primary" : "default"}
              onClick={() => togglePriority(priority)}
            />
          );
        })}
      </Stack>

      {isLoading ? (
        <Stack gap={1.5}>
          {Array.from({ length: 5 }).map((_, i) => (
            <VulnerabilityCardSkeleton key={i} />
          ))}
        </Stack>
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : items.length === 0 ? (
        <EmptyState message="No vulnerabilities found." />
      ) : (
        <Stack gap={1.5}>
          <Typography variant="caption" color="text.secondary">
            {items.length} of {total}
          </Typography>

          {items.map((item) => (
            <VulnerabilityCard key={item.id} item={item} />
          ))}

          <div ref={sentinelRef} style={{ height: 1 }} />

          {isFetchingNextPage && <VulnerabilityCardSkeleton />}
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
