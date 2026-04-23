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
import { Skeleton, Stack } from "@wso2/oxygen-ui";
import { useLocation, useSearchParams } from "react-router-dom";
import { useLayout } from "@context/layout";
import { useEffect, useLayoutEffect, useMemo } from "react";
import { ErrorBoundary } from "../components/core";
import { SecurityReportAnalysisListContent } from "../components/features/support/SecurityReportAnalysisListContent";
import { EngagementListContent } from "../components/features/support/EngagementListContent";
import ErrorState from "../components/shared/ErrorState";
import { useQueryErrorResetBoundary } from "@tanstack/react-query";

export type ModeType = OfStatusModeType | OfSeverityModeType;

export interface OfStatusModeType {
  type: "status";
  status: "action_required" | "outstanding" | "resolved";
}

export interface OfSeverityModeType {
  type: "severity";
  severity: string;
}

export default function AllItemsPage({ type }: { type: ItemCardProps["type"] }) {
  const [searchParams] = useSearchParams();
  const location = useLocation();
  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();
  const { reset } = useQueryErrorResetBoundary();

  const mode: ModeType | undefined = location.state?.mode;

  const { setTitleOverride } = useLayout();

  useEffect(() => {
    if (!mode) return;

    const value = (() => {
      switch (mode.type) {
        case "status":
          switch (mode.status) {
            case "action_required":
              return "Action Required";

            case "outstanding":
              return "Outstanding";

            case "resolved":
              return "Resolved";
          }
      }
    })();

    setTitleOverride(value);

    return () => {
      setTitleOverride(undefined);
    };
  }, [mode]);

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
        <ItemsListContent type={type} filter={filter} search={search} />
      </ErrorBoundary>
    </Stack>
  );
}

function ItemsListContent({ type, filter, search }: { type: ItemCardProps["type"]; filter: string; search: string }) {
  const location = useLocation();
  const mode: ModeType | undefined = location.state?.mode;

  switch (type) {
    case "case":
      return <CaseListContent filter={filter} search={search} mode={mode} />;
    case "chat":
      return <ChatListContent filter={filter} search={search} />;
    case "service":
      return <ServiceRequestListContent filter={filter} search={search} />;
    case "change":
      return <ChangeRequestListContent filter={filter} search={search} />;
    case "sra":
      return <SecurityReportAnalysisListContent filter={filter} search={search} />;
    case "engagement":
      return <EngagementListContent filter={filter} search={search} />;
    default:
      return null;
  }
}

export function FilterAppBarSlot({ type }: { type: ItemCardProps["type"] }) {
  const location = useLocation();
  const mode: ModeType | undefined = location.state?.mode;

  const showTabs = useMemo(() => {
    if (mode?.type === "status") {
      return (["action_required", "outstanding"] as (typeof mode.status)[]).includes(mode.status);
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

export function usePaginationSubtitleOverride(count?: number, total?: number) {
  const layout = useLayout();
  const data = count !== undefined && total !== undefined;

  useLayoutEffect(() => {
    layout.setSubtitleSlotOverride(data ? `${count} of ${total}` : <Skeleton variant="text" width={50} height={20} />);
    return () => layout.setSubtitleSlotOverride(null);

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [count, total]);
}
