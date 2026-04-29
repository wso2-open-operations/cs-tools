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

import { useParams } from "react-router";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
import { useEffect, useMemo, type JSX } from "react";
import { Grid, Stack } from "@wso2/oxygen-ui";
import { FileText, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { useAsgardeo } from "@asgardeo/react";
import CasesOverviewStatCard from "@features/support/components/cases-overview-stats/CasesOverviewStatCard";
import SupportOverviewCard from "@features/support/components/support-overview-cards/SupportOverviewCard";
import { SupportOverviewIconVariant } from "@features/support/types/supportOverview";
import OutstandingCasesList from "@features/support/components/support-overview-cards/OutstandingCasesList";
import ChatHistoryList from "@features/support/components/support-overview-cards/ChatHistoryList";
import { useGetProjectSupportStats } from "@features/support/api/useGetProjectSupportStats";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import { useSearchConversations } from "@features/support/api/useSearchConversations";
import { useLogger } from "@hooks/useLogger";
import {
  SUPPORT_OVERVIEW_CASES_LIMIT,
  SUPPORT_OVERVIEW_CHAT_LIMIT,
  CaseType,
} from "@features/support/constants/supportConstants";
import { getProjectPermissions } from "@utils/permission";
import { isS0Case } from "@features/support/utils/support";
import { SortOrder } from "@/types/common";
import { resolveCasesTableDefaultStatusIds } from "@features/dashboard/utils/casesTable";
import type { ChatHistoryItem } from "@features/support/types/conversations";

/**
 * SupportPage component to display case details for a project.
 *
 * @returns {JSX.Element} The rendered Support page.
 */
export default function SupportPage(): JSX.Element {
  const logger = useLogger();
  const navigate = useModifierAwareNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const supportPath = `/projects/${projectId}/support`;

  const { data: project } = useGetProjectDetails(projectId || "");
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");
  const includeS0InSupportMetrics =
    !isProjectFeaturesLoading && projectFeatures
      ? getProjectPermissions(project?.type?.label, {
          projectFeatures,
        }).includeS0InSupportMetrics
      : undefined;

  const {
    data: stats,
    isLoading,
    isError,
  } = useGetProjectSupportStats(projectId || "", {
    caseTypes: [CaseType.DEFAULT_CASE],
  });

  const { data: filterMetadata, isLoading: isFilterMetadataLoading, isError: isFilterMetadataError } =
    useGetProjectFilters(projectId || "");

  const nonClosedStatusIds = useMemo(
    () => resolveCasesTableDefaultStatusIds(filterMetadata?.caseStates),
    [filterMetadata?.caseStates],
  );

  const {
    data: casesPageData,
    isLoading: isCasesDataLoading,
    isError: isCasesError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.DEFAULT_CASE],
        statusIds: nonClosedStatusIds,
      },
      sortBy: { field: "createdOn", order: SortOrder.DESC },
    },
    0,
    SUPPORT_OVERVIEW_CASES_LIMIT,
    { enabled: !!projectId && filterMetadata !== undefined },
  );

  const isCasesLoading = isFilterMetadataLoading || isCasesDataLoading;
  const combinedIsCasesError = isCasesError || isFilterMetadataError;

  const {
    data: conversationsData,
    isLoading: isChatLoading,
    isError: isChatError,
  } = useSearchConversations(projectId || "", {
    pagination: { limit: SUPPORT_OVERVIEW_CHAT_LIMIT, offset: 0 },
    sortBy: { field: "updatedOn", order: SortOrder.DESC },
  });

  const { isLoading: isAuthLoading } = useAsgardeo();

  const allFetchedCases = casesPageData?.cases ?? [];
  const cases =
    includeS0InSupportMetrics === false
      ? allFetchedCases.filter((c) => !isS0Case(c))
      : allFetchedCases;

  const chatItems: ChatHistoryItem[] = (
    conversationsData?.conversations?.slice(0, SUPPORT_OVERVIEW_CHAT_LIMIT) ??
    []
  ).map((c) => ({
    chatId: c.id,
    chatNumber: c.number ?? undefined,
    title: c.initialMessage || c.number || "",
    startedTime: c.createdOn ?? "",
    messages: c.messageCount,
    kbArticles: 0,
    status: c.state?.label ?? "Open",
  }));

  const isActuallyLoading = isAuthLoading || isLoading || (!stats && !isError);

  const handleStatClick = (key: string) => {
    const path = supportPath;
    switch (key) {
      case "ongoingCases":
        navigate("cases?statusFilter=active", { state: { returnTo: path } });
        break;
      case "activeChats":
        navigate("conversations?statusFilter=active", { state: { returnTo: path } });
        break;
      case "resolvedPast30DaysCasesCount":
        navigate("cases?statusFilter=resolved", { state: { returnTo: path } });
        break;
      case "resolvedChats":
        navigate("conversations?statusFilter=resolvedViaChat", { state: { returnTo: path } });
        break;
    }
  };

  useEffect(() => {
    if (isError) {
      logger.error(`Failed to load support stats for project: ${projectId}`);
    }
  }, [isError, projectId, logger]);

  useEffect(() => {
    if (stats) {
      logger.debug(`Support stats loaded for project: ${projectId}`);
    }
  }, [stats, projectId, logger]);

  return (
    <Stack spacing={3} sx={{ width: "100%", minWidth: 0 }}>
      <CasesOverviewStatCard
        isLoading={isActuallyLoading}
        isError={isError}
        stats={stats}
        onStatClick={handleStatClick}
      />
      <Grid
        container
        spacing={3}
        sx={{ alignItems: "stretch", minWidth: 0, width: "100%", overflowX: "hidden" }}
      >
        <Grid size={{ xs: 12, lg: 6 }} sx={{ display: "flex", minWidth: 0 }}>
          <SupportOverviewCard
            sx={{ flex: 1, width: "100%", minWidth: 0 }}
            title="Outstanding Cases"
            subtitle={`Latest ${SUPPORT_OVERVIEW_CASES_LIMIT} outstanding support tickets`}
            icon={FileText}
            iconVariant={SupportOverviewIconVariant.Orange}
            footerButtons={[
              {
                label: "View my cases",
                onClick: () =>
                  navigate("cases?createdByMe=true", {
                    state: { returnTo: supportPath },
                  }),
              },
              {
                label: "View all cases",
                onClick: () =>
                  navigate("cases", { state: { returnTo: supportPath } }),
              },
            ]}
            isError={combinedIsCasesError}
          >
            <OutstandingCasesList
              cases={cases}
              isLoading={isCasesLoading}
              onCaseClick={
                projectId
                  ? (c) =>
                      navigate(`/projects/${projectId}/support/cases/${c.id}`, {
                        state: { returnTo: supportPath },
                      })
                  : undefined
              }
            />
          </SupportOverviewCard>
        </Grid>
        <Grid size={{ xs: 12, lg: 6 }} sx={{ display: "flex", minWidth: 0 }}>
          <SupportOverviewCard
            sx={{ flex: 1, width: "100%", minWidth: 0 }}
            title="Chat History"
            subtitle="Recent Novera conversations"
            icon={MessageSquare}
            iconVariant={SupportOverviewIconVariant.Blue}
            footerButtons={[
              {
                label: "View my chat history",
                onClick: () =>
                  navigate("conversations?createdByMe=true", {
                    state: { returnTo: supportPath },
                  }),
              },
              {
                label: "View all chat history",
                onClick: () =>
                  navigate("conversations", {
                    state: { returnTo: supportPath },
                  }),
              },
            ]}
            isError={isChatError}
          >
            <ChatHistoryList
              items={chatItems}
              isLoading={isChatLoading}
              onItemAction={
                projectId
                  ? (chatId, action) => {
                      const summary = chatItems.find(
                        (item) => item.chatId === chatId,
                      );

                      if (!summary) {
                        return;
                      }

                      if (action === "resume") {
                        navigate(
                          `/projects/${projectId}/support/chat/${chatId}`,
                          {
                            state: { chatNumber: summary.chatNumber },
                          },
                        );
                      } else {
                        navigate(
                          `/projects/${projectId}/support/conversations/${chatId}`,
                          {
                            state: {
                              conversationSummary: summary,
                              returnTo: supportPath,
                            },
                          },
                        );
                      }
                    }
                  : undefined
              }
            />
          </SupportOverviewCard>
        </Grid>
      </Grid>
    </Stack>
  );
}
