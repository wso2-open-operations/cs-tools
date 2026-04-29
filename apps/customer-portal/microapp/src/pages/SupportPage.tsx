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
import { ItemListView, ItemCard, type ItemCardProps, ItemCardSkeleton } from "@components/support";

import { useQueryErrorResetBoundary } from "@tanstack/react-query";
import { useProject } from "@context/project";
import { ErrorBoundary, Fab } from "@components/core";
import EmptyState from "@components/common/EmptyState";
import { useNotify } from "@context/snackbar";
import { ITEM_DETAIL_PATHS, TAB_CONFIG } from "@config/constants";
import ErrorState from "@components/common/ErrorState";
import {
  useOutstandingCases,
  useOutstandingChats,
  useOutstandingChangeRequests,
  useOutstandingServiceRequests,
  useOutstandingSecurityReports,
  useOutstandingEngagements,
  useOutstandingAnnouncements,
} from "@features/dashboard/hooks/useOutstandingItems";

type TabType = ItemCardProps["type"];

export default function SupportPage() {
  const { features } = useProject();
  const [searchParams, setSearchParams] = useSearchParams();
  const rawTab = searchParams.get("tab");
  const allowedTabs: TabType[] = ["case", "chat", "service", "change", "sra", "engagement", "announcement"];
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
        <Tab label="Announcements" value="announcement" disableRipple />
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

    case "announcement":
      return (
        <ItemsListErrorBoundary>
          <Suspense fallback={<ItemsListContentSkeleton />}>
            <AnnouncementItemListContent />
          </Suspense>
        </ItemsListErrorBoundary>
      );
  }
}

function CaseItemListContent() {
  const { projectId } = useProject();
  const { data } = useOutstandingCases(projectId!);

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
  const { data } = useOutstandingChats(projectId!);

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
  const { data } = useOutstandingChangeRequests(projectId!);

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
  const { data } = useOutstandingServiceRequests(projectId!);

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
  const { data } = useOutstandingSecurityReports(projectId!);

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
  const { data } = useOutstandingEngagements(projectId!);

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="engagement" to={ITEM_DETAIL_PATHS["engagement"](item.id)} {...item} />
      ))}
    </>
  );
}

function AnnouncementItemListContent() {
  const { projectId } = useProject();
  const { data } = useOutstandingAnnouncements(projectId!);

  if (data.length === 0) return <EmptyState />;

  return (
    <>
      {data.map((item) => (
        <ItemCard key={item.id} type="announcement" to={ITEM_DETAIL_PATHS["announcement"](item.id)} {...item} />
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
