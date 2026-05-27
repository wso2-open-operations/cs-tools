import { type ReactNode } from "react";

import type { InfiniteData, UseInfiniteQueryResult } from "@tanstack/react-query";

import { useDeclareLayout } from "@context/layout";

import {
  AnnouncementItemCard,
  CaseItemCard,
  ChangeRequestItemCard,
  ChatItemCard,
  EngagementItemCard,
  FilterContentSkeleton,
  Filters,
  ItemsListSubtitle,
  ItemsListWrapper,
  SecurityReportAnalysisItemCard,
  ServiceRequestItemCard,
} from "@features/items/components";
import {
  useAnnouncementItems,
  useCaseItems,
  useChangeRequestItems,
  useChatItems,
  useEngagementItems,
  useFilters,
  useInfiniteListTail,
  useSecurityReportAnalysisItems,
  useServiceRequestItems,
} from "@features/items/hooks";

import { InfiniteList } from "@shared/components/common";

import { CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

export function CaseItemsList() {
  const { query, total, count } = useCaseItems();
  const { filters } = useFilters();

  const enabled = filters.types.length === 1 && !filters.severities?.length && (filters.statuses?.length ?? 0) <= 1;

  useDeclareLayout(
    {
      title: "All Cases",
      slots: {
        bottom: <Filters type={CASE_TYPES.DEFAULT} />,
        subtitle: <ItemsListSubtitle count={count} total={total} />,
      },
    },
    { enabled },
    [total, count, enabled],
  );

  return (
    <ItemsList type={CASE_TYPES.DEFAULT} query={query} total={count}>
      {(item) => <CaseItemCard {...item} />}
    </ItemsList>
  );
}

export function ChatItemsList() {
  const { query, total, count } = useChatItems();
  const { filters } = useFilters();

  const enabled = filters.types.length === 1 && !filters.severities?.length && (filters.statuses?.length ?? 0) <= 1;

  useDeclareLayout(
    {
      title: "All Chats",
      slots: {
        bottom: <Filters type={CASE_TYPES.CHAT} />,
        subtitle: <ItemsListSubtitle count={count} total={total} />,
      },
    },
    { enabled },
    [total, count],
  );

  return (
    <ItemsList type={CASE_TYPES.CHAT} query={query} total={count}>
      {(item) => <ChatItemCard {...item} />}
    </ItemsList>
  );
}

export function ServiceRequestItemsList() {
  const { query, total, count } = useServiceRequestItems();
  const { filters } = useFilters();

  const enabled = filters.types.length === 1 && !filters.severities?.length && (filters.statuses?.length ?? 0) <= 1;

  useDeclareLayout(
    {
      title: "All Service Requests",
      slots: {
        bottom: <Filters type={CASE_TYPES.SERVICE_REQUEST} />,
        subtitle: <ItemsListSubtitle count={count} total={total} />,
      },
    },
    { enabled },
    [total, count],
  );

  return (
    <ItemsList type={CASE_TYPES.SERVICE_REQUEST} query={query} total={count}>
      {(item) => <ServiceRequestItemCard {...item} />}
    </ItemsList>
  );
}

export function ChangeRequestItemsList() {
  const { query, total, count } = useChangeRequestItems();
  const { filters } = useFilters();

  const enabled = filters.types.length === 1 && !filters.severities?.length && (filters.states?.length ?? 0) <= 1;

  useDeclareLayout(
    {
      title: "All Change Requests",
      slots: {
        bottom: <Filters type={CASE_TYPES.CHANGE_REQUEST} />,
        subtitle: <ItemsListSubtitle count={count} total={total} />,
      },
    },
    { enabled },
    [total, count],
  );

  return (
    <ItemsList type={CASE_TYPES.CHANGE_REQUEST} query={query} total={count}>
      {(item) => <ChangeRequestItemCard {...item} />}
    </ItemsList>
  );
}

export function SecurityReportAnalysisItemsList() {
  const { query, total, count } = useSecurityReportAnalysisItems();
  const { filters } = useFilters();

  const enabled = filters.types.length === 1 && !filters.severities?.length && (filters.statuses?.length ?? 0) <= 1;

  useDeclareLayout(
    {
      title: "All Security Report Analysis",
      slots: {
        bottom: <Filters type={CASE_TYPES.SERVICE_REQUEST} />,
        subtitle: <ItemsListSubtitle count={count} total={total} />,
      },
    },
    { enabled },
    [total, count],
  );

  return (
    <ItemsList type={CASE_TYPES.SECURITY_REPORT_ANALYSIS} query={query} total={count}>
      {(item) => <SecurityReportAnalysisItemCard {...item} />}
    </ItemsList>
  );
}

export function EngagementItemsList() {
  const { query, total, count } = useEngagementItems();
  const { filters } = useFilters();

  const enabled = filters.types.length === 1 && !filters.severities?.length && (filters.statuses?.length ?? 0) <= 1;

  useDeclareLayout(
    {
      title: "All Engagements",
      slots: {
        bottom: <Filters type={CASE_TYPES.ENGAGEMENT} />,
        subtitle: <ItemsListSubtitle count={count} total={total} />,
      },
    },
    { enabled },
    [total, count],
  );

  return (
    <ItemsList type={CASE_TYPES.ENGAGEMENT} query={query} total={count}>
      {(item) => <EngagementItemCard {...item} />}
    </ItemsList>
  );
}

export function AnnouncementItemsList() {
  const { query, total, count } = useAnnouncementItems();
  const { filters } = useFilters();

  const enabled = filters.types.length === 1 && !filters.severities?.length && (filters.statuses?.length ?? 0) <= 1;

  useDeclareLayout(
    {
      title: "All Announcements",
      slots: {
        bottom: <Filters variant="search-only" type={CASE_TYPES.ANNOUNCEMENT} />,
        subtitle: <ItemsListSubtitle count={count} total={total} />,
      },
    },
    { enabled },
    [total, count],
  );

  return (
    <ItemsList type={CASE_TYPES.ANNOUNCEMENT} query={query} total={count}>
      {(item) => <AnnouncementItemCard {...item} />}
    </ItemsList>
  );
}

function ItemsList<TItem, TError>({
  type,
  query,
  total,
  children,
}: {
  type: CaseType;
  query: Pick<
    UseInfiniteQueryResult<InfiniteData<TItem[]>, TError>,
    "data" | "hasNextPage" | "isFetchingNextPage" | "fetchNextPage"
  >;
  total?: number;
  children: (item: TItem, index: number) => ReactNode;
}) {
  const tail = useInfiniteListTail(total ?? 0);

  return (
    <ItemsListWrapper type={type} count={total}>
      <InfiniteList {...query} sentinel={<FilterContentSkeleton />} tail={tail}>
        {children}
      </InfiniteList>
    </ItemsListWrapper>
  );
}
