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
import { Typography, Box, Grid } from "@wso2/oxygen-ui";
import { FileText, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { useAsgardeo } from "@asgardeo/react";
import CasesOverviewStatCard from "@components/support/cases-overview-stats/CasesOverviewStatCard";
import NoveraChatBanner from "@components/support/novera-ai-assistant/novera-chat-banner/NoveraChatBanner";
import SupportOverviewCard from "@components/support/support-overview-cards/SupportOverviewCard";
import { useGetProjectSupportStats } from "@api/useGetProjectSupportStats";
import useGetProjectCases from "@api/useGetProjectCases";
import { useLogger } from "@hooks/useLogger";
import {
  SUPPORT_OVERVIEW_CASES_LIMIT,
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

  const { data: casesData, isFetching: isCasesLoading } = useGetProjectCases(
    projectId || "",
    {
      pagination: { offset: 0, limit: SUPPORT_OVERVIEW_CASES_LIMIT },
      sortBy: { field: "createdOn", order: "desc" },
    },
  );
  const { isLoading: isAuthLoading } = useAsgardeo();

  const cases = casesData?.cases ?? [];
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
    <Box>
      <CasesOverviewStatCard isLoading={isActuallyLoading} stats={stats} />
      <NoveraChatBanner />
      <Grid container spacing={2} sx={{ mt: 3 }}>
        <Grid size={{ xs: 12, lg: 6 }}>
          <SupportOverviewCard
            title="Outstanding Cases"
            subtitle="Latest 5 support tickets"
            icon={FileText}
            iconVariant="orange"
            footerButtonLabel="View all cases"
            onFooterClick={() => navigate("../dashboard")}
          >
            <OutstandingCasesList cases={cases} isLoading={isCasesLoading} />
          </SupportOverviewCard>
        </Grid>
      </Grid>
    </Box>
  );
}
