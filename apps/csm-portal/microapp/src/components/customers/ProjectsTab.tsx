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
import { Card, Chip, Stack, Typography } from "@wso2/oxygen-ui";
import { Link } from "react-router-dom";
import { useInfiniteQuery } from "@tanstack/react-query";
import { projects } from "@src/services/projects";
import type { ProjectSummary } from "@src/types";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { formatDateOnly, formatEnumLabel } from "@utils/customers";
import { SearchBar } from "@components/support/SearchBar";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";

function ProjectCard({ item }: { item: ProjectSummary }) {
  return (
    <Card
      component={Link}
      to={`/more/customers/projects/${item.id}`}
      variant="outlined"
      sx={{ p: 2, textDecoration: "none", display: "block" }}
    >
      <Stack gap={1}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" gap={1}>
          <Typography variant="body2" noWrap sx={{ minWidth: 0 }}>
            {item.name}
          </Typography>
          <Chip size="small" label={formatEnumLabel(item.subscriptionType)} variant="outlined" />
        </Stack>
        <Typography variant="caption" color="text.secondary" noWrap sx={{ fontFamily: "monospace" }}>
          {item.key ?? "—"}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {formatDateOnly(item.startDate)} – {formatDateOnly(item.endDate)}
        </Typography>
      </Stack>
    </Card>
  );
}

function ProjectCardSkeleton() {
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

// Read-only projects list (search across name, project key, subscription), infinite-scrolled —
// mirrors the webapp's CsmProjectsPage's search, minus its Table/TablePagination.
export function ProjectsTab() {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);

  const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    projects.list(debouncedSearch),
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
      <SearchBar value={search} onChange={setSearch} placeholder="Search by name, key, or subscription…" />

      {isLoading ? (
        <Stack gap={1.5}>
          {Array.from({ length: 5 }).map((_, i) => (
            <ProjectCardSkeleton key={i} />
          ))}
        </Stack>
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : items.length === 0 ? (
        <EmptyState message="No projects found." />
      ) : (
        <Stack gap={1.5}>
          <Typography variant="caption" color="text.secondary">
            {items.length} of {total}
          </Typography>

          {items.map((item) => (
            <ProjectCard key={item.id} item={item} />
          ))}

          <div ref={sentinelRef} style={{ height: 1 }} />

          {isFetchingNextPage && <ProjectCardSkeleton />}
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
