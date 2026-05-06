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
  CaseListContent,
  ChangeRequestListContent,
  ChatListContent,
  FilterSlotBuilderSkeleton,
  FilterSlotContent,
  ItemCardExtendedSkeleton,
  ServiceRequestListContent,
  type ItemCardProps,
} from "@components/features/support";
import { Box, Skeleton, Stack } from "@wso2/oxygen-ui";
import { useLocation, useSearchParams } from "react-router-dom";
import { useLayout } from "@context/layout";
import { useCallback, useEffect, useLayoutEffect, useMemo, useState } from "react";
import { ErrorBoundary } from "../components/core";
import { SecurityReportAnalysisListContent } from "../components/features/support/SecurityReportAnalysisListContent";
import { EngagementListContent } from "../components/features/support/EngagementListContent";
import ErrorState from "../components/shared/ErrorState";
import { useQueryErrorResetBoundary } from "@tanstack/react-query";
import { STATUS_MODE_TYPES } from "../utils/filters";
import EmptyState from "../components/shared/EmptyState";
import { AnnouncementListContent } from "../components/features/support/AnnouncementListContent";

export type ModeType = (OfStatusModeType | OfSeverityModeType) & {
  title: string;
};

export interface OfStatusModeType {
  type: "status";
  status: "action_required" | "outstanding" | "resolved";
}

export interface OfSeverityModeType {
  type: "severity";
  id: string | number;
}

export default function AllItemsPage({ type }: { type: ItemCardProps["type"] | "multiple" }) {
  const [searchParams] = useSearchParams();
  const location = useLocation();
  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();
  const { reset } = useQueryErrorResetBoundary();

  const mode: ModeType | undefined = location.state?.mode;

  const { setTitleOverride } = useLayout();

  useEffect(() => {
    if (!mode) return;

    setTitleOverride(mode.title);

    return () => {
      setTitleOverride(undefined);
    };
  }, [mode]);

  const resolvedTypes =
    type === "multiple" ? (mode?.type === "status" ? (STATUS_MODE_TYPES[mode.status] ?? []) : []) : type;

  return (
    <Stack gap={2}>
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
        <ItemsListContent type={resolvedTypes} filter={filter} search={search} />
      </ErrorBoundary>
    </Stack>
  );
}

function ItemsListContentSingle({
  type,
  filter,
  search,
  mode,
  grouped,
  onCountChange,
}: {
  type: ItemCardProps["type"];
  filter: string;
  search: string;
  mode?: ModeType;
  grouped?: boolean;
  onCountChange?: (count: number | undefined) => void;
}) {
  switch (type) {
    case "case":
      return (
        <CaseListContent filter={filter} search={search} mode={mode} grouped={grouped} onCountChange={onCountChange} />
      );
    case "chat":
      return <ChatListContent filter={filter} search={search} />;
    case "service":
      return (
        <ServiceRequestListContent
          filter={filter}
          search={search}
          mode={mode}
          grouped={grouped}
          onCountChange={onCountChange}
        />
      );
    case "change":
      return (
        <ChangeRequestListContent
          filter={filter}
          search={search}
          mode={mode}
          grouped={grouped}
          onCountChange={onCountChange}
        />
      );
    case "sra":
      return (
        <SecurityReportAnalysisListContent
          filter={filter}
          search={search}
          mode={mode}
          grouped={grouped}
          onCountChange={onCountChange}
        />
      );
    case "engagement":
      return (
        <EngagementListContent
          filter={filter}
          search={search}
          mode={mode}
          grouped={grouped}
          onCountChange={onCountChange}
        />
      );
    case "announcement":
      return <AnnouncementListContent filter={filter} search={search} />;
    default:
      return null;
  }
}

function ItemsListContent({
  type,
  filter,
  search,
}: {
  type: ItemCardProps["type"] | ItemCardProps["type"][];
  filter: string;
  search: string;
}) {
  const location = useLocation();
  const mode: ModeType | undefined = location.state?.mode;

  const types = Array.isArray(type) ? type : [type];
  const grouped = Array.isArray(type);

  const [counts, setCounts] = useState<Record<number, number | undefined>>({});

  const handleCountChange = useCallback((index: number, count: number | undefined) => {
    setCounts((prev) => ({ ...prev, [index]: count }));
  }, []);

  const allSettled = types.every((_, i) => counts[i] !== undefined);
  const allEmpty = allSettled && types.every((_, i) => counts[i] === 0);

  return (
    <>
      {allEmpty && <EmptyState />}
      <Box sx={{ display: allEmpty ? "none" : "contents" }}>
        {types.map((type, index) => (
          <ItemsListContentSingle
            key={index}
            type={type}
            filter={filter}
            search={search}
            mode={mode}
            grouped={grouped}
            onCountChange={(count) => handleCountChange(index, count)}
          />
        ))}
      </Box>
    </>
  );
}

export function FilterAppBarSlot({ type }: { type: ItemCardProps["type"] }) {
  const location = useLocation();
  const mode: ModeType | undefined = location.state?.mode;

  const showTabs = useMemo(() => {
    if (mode) {
      return false;
    }
    return true;
  }, [mode]);

  return (
    <ErrorBoundary fallback={<FilterSlotBuilderSkeleton />}>
      <FilterSlotContent type={type} state={location.state} showTabs={showTabs} />
    </ErrorBoundary>
  );
}

export function ItemsListContentSkeleton() {
  return (
    <>
      {Array.from({ length: 10 }).map((_, index) => (
        <ItemCardExtendedSkeleton key={index} />
      ))}
    </>
  );
}

export function usePaginationSubtitleOverride(count?: number | null, total?: number | null) {
  const layout = useLayout();
  const disabled = count === null && total === null;
  const data = count !== undefined && total !== undefined;

  useLayoutEffect(() => {
    if (!disabled)
      layout.setSubtitleSlotOverride(
        data ? `${count} of ${total}` : <Skeleton variant="text" width={50} height={20} />,
      );

    return () => layout.setSubtitleSlotOverride(null);

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [count, total]);
}
