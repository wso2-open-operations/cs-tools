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
import { accounts } from "@src/services/accounts";
import type { Account } from "@src/types";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { useInfiniteScrollSentinel } from "@utils/useInfiniteScrollSentinel";
import { formatDateOnly } from "@utils/customers";
import { SearchBar } from "@components/support/SearchBar";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";

function AccountCard({ item }: { item: Account }) {
  return (
    <Card
      component={Link}
      to={`/more/customers/accounts/${item.id}`}
      variant="outlined"
      sx={{ p: 2, textDecoration: "none", display: "block" }}
    >
      <Stack gap={1}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" gap={1}>
          <Typography variant="body2" noWrap sx={{ minWidth: 0 }}>
            {item.name}
          </Typography>
          <Chip
            size="small"
            label={item.tier}
            color={item.tier === "enterprise" ? "primary" : "default"}
            variant="outlined"
            sx={{ textTransform: "capitalize" }}
          />
        </Stack>
        <Typography variant="caption" color="text.secondary" noWrap sx={{ fontFamily: "monospace" }}>
          {item.sfId}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {item.region ?? "—"} · Activated {formatDateOnly(item.activationDate)}
          {item.deactivationDate ? ` · Deactivated ${formatDateOnly(item.deactivationDate)}` : ""}
        </Typography>
      </Stack>
    </Card>
  );
}

function AccountCardSkeleton() {
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

// Read-only accounts list (search across name + Salesforce ID), infinite-scrolled — mirrors the
// webapp's CsmAccountsPage's search, minus its Table/TablePagination (this app's list pages are
// all infinite-scrolled, not classically paginated).
export function AccountsTab() {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);

  const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    accounts.infinite(debouncedSearch),
  );

  const items = data?.pages.flatMap((p) => p.items) ?? [];
  const total = data?.pages[0]?.total ?? items.length;

  const sentinelRef = useInfiniteScrollSentinel({ hasNextPage, isFetchingNextPage, fetchNextPage });

  return (
    <Stack gap={2}>
      <SearchBar value={search} onChange={setSearch} placeholder="Search by name or SF ID…" />

      {isLoading ? (
        <Stack gap={1.5}>
          {Array.from({ length: 5 }).map((_, i) => (
            <AccountCardSkeleton key={i} />
          ))}
        </Stack>
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : items.length === 0 ? (
        <EmptyState message="No accounts found." />
      ) : (
        <Stack gap={1.5}>
          <Typography variant="caption" color="text.secondary">
            {items.length} of {total}
          </Typography>

          {items.map((item) => (
            <AccountCard key={item.id} item={item} />
          ))}

          <div ref={sentinelRef} style={{ height: 1 }} />

          {isFetchingNextPage && <AccountCardSkeleton />}
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
