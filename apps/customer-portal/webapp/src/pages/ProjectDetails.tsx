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

import { Box, Typography, Grid } from "@wso2/oxygen-ui";
import { useParams, useOutletContext } from "react-router";
import { useState, useEffect, type JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";
import TabBar from "@components/common/tab-bar/TabBar";
import { PROJECT_DETAILS_TABS } from "@constants/projectDetailsConstants";
import ProjectInformationCard from "@components/project-details/project-overview/project-information/ProjectInformationCard";
import ProjectUsersTab from "@components/project-details/users/ProjectUsersTab";
import ProjectStatisticsCard from "@components/project-details/project-overview/project-statistics/ProjectStatisticsCard";
import ContactInfoCard from "@components/project-details/project-overview/contact-info/ContactInfoCard";
import RecentActivityCard from "@components/project-details/project-overview/recent-activity/RecentActivityCard";
import TimeTrackingStatCards from "@components/project-details/time-tracking/TimeTrackingStatCards";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useGetProjectStat } from "@api/useGetProjectStat";
import useGetProjectTimeTrackingStat from "@api/useGetProjectTimeTrackingStat";
import { useLogger } from "@hooks/useLogger";
import { useLoader } from "@context/linear-loader/LoaderContext";

/**
 * ProjectDetails component.
 *
 * @returns {JSX.Element} The ProjectDetails component.
 */
export default function ProjectDetails(): JSX.Element {
  const [activeTab, setActiveTab] = useState<string>("overview");

  const logger = useLogger();
  const { showLoader, hideLoader } = useLoader();
  const { isLoading: isAuthLoading } = useAsgardeo();

  const { sidebarCollapsed } = useOutletContext<{
    sidebarCollapsed: boolean;
  }>() || { sidebarCollapsed: false };

  const { projectId } = useParams<{ projectId: string }>();

  const {
    data: project,
    isFetching: isProjectFetching,
    error: projectError,
  } = useGetProjectDetails(projectId || "");

  const {
    data: stats,
    isFetching: isStatsFetching,
    error: statsError,
  } = useGetProjectStat(projectId || "");

  const {
    data: timeTrackingStats,
    isFetching: isTimeTrackingFetching,
    error: timeTrackingError,
  } = useGetProjectTimeTrackingStat(projectId || "");

  const isDetailsLoading =
    isAuthLoading ||
    isProjectFetching ||
    isStatsFetching ||
    (!project && !projectError) ||
    (!stats && !statsError);

  useEffect(() => {
    if (isDetailsLoading) {
      showLoader();
    } else {
      hideLoader();
    }
    return () => hideLoader();
  }, [isDetailsLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (projectError) {
      logger.error("Error loading project details:", projectError);
    }
    if (statsError) {
      logger.error("Error loading project stats:", statsError);
    }
  }, [projectError, statsError, logger]);

  const renderContent = () => {
    switch (activeTab) {
      case "overview":
        if (!projectId) {
          return (
            <Box sx={{ p: 3, textAlign: "center" }}>
              <Typography color="error">
                Invalid Project ID. Please check the URL.
              </Typography>
            </Box>
          );
        }

        return (
          <Box>
            <Grid container spacing={3}>
              <Grid size={{ xs: 12, md: 6 }}>
                <ProjectInformationCard
                  project={project}
                  slaStatus={stats?.projectStats?.slaStatus || "--"}
                  isLoading={(isDetailsLoading || !project) && !projectError}
                  isError={!!projectError}
                />
              </Grid>
              <Grid size={{ xs: 12, md: 6 }}>
                <ProjectStatisticsCard
                  stats={stats?.projectStats}
                  isLoading={(isDetailsLoading || !stats) && !statsError}
                  isError={!!statsError}
                  isSidebarOpen={!sidebarCollapsed}
                />
              </Grid>
              <Grid size={{ xs: 12, md: 6 }}>
                <ContactInfoCard />
              </Grid>
              <Grid size={{ xs: 12, md: 6 }}>
                <RecentActivityCard
                  activity={stats?.recentActivity}
                  isLoading={(isDetailsLoading || !stats) && !statsError}
                  isError={!!statsError}
                />
              </Grid>
            </Grid>
          </Box>
        );
      case "deployments":
        return (
          <Box sx={{ p: 3, textAlign: "center" }}>
            <Typography variant="h6" color="text.secondary">
              Deployments (Coming Soon)
            </Typography>
          </Box>
        );
      case "time-tracking":
        return (
          <Box>
            <TimeTrackingStatCards
              stats={timeTrackingStats}
              isLoading={
                isTimeTrackingFetching ||
                (!timeTrackingStats && !timeTrackingError)
              }
              isError={!!timeTrackingError}
            />
          </Box>
        );
      case "users":
        return <ProjectUsersTab projectId={projectId ?? ""} />;
      default:
        return null;
    }
  };

  return (
    <>
      {/* project page tabs */}
      <TabBar
        tabs={PROJECT_DETAILS_TABS}
        activeTab={activeTab}
        onTabChange={setActiveTab}
      />

      {/* project page content */}
      <Box sx={{ flex: 1 }}>{renderContent()}</Box>
    </>
  );
}
