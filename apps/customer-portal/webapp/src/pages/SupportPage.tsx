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
import { Typography, Box, Grid, Stack } from "@wso2/oxygen-ui";
import { FileText, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { useAsgardeo } from "@asgardeo/react";
import CasesOverviewStatCard from "@components/support/cases-overview-stats/CasesOverviewStatCard";
import NoveraChatBanner from "@components/support/novera-ai-assistant/novera-chat-banner/NoveraChatBanner";
import SupportOverviewCard from "@components/support/support-overview-cards/SupportOverviewCard";
import OutstandingCasesList from "@components/support/support-overview-cards/OutstandingCasesList";
import ServiceRequestCard from "@components/support/request-cards/ServiceRequestCard";
import ChangeRequestCard from "@components/support/request-cards/ChangeRequestCard";
import ChatHistoryList from "@components/support/support-overview-cards/ChatHistoryList";
import { useGetProjectSupportStats } from "@api/useGetProjectSupportStats";
import useGetProjectCases from "@api/useGetProjectCases";
import { useGetChatHistory } from "@api/useGetChatHistory";
import { useLogger } from "@hooks/useLogger";
import {
  SUPPORT_OVERVIEW_CASES_LIMIT,
  SUPPORT_OVERVIEW_CHAT_LIMIT,
} from "@constants/supportConstants";

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
    data: stats,
    isFetching,
    isError,
  } = useGetProjectSupportStats(projectId || "");

  const { data, isFetching: isCasesLoading } = useGetProjectCases(
    projectId || "",
    {
      sortBy: { field: "createdOn", order: "desc" },
    },
  );

  const { data: chatHistoryData, isFetching: isChatLoading } =
    useGetChatHistory(projectId || "");

  const { isLoading: isAuthLoading } = useAsgardeo();

  const cases =
    data?.pages?.[0]?.cases?.slice(0, SUPPORT_OVERVIEW_CASES_LIMIT) ?? [];
  const chatItems =
    chatHistoryData?.chatHistory?.slice(0, SUPPORT_OVERVIEW_CHAT_LIMIT) ?? [];

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

  if (isError) {
    return (
      <Box sx={{ mt: 3, textAlign: "center" }}>
        <Typography variant="h6" color="error">
          Error loading support statistics. Please try again later.
        </Typography>
      </Box>
    );
  }

  return (
    <Stack spacing={3}>
      <CasesOverviewStatCard
        isLoading={isActuallyLoading}
        isError={isError}
        stats={stats}
      />
      <NoveraChatBanner />
      <Grid container spacing={3} sx={{ alignItems: "stretch" }}>
        <Grid size={{ xs: 12, lg: 6 }} sx={{ display: "flex" }}>
          <SupportOverviewCard
            title="Outstanding Cases"
            subtitle={`Latest ${SUPPORT_OVERVIEW_CASES_LIMIT} support tickets`}
            icon={FileText}
            iconVariant="orange"
            footerButtonLabel="View all cases"
            onFooterClick={() => navigate("cases")}
          >
            <OutstandingCasesList cases={cases} isLoading={isCasesLoading} />
          </SupportOverviewCard>
        </Grid>
        <Grid size={{ xs: 12, lg: 6 }} sx={{ display: "flex" }}>
          <SupportOverviewCard
            title="Chat History"
            subtitle="Recent Novera conversations"
            icon={MessageSquare}
            iconVariant="blue"
            footerButtonLabel="View all chat history"
            onFooterClick={() => navigate("chat")}
          >
            <ChatHistoryList
              items={chatItems}
              isLoading={isChatLoading}
              onItemAction={(chatId) => {
                navigate("chat", { state: { chatId } });
              }}
            />
          </SupportOverviewCard>
        </Grid>
      </Grid>
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
    </Stack>
  );
}
