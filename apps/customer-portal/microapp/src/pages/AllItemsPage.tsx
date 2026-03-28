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
  FilterSlotBuilderSkeleton,
  ItemCardExtended,
  ItemCardExtendedSkeleton,
  type ItemCardProps,
} from "@components/features/support";
import { InfiniteScroll } from "@components/shared";
import { Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { useSearchParams } from "react-router-dom";
import { useLayout } from "@context/layout";
import React, { Fragment, Suspense, useLayoutEffect } from "react";
import { useInfiniteQuery, useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { useProject } from "@context/project";
import { chats } from "@src/services/chats";
import { changeRequests } from "@src/services/changes";

import { ITEM_DETAIL_PATHS } from "@pages/SupportPage";
import { serviceRequests } from "../services/services";
import type { GetCasesRequestDTO, GetChangeRequestsRquestDTO } from "../types";
import type { GetChatsRequestDTO } from "../types/chat.dto";
import EmptyState from "../components/shared/EmptyState";
import { ErrorBoundary } from "../components/core";

export default function AllItemsPage({ type }: { type: ItemCardProps["type"] }) {
  const [searchParams] = useSearchParams();
  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();

  return (
    <Stack gap={2}>
      <ErrorBoundary fallback={<ItemsListContentSkeleton />}>
        <ItemsListContent type={type} filter={filter} search={search} />
      </ErrorBoundary>
    </Stack>
  );
}

export function FilterAppBarSlot({ type }: { type: ItemCardProps["type"] }) {
  return (
    <ErrorBoundary fallback={<FilterSlotBuilderSkeleton />}>
      <Suspense fallback={<FilterSlotBuilderSkeleton />}>
        <FilterSlotContent type={type} />
      </Suspense>
    </ErrorBoundary>
  );
}

function FilterSlotContent({ type }: { type: ItemCardProps["type"] }) {
  const { projectId } = useProject();
  const { data: filters } = useSuspenseQuery(cases.filters(projectId!));

  const SEARCH_PLACEHOLDER_CONFIG: Record<typeof type, string> = {
    case: "Search cases by ID, title, or description...",
    chat: "Search chats by ID, title, or message...",
    service: "Search Service Requests by ID, title, or description...",
    change: "Search Change Requests by ID, title, or description...",
  };

  const tabs = (() => {
    switch (type) {
      case "chat":
        return filters.conversationStates;
      case "change":
        return filters.changeRequestStates;
      default:
        return filters.caseStates;
    }
  })().map((filter) => ({ label: filter.label, value: filter.id }));

  return <FilterSlotBuilder searchPlaceholder={SEARCH_PLACEHOLDER_CONFIG[type]} tabs={tabs} />;
}

function ItemsListContent({ type, filter, search }: { type: ItemCardProps["type"]; filter: string; search: string }) {
  switch (type) {
    case "case":
      return <CaseListContent filter={filter} search={search} />;
    case "chat":
      return <ChatListContent filter={filter} search={search} />;
    case "service":
      return <ServiceRequestsListContent filter={filter} search={search} />;
    case "change":
      return <ChangeRequestsListContent filter={filter} search={search} />;
    default:
      return null;
  }
}

function CaseListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: GetCasesRequestDTO["filters"] = {};

  if (filter !== "all") {
    filters.statusIds = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }

  const totalQuery = useQuery(cases.all(projectId!));
  const query = useInfiniteQuery(cases.paginated(projectId!, { filters }));

  const total = totalQuery.data?.pagination.totalRecords;
  const count = query.data?.pages[0].pagination.totalRecords;

  useSubtitleOverride(count, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        count === 0 ? (
          <EmptyState />
        ) : (
          <Typography variant="subtitle2" textAlign="center">
            You're all caught up!
          </Typography>
        )
      }
    >
      {(data) => (
        <>
          {data &&
            data.pages.map((page, pageIndex) => (
              <Fragment key={pageIndex}>
                {page.map((item) => (
                  <ItemCardExtended key={item.id} type="case" to={ITEM_DETAIL_PATHS.case(item.id)} {...item} />
                ))}
              </Fragment>
            ))}
        </>
      )}
    </InfiniteScroll>
  );
}

function ChatListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: GetChatsRequestDTO["filters"] = {};

  if (filter !== "all") {
    filters.stateKeys = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }
  const totalQuery = useQuery(chats.all(projectId!));
  const query = useInfiniteQuery(chats.paginated(projectId!, { filters }));

  const total = totalQuery.data?.pagination.totalRecords;
  const count = query.data?.pages[0].pagination.totalRecords;

  useSubtitleOverride(count, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        count === 0 ? (
          <EmptyState />
        ) : (
          <Typography variant="subtitle2" textAlign="center">
            You're all caught up!
          </Typography>
        )
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

function ChangeRequestsListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: GetChangeRequestsRquestDTO["filters"] = {};

  if (filter !== "all") {
    filters.stateKeys = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }

  const totalQuery = useQuery(changeRequests.all(projectId!));
  const query = useInfiniteQuery(changeRequests.paginated(projectId!, { filters }));

  const total = totalQuery.data?.pagination.totalRecords;
  const count = query.data?.pages[0].pagination.totalRecords;

  useSubtitleOverride(count, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        count === 0 ? (
          <EmptyState />
        ) : (
          <Typography variant="subtitle2" textAlign="center">
            You're all caught up!
          </Typography>
        )
      }
    >
      {(data) => (
        <>
          {data &&
            data.pages.map((page) =>
              page.map((item) => (
                <ItemCardExtended key={item.id} type="change" to={ITEM_DETAIL_PATHS.change(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );
}

function ServiceRequestsListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: GetCasesRequestDTO["filters"] = {};

  if (filter !== "all") {
    filters.statusIds = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }

  const totalQuery = useQuery(serviceRequests.all(projectId!));
  const query = useInfiniteQuery(serviceRequests.paginated(projectId!, { filters }));

  const total = totalQuery.data?.pagination.totalRecords;
  const count = query.data?.pages[0].pagination.totalRecords;

  useSubtitleOverride(count, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        count === 0 ? (
          <EmptyState />
        ) : (
          <Typography variant="subtitle2" textAlign="center">
            You're all caught up!
          </Typography>
        )
      }
    >
      {(data) => (
        <>
          {data &&
            data.pages.map((page) =>
              page.map((item) => (
                <ItemCardExtended key={item.id} type="service" to={ITEM_DETAIL_PATHS.service(item.id)} {...item} />
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
  const data = count !== undefined && total !== undefined;

  useLayoutEffect(() => {
    layout.setSubtitleSlotOverride(data ? `${count} of ${total}` : <Skeleton variant="text" width={50} height={20} />);
    return () => layout.setSubtitleSlotOverride(null);
  }, [count, total]);
}
