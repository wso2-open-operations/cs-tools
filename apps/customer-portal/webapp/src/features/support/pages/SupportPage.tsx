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
import useGetProjectCases from "@api/useGetProjectCases";
import { useSearchConversations } from "@features/support/api/useSearchConversations";
import { useLogger } from "@hooks/useLogger";
import {
  SUPPORT_OVERVIEW_CASES_LIMIT,
  SUPPORT_OVERVIEW_CHAT_LIMIT,
  CaseType,
} from "@features/support/constants/supportConstants";
import { getProjectPermissions } from "@utils/permission";
import { isClosedLikeCaseStatus, isS0Case } from "@features/support/utils/support";
import { SortOrder } from "@/types/common";
import type { ChatHistoryItem } from "@features/support/types/conversations";

/**
 * SupportPage component to display case details for a project.
 *
 * @returns {JSX.Element} The rendered Support page.
 */
export default function SupportPage(): JSX.Element {
  const logger = useLogger();
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const supportPath = `/projects/${projectId}/support`;

  const { data: project } = useGetProjectDetails(projectId || "");
  const includeS0InSupportMetrics = getProjectPermissions(
    project?.type?.label,
  ).includeS0InSupportMetrics;

  const {
    data: stats,
    isLoading,
    isError,
  } = useGetProjectSupportStats(projectId || "", {
    caseTypes: [CaseType.DEFAULT_CASE],
  });

  const {
    data,
    isLoading: isCasesLoading,
    isError: isCasesError,
  } = useGetProjectCases(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.DEFAULT_CASE],
      },
      sortBy: { field: "createdOn", order: SortOrder.DESC },
    },
    {
      enabled: !!projectId,
    },
  );

  const {
    data: conversationsData,
    isLoading: isChatLoading,
    isError: isChatError,
  } = useSearchConversations(projectId || "", {
    pagination: { limit: SUPPORT_OVERVIEW_CHAT_LIMIT, offset: 0 },
    sortBy: { field: "updatedOn", order: SortOrder.DESC },
  });

  const { isLoading: isAuthLoading } = useAsgardeo();

  const rawCases =
    data?.pages?.[0]?.cases
      ?.filter((c) => !isClosedLikeCaseStatus(c.status?.label))
      .slice(0, SUPPORT_OVERVIEW_CASES_LIMIT) ?? [];
  const cases = !includeS0InSupportMetrics
    ? rawCases.filter((c) => !isS0Case(c))
    : rawCases;

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
            isError={isCasesError}
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
        <Grid size={{ xs: 12, lg: 6 }} sx={{ display: "flex" }}>
          <SupportOverviewCard
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
