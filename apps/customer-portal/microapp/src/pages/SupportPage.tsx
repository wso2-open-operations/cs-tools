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

import { Suspense, useState, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { Card, Stack, Tab, Tabs } from "@wso2/oxygen-ui";
import { ItemListView, ItemCard, type ItemCardProps, ItemCardSkeleton } from "@components/features/support";

import { useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { useProject } from "@context/project";
import { ErrorBoundary, Fab } from "@components/core";
import { chats } from "../services/chats";
import { changeRequests } from "../services/changes";
import { serviceRequests } from "../services/services";
import EmptyState from "../components/shared/EmptyState";
import { useNotify } from "../context/snackbar";
import {
  ITEM_DETAIL_PATHS,
  OUTSTANDING_CASE_STATUS_IDS,
  OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS,
  OUTSTANDING_CONVERSATIONS_STATUS_IDS,
  TAB_CONFIG,
} from "../config/constants";
import { securityReportAnalysis } from "../services/sra";
import { engagements } from "../services/engagements";
import ErrorState from "../components/shared/ErrorState";

type TabType = ItemCardProps["type"];

export default function SupportPage() {
  const { features } = useProject();
  const [searchParams, setSearchParams] = useSearchParams();
  const rawTab = searchParams.get("tab");
  const allowedTabs: TabType[] = ["case", "chat", "service", "change"];
  const tabFromParams = allowedTabs.find((t) => t === rawTab) ?? "case";
  const [tab, setTab] = useState<TabType>(tabFromParams);

  const handleTabChange = (tab: TabType) => {
    setTab(tab);
    setSearchParams(
      (prev) => {
        prev.set("tab", tab);
        return prev;
      },
      { replace: true },
    );
  };

  return (
    <>
      <Tabs variant="scrollable" value={tab} onChange={(_, value) => handleTabChange(value)}>
        <Tab label="Cases" value="case" disableRipple />
        <Tab label="Chats" value="chat" disableRipple />
        {features?.hasServiceRequestReadAccess && <Tab label="Service Requests" value="service" disableRipple />}
        {features?.hasChangeRequestReadAccess && <Tab label="Change Requests" value="change" disableRipple />}
        <Tab label="Security Report Analysis" value="sra" disableRipple />
        {features?.hasEngagementsReadAccess && <Tab label="Engagements" value="engagement" disableRipple />}
      </Tabs>
      <Card component={Stack} p={2} mt={2} gap={0.5}>
        <ItemListView title={TAB_CONFIG[tab].title} subtitle={TAB_CONFIG[tab].subtitle} viewAllPath={`/${tab}s/all`}>
          <ItemsListContent tab={tab} />
        </ItemListView>
      </Card>

      {/* Floating Action Button */}
      <Fab />
    </>
  );
}

function ItemsListContent({ tab }: { tab: ItemCardProps["type"] }) {
  switch (tab) {
    case "case":
      return (
        <ItemsListErrorBoundary>
          <Suspense fallback={<ItemsListContentSkeleton />}>
            <CaseItemListContent />
          </Suspense>
        </ItemsListErrorBoundary>
      );

    case "chat":
      return (
        <ItemsListErrorBoundary>
          <Suspense fallback={<ItemsListContentSkeleton />}>
            <ChatItemListContent />
          </Suspense>
        </ItemsListErrorBoundary>
      );

    case "service":
      return (
        <ItemsListErrorBoundary>
          <Suspense fallback={<ItemsListContentSkeleton />}>
            <ServiceRequestItemListContent />
          </Suspense>
        </ItemsListErrorBoundary>
      );

    case "change":
      return (
        <ItemsListErrorBoundary>
          <Suspense fallback={<ItemsListContentSkeleton />}>
            <ChangeRequestItemListContent />
          </Suspense>
        </ItemsListErrorBoundary>
      );

    case "sra":
      return (
        <ItemsListErrorBoundary>
          <Suspense fallback={<ItemsListContentSkeleton />}>
            <SecurityReportAnalysisItemListContent />
          </Suspense>
        </ItemsListErrorBoundary>
      );

    case "engagement":
      return (
        <ItemsListErrorBoundary>
          <Suspense fallback={<ItemsListContentSkeleton />}>
            <EngagementItemListContent />
          </Suspense>
        </ItemsListErrorBoundary>
      );
  }
}

function CaseItemListContent() {
  const { projectId } = useProject();
  const { data } = useSuspenseQuery(
    cases.all(projectId!, {
      filters: {
        statusIds: OUTSTANDING_CASE_STATUS_IDS,
      },
      pagination: { limit: 5 },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="case" to={ITEM_DETAIL_PATHS["case"](item.id)} {...item} />
      ))}
    </>
  );
}

function ChatItemListContent() {
  const { projectId } = useProject();
  const { data } = useSuspenseQuery(
    chats.all(projectId!, {
      filters: {
        stateKeys: OUTSTANDING_CONVERSATIONS_STATUS_IDS,
      },
      pagination: { limit: 5 },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="chat" to={ITEM_DETAIL_PATHS["chat"](item.id)} {...item} />
      ))}
    </>
  );
}

function ChangeRequestItemListContent() {
  const { projectId } = useProject();
  const { data } = useSuspenseQuery(
    changeRequests.all(projectId!, {
      filters: { stateKeys: OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS },
      pagination: { offset: 0, limit: 5 },
    }),
  );

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="change" to={ITEM_DETAIL_PATHS["change"](item.id)} {...item} />
      ))}
    </>
  );
}

function ServiceRequestItemListContent() {
  const { projectId } = useProject();
  const { data } = useSuspenseQuery(
    serviceRequests.all(projectId!, {
      filters: {
        statusIds: OUTSTANDING_CASE_STATUS_IDS,
      },
      pagination: { limit: 5 },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="service" to={ITEM_DETAIL_PATHS["service"](item.id)} {...item} />
      ))}
    </>
  );
}

function SecurityReportAnalysisItemListContent() {
  const { projectId } = useProject();
  const { data } = useSuspenseQuery(
    securityReportAnalysis.all(projectId!, {
      filters: {
        statusIds: OUTSTANDING_CASE_STATUS_IDS,
      },
      pagination: { limit: 5 },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="sra" to={ITEM_DETAIL_PATHS["sra"](item.id)} {...item} />
      ))}
    </>
  );
}

function EngagementItemListContent() {
  const { projectId } = useProject();
  const { data } = useSuspenseQuery(
    engagements.all(projectId!, {
      filters: {
        statusIds: OUTSTANDING_CASE_STATUS_IDS,
      },
      pagination: { limit: 5 },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="engagement" to={ITEM_DETAIL_PATHS["engagement"](item.id)} {...item} />
      ))}
    </>
  );
}

function ItemsListContentSkeleton() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, index) => (
        <ItemCardSkeleton key={index} />
      ))}
    </>
  );
}

function ItemsListErrorBoundary({ children }: { children: ReactNode }) {
  const notify = useNotify();
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
      onError={() => notify.error("Content failed to load. Please try again later.")}
    >
      {children}
    </ErrorBoundary>
  );
}
