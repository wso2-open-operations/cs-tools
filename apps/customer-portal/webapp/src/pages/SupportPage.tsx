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

import { useParams, useNavigate } from "react-router";
import { useEffect, type JSX } from "react";
import { Box, Grid, Stack } from "@wso2/oxygen-ui";
import { FileText, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { useAsgardeo } from "@asgardeo/react";
import CasesOverviewStatCard from "@components/support/cases-overview-stats/CasesOverviewStatCard";
import SupportOverviewCard from "@components/support/support-overview-cards/SupportOverviewCard";
import OutstandingCasesList from "@components/support/support-overview-cards/OutstandingCasesList";
import ServiceRequestCard from "@components/support/request-cards/ServiceRequestCard";
import ChangeRequestCard from "@components/support/request-cards/ChangeRequestCard";
import ChatHistoryList from "@components/support/support-overview-cards/ChatHistoryList";
import { useGetProjectSupportStats } from "@api/useGetProjectSupportStats";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { useSearchConversations } from "@api/useSearchConversations";
import { useLogger } from "@hooks/useLogger";
import {
  SUPPORT_OVERVIEW_CASES_LIMIT,
  SUPPORT_OVERVIEW_CHAT_LIMIT,
  CaseType,
} from "@constants/supportConstants";
import { PROJECT_TYPE_LABELS } from "@constants/projectDetailsConstants";
import { getIncidentAndQueryIds, isS0Case } from "@utils/support";
import type { ChatHistoryItem } from "@models/responses";

/**
 * SupportPage component to display case details for a project.
 *
 * @returns {JSX.Element} The rendered Support page.
 */
export default function SupportPage(): JSX.Element {
  const logger = useLogger();
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const {
    data: project,
    isLoading: isProjectLoading,
  } = useGetProjectDetails(projectId || "");
  const isProjectLoaded = !isProjectLoading && project !== undefined;
  const isManagedCloudSubscription =
    isProjectLoaded &&
    project?.type?.label === PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const { incidentId, queryId } = getIncidentAndQueryIds(
    filterMetadata?.caseTypes,
  );

  const hasFilterIds = !!(incidentId || queryId);

  const {
    data: stats,
    isFetching,
    isError,
  } = useGetProjectSupportStats(
    projectId || "",
    {
      incidentId,
      queryId,
    },
    hasFilterIds,
  );

  const {
    data,
    isFetching: isCasesLoading,
    isError: isCasesError,
  } = useGetProjectCases(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.DEFAULT_CASE],
      },
      sortBy: { field: "createdOn", order: "desc" },
    },
    {
      enabled: !!projectId,
    },
  );

  const {
    data: conversationsData,
    isFetching: isChatLoading,
    isError: isChatError,
  } = useSearchConversations(projectId || "", {
    pagination: { limit: SUPPORT_OVERVIEW_CHAT_LIMIT, offset: 0 },
    sortBy: { field: "updatedOn", order: "desc" },
  });

  const { isLoading: isAuthLoading } = useAsgardeo();

  const rawCases =
    data?.pages?.[0]?.cases?.slice(0, SUPPORT_OVERVIEW_CASES_LIMIT) ?? [];
  const cases =
    isProjectLoaded && !isManagedCloudSubscription
      ? rawCases.filter((c) => !isS0Case(c))
      : rawCases;

  const chatItems: ChatHistoryItem[] = (
    conversationsData?.conversations?.slice(0, SUPPORT_OVERVIEW_CHAT_LIMIT) ??
    []
  ).map((c) => ({
    chatId: c.id,
    title: c.initialMessage || c.number,
    startedTime: c.createdOn,
    messages: c.messageCount,
    kbArticles: 0,
    status: c.state?.label ?? "Open",
  }));

  const isActuallyLoading = isAuthLoading || isFetching || (!stats && !isError);

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
    <Stack spacing={3}>
      <CasesOverviewStatCard
        isLoading={isActuallyLoading}
        isError={isError}
        stats={stats}
      />
      <Grid container spacing={3} sx={{ alignItems: "stretch" }}>
        <Grid size={{ xs: 12, lg: 6 }} sx={{ display: "flex" }}>
          <SupportOverviewCard
            title="Outstanding Cases"
            subtitle={`Latest ${SUPPORT_OVERVIEW_CASES_LIMIT} support tickets`}
            icon={FileText}
            iconVariant="orange"
            footerButtons={[
              {
                label: "View my cases",
                onClick: () => navigate("cases?createdByMe=true"),
              },
              {
                label: "View all cases",
                onClick: () => navigate("cases"),
              },
            ]}
            isError={isCasesError}
          >
            <OutstandingCasesList
              cases={cases}
              isLoading={isCasesLoading}
              onCaseClick={
                projectId
                  ? (c) => navigate(`/${projectId}/support/cases/${c.id}`)
                  : undefined
              }
            />
          </SupportOverviewCard>
        </Grid>
        <Grid size={{ xs: 12, lg: 6 }} sx={{ display: "flex" }}>
          <SupportOverviewCard
            title="Chat History"
            subtitle="Recent Novera conversations"
            icon={MessageSquare}
            iconVariant="blue"
            footerButtons={[
              {
                label: "View my chat history",
                onClick: () => navigate("conversations?createdByMe=true"),
              },
              {
                label: "View all chat history",
                onClick: () => navigate("conversations"),
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
                        navigate(`/${projectId}/support/chat/${chatId}`);
                      } else {
                        navigate(
                          `/${projectId}/support/conversations/${chatId}`,
                          {
                            state: { conversationSummary: summary },
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
      {isProjectLoaded && isManagedCloudSubscription && (
        <Box sx={{ mt: 3 }}>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, lg: 6 }}>
              <ServiceRequestCard />
            </Grid>
            <Grid size={{ xs: 12, lg: 6 }}>
              <ChangeRequestCard />
            </Grid>
          </Grid>
        </Box>
      )}
    </Stack>
  );
}
