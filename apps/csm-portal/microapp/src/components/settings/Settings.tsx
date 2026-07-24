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
import { Badge, Chip, IconButton, Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { SlidersHorizontal } from "@wso2/oxygen-ui-icons-react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { adminUsers } from "@src/services/adminUsers";
import { ComingSoonPage } from "@components/common/ComingSoonPage";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";
import { SearchBar } from "@components/support/SearchBar";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { ADMIN_TABS, type AdminTabId } from "./config";
import { UserCard, UserCardSkeleton } from "./UserCard";
import { UsersFiltersSheet } from "./UsersFiltersSheet";
import { EMPTY_USERS_FILTERS, countActiveUsersFilters, toUserSearchFilters, type UsersFilters } from "./filters";

// The account-administration section: mirrors the webapp's CsmAdminLayout (Users/Roles/Groups/
// Permissions tabs, only Users built) rather than personal profile editing. Deliberately
// self-contained — it owns its own tab state, search/filters, and data fetching — so it's a
// drop-in that can move to a different page later without threading state through props.
export default function Settings() {
  const [tab, setTab] = useState<AdminTabId>("users");
  const activeTab = ADMIN_TABS.find((t) => t.id === tab) ?? ADMIN_TABS[0];

  return (
    <Stack gap={2}>
      <Typography variant="h5">Settings</Typography>

      <Tabs variant="scrollable" value={tab} onChange={(_, value: AdminTabId) => setTab(value)}>
        {ADMIN_TABS.map((t) => (
          <Tab
            key={t.id}
            value={t.id}
            disableRipple
            label={
              <Stack direction="row" alignItems="center" gap={0.75}>
                {t.label}
                {t.wip && (
                  <Chip size="small" label="WIP" color="warning" variant="outlined" sx={{ height: 18, fontSize: 10 }} />
                )}
              </Stack>
            }
          />
        ))}
      </Tabs>

      {activeTab.id === "users" ? (
        <UsersTab />
      ) : (
        <ComingSoonPage
          title={activeTab.label}
          description={activeTab.description ?? ""}
          blockedOn={activeTab.blockedOn}
        />
      )}
    </Stack>
  );
}

function UsersTab() {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);
  const [filters, setFilters] = useState<UsersFilters>(EMPTY_USERS_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const activeFilterCount = countActiveUsersFilters(filters);

  const { data, isLoading, isError, refetch, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery(
    adminUsers.infinite(toUserSearchFilters(debouncedSearch, filters)),
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

  const users = data?.pages.flatMap((page) => page.users) ?? [];
  const total = data?.pages[0]?.total ?? users.length;

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        Search across username and email. Filter by role and status.
      </Typography>

      <Stack direction="row" gap={1} alignItems="center">
        <SearchBar value={search} onChange={setSearch} placeholder="Search users by username or email" />
        <Badge badgeContent={activeFilterCount} color="primary" invisible={activeFilterCount === 0}>
          <IconButton aria-label="Filters" onClick={() => setFiltersOpen(true)}>
            <SlidersHorizontal size={18} />
          </IconButton>
        </Badge>
      </Stack>

      {isLoading ? (
        <UsersSkeleton />
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : users.length === 0 ? (
        <EmptyState message="No users found." />
      ) : (
        <Stack gap={1.5}>
          <Typography variant="caption" color="text.secondary">
            {users.length} of {total}
          </Typography>

          {users.map((user) => (
            <UserCard key={user.id} user={user} />
          ))}

          {/* IntersectionObserver can miss a zero-height target, so give the sentinel 1px to observe. */}
          <div ref={sentinelRef} style={{ height: 1 }} />

          {isFetchingNextPage && <UserCardSkeleton />}
          {!hasNextPage && (
            <Typography variant="body2" color="text.secondary" textAlign="center" py={2}>
              You're all caught up!
            </Typography>
          )}
        </Stack>
      )}

      <UsersFiltersSheet
        open={filtersOpen}
        onClose={() => setFiltersOpen(false)}
        filters={filters}
        onApply={setFilters}
      />
    </Stack>
  );
}

function UsersSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 6 }).map((_, index) => (
        <UserCardSkeleton key={index} />
      ))}
    </Stack>
  );
}
