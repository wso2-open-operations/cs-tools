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
import { useEffect, type JSX } from "react";
import { Typography, Box } from "@wso2/oxygen-ui";
import CasesOverviewStatCard from "@/components/support/casesOverviewStats/CasesOverviewStatCard";
import NoveraChatBanner from "@/components/support/noveraAIAssistant/noveraChatBanner/NoveraChatBanner";
import { useGetProjectSupportStats } from "@/api/useGetProjectSupportStats";
import { useLogger } from "@/hooks/useLogger";

/**
 * SupportPage component to display case details for a project.
 *
 * @returns {JSX.Element} The rendered Support page.
 */
export default function SupportPage(): JSX.Element {
  const logger = useLogger();

  const { projectId } = useParams<{ projectId: string }>();

  const {
    data: stats,
    isLoading,
    isError,
  } = useGetProjectSupportStats(projectId || "");

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
      <CasesOverviewStatCard isLoading={isLoading} stats={stats} />
      <NoveraChatBanner />
    </Box>
  );
}
