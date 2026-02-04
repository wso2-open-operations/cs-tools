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

import { Box, Button, Grid, LinearProgress, Typography } from "@wso2/oxygen-ui";
import { useNavigate, useParams } from "react-router";
import { useEffect, type JSX } from "react";
import { useLogger } from "@/hooks/useLogger";
import { useGetDashboardMockStats } from "@/api/useGetDashboardMockStats";
import { useGetProjectCasesStats } from "@/api/useGetProjectCasesStats";
import { DASHBOARD_STATS } from "@/constants/dashboardConstants";
import { StatCard } from "@/components/dashboard/stats/StatCard";
import { ArrowRight, MessageSquare } from "@wso2/oxygen-ui-icons-react";

/**
 * DashboardPage component to display project-specific statistics and overview.
 *
 * @returns {JSX.Element} The rendered Dashboard page.
 */
export default function DashboardPage(): JSX.Element {
  /**
   * Navigation hook.
   */
  const navigate = useNavigate();

  /**
   * Logger hook.
   */
  const logger = useLogger();

  /**
   * Get project ID from URL.
   */
  const { projectId } = useParams<{ projectId: string }>();

  /**
   * Get dashboard mock stats.
   */
  const {
    data: mockStats,
    isLoading: isMockLoading,
    isError: isErrorMock,
  } = useGetDashboardMockStats(projectId || "");

  /**
   * Get project cases stats.
   */
  const {
    data: casesStats,
    isLoading: isCasesLoading,
    isError: isErrorCases,
  } = useGetProjectCasesStats(projectId || "");

  /**
   * Loading state.
   */
  const isLoading = isMockLoading || isCasesLoading;

  /**
   * Error state.
   */
  const isError = isErrorMock || isErrorCases;

  /**
   * Use effect to log errors when they occur.
   */
  useEffect(() => {
    if (isErrorMock) {
      logger.error(`Failed to load mock stats for project ID: ${projectId}`);
    }
    if (isErrorCases) {
      logger.error(`Failed to load cases stats for project ID: ${projectId}`);
    }
  }, [isErrorMock, isErrorCases, logger, projectId]);

  /**
   * Use effect to log success when data is loaded.
   */
  useEffect(() => {
    if (mockStats && casesStats) {
      logger.debug(`Dashboard data loaded for project ID: ${projectId}`);
    }
  }, [mockStats, casesStats, logger, projectId]);

  /**
   * Handle support click.
   */
  const handleSupportClick = () => {
    if (projectId) {
      navigate(`/${projectId}/support/chat`);
    } else {
      navigate("/");
    }
  };

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      {/* Loading progress bar */}
      {isLoading && (
        <LinearProgress
          color="warning"
          sx={{
            left: "-1.5rem",
            position: "absolute",
            right: "-1.5rem",
            top: "-1.5rem",
            zIndex: 1,
          }}
        />
      )}
      {/* Get support button */}
      <Box
        sx={{
          mb: 3,
          display: "flex",
          alignItems: "center",
          justifyContent: "flex-end",
        }}
      >
        <Button
          variant="contained"
          size="small"
          color="warning"
          startIcon={<MessageSquare size={16} />}
          endIcon={<ArrowRight size={14} />}
          sx={{ px: 2 }}
          onClick={handleSupportClick}
        >
          Get Support
        </Button>
      </Box>
      {/* Dashboard stats grid */}
      {isError ? (
        <Typography variant="h6" color="error" sx={{ mt: 2 }}>
          Error loading dashboard statistics. Please try again later.
        </Typography>
      ) : (
        <Grid container spacing={2} sx={{ mb: 3 }}>
          {DASHBOARD_STATS.map((stat) => {
            const trend = mockStats ? mockStats[stat.id]?.trend : undefined;
            let value: string | number = 0;

            if (casesStats) {
              switch (stat.id) {
                case "totalCases":
                  value = casesStats.totalCases;
                  break;
                case "openCases":
                  value = casesStats.openCases;
                  break;
                case "resolvedCases":
                  value = casesStats.resolvedCases.total;
                  break;
                case "avgResponseTime":
                  value = `${casesStats.averageResponseTime}h`;
                  break;
                default:
                  break;
              }
            }

            return (
              <Grid key={stat.id} size={{ xs: 12, sm: 6, md: 3 }}>
                {/* Stat card for each statistic */}
                <StatCard
                  label={stat.label}
                  value={value}
                  icon={<stat.icon size={20} />}
                  iconColor={stat.iconColor}
                  tooltipText={stat.tooltipText}
                  trend={trend}
                  isLoading={isLoading}
                />
              </Grid>
            );
          })}
        </Grid>
      )}
    </Box>
  );
}
