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

import {
  FilterSlotBuilder,
  ItemCardExtended,
  ItemCardExtendedSkeleton,
  type ItemCardProps,
} from "@components/features/support";
import { InfiniteScroll } from "@components/shared";
import { Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { useSearchParams } from "react-router-dom";
import { useLayout } from "@context/layout";
import { useLayoutEffect } from "react";
import { useInfiniteQuery, useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { useProject } from "@context/project";
import { chats } from "@src/services/chats";
import { changeRequests } from "@src/services/changes";

import { ITEM_DETAIL_PATHS } from "@pages/SupportPage";

export default function AllItemsPage({ type }: { type: ItemCardProps["type"] }) {
  const [searchParams] = useSearchParams();
  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();

  return (
    <Stack gap={2}>
      <ItemsListContent type={type} filter={filter} search={search} />
    </Stack>
  );
}

export function FilterAppBarSlot({ type }: { type: ItemCardProps["type"] | "notifications" }) {
  const { projectId } = useProject();
  const { data: filters } = useSuspenseQuery(cases.filters(projectId!));

  const SEARCH_PLACEHOLDER_CONFIG: Record<typeof type, string> = {
    case: "Search cases by ID, title, or description...",
    chat: "Search chats by ID, title, or message...",
    change: "Search Change Requests by ID, title, or description...",
    notifications: "Search Notifications",
  };

  return (
    <FilterSlotBuilder
      searchPlaceholder={SEARCH_PLACEHOLDER_CONFIG[type]}
      tabs={filters.caseStates.map((filter) => ({ label: filter.label, value: filter.id }))}
    />
  );
}

function ItemsListContent({ type, filter, search }: { type: ItemCardProps["type"]; filter: string; search: string }) {
  switch (type) {
    case "case":
      return <CaseListContent filter={filter} />;
    case "chat":
      return <ChatListContent filter={filter} />;
    case "change":
      return <ChangeRequestsListContent filter={filter} />;
    default:
      return null;
  }
}

function CaseListContent({ filter }: { filter: string }) {
  const { projectId } = useProject();
  const query = useInfiniteQuery(
    cases.paginated(projectId!, filter !== "all" ? { filters: { statusIds: [Number(filter)] } } : undefined),
  );
  const total = query.data?.pages[0].pagination.totalRecords;

  useSubtitleOverride(total, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        <Typography variant="subtitle2" textAlign="center">
          You're all caught up!
        </Typography>
      }
    >
      {(data) => (
        <>
          {data &&
            data.pages.map((page) =>
              page.map((item) => (
                <ItemCardExtended key={item.id} type="case" to={ITEM_DETAIL_PATHS.case(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );
}

function ChatListContent({ filter }: { filter: string }) {
  const { projectId } = useProject();
  const query = useInfiniteQuery(
    chats.paginated(projectId!, filter !== "all" ? { filters: { stateKeys: [Number(filter)] } } : undefined),
  );
  const total = query.data?.pages[0].pagination.totalRecords;

  useSubtitleOverride(total, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        <Typography variant="subtitle2" textAlign="center">
          You're all caught up!
        </Typography>
      }
    >
      {(data) => (
        <>
          {data &&
            data.pages.map((page) =>
              page.map((item) => (
                <ItemCardExtended key={item.id} type="chat" to={ITEM_DETAIL_PATHS.chat(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );
}

function ChangeRequestsListContent({ filter }: { filter: string }) {
  const { projectId } = useProject();
  const query = useInfiniteQuery(
    changeRequests.paginated(projectId!, filter !== "all" ? { filters: { stateKeys: [Number(filter)] } } : undefined),
  );
  const total = query.data?.pages[0].pagination.totalRecords;

  useSubtitleOverride(total, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        <Typography variant="subtitle2" textAlign="center">
          You're all caught up!
        </Typography>
      }
    >
      {(data) => (
        <>
          {data &&
            data.pages.map((page) =>
              page.map((item) => (
                <ItemCardExtended key={item.id} type="change" to={ITEM_DETAIL_PATHS.chat(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );
}

function ItemsListContentSkeleton() {
  return (
    <>
      {Array.from({ length: 10 }).map((_, index) => (
        <ItemCardExtendedSkeleton key={index} />
      ))}
    </>
  );
}

function useSubtitleOverride(count?: number, total?: number) {
  const layout = useLayout();
  const data = count && total;

  useLayoutEffect(() => {
    layout.setSubtitleSlotOverride(data ? `${count} of ${total}` : <Skeleton variant="text" width={50} height={20} />);
    return () => layout.setSubtitleSlotOverride(null);
  }, [count, total]);
}
