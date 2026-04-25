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
import { useParams } from "react-router";
import { useState, useEffect, useMemo, type JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";
import TabBar from "@components/tab-bar/TabBar";
import {
  PROJECT_DETAILS_INVALID_PROJECT_ID_MESSAGE,
  PROJECT_DETAILS_TABS,
} from "@features/project-details/constants/projectDetailsConstants";
import { ProjectDetailsTabId } from "@features/project-details/types/projectDetails";
import { getProjectPermissions } from "@utils/permission";
import ProjectInformationCard from "@features/project-details/components/ProjectInformationCard";
import ProjectStatisticsCard from "@features/project-details/components/ProjectStatisticsCard";
import ContactInfoCard from "@features/project-details/components/project-overview/contact-info/ContactInfoCard";
import ServiceHoursAllocationsCard from "@features/project-details/components/project-overview/service-hours-allocations/ServiceHoursAllocationsCard";
import ProjectDeployments from "@features/project-details/components/deployments/ProjectDeployments";
import ProjectTimeTracking from "@features/project-details/components/time-tracking/ProjectTimeTracking";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import { useGetProjectStat } from "@features/project-details/api/useGetProjectStat";
import useInfiniteProjects, { flattenProjectPages } from "@api/useGetProjects";
import { useLogger } from "@hooks/useLogger";
import { useLoader } from "@context/linear-loader/LoaderContext";
import {
  filterProjectDetailsTabsByPermissions,
  getProjectDetailsLoadingState,
} from "@features/project-details/utils/projectDetailsPage";

/**
 * ProjectDetails component.
 *
 * @returns {JSX.Element} The ProjectDetails component.
 */
export default function ProjectDetails(): JSX.Element {
  const [activeTab, setActiveTab] = useState<string>(
    ProjectDetailsTabId.OVERVIEW,
  );

  const { projectId } = useParams<{ projectId: string }>();

  const logger = useLogger();
  const { showLoader, hideLoader } = useLoader();
  const { isLoading: isAuthLoading } = useAsgardeo();

  const { data: projectsData } = useInfiniteProjects({ enabled: true });
  const allProjects = flattenProjectPages(projectsData);
  const currentProject = useMemo(
    () => allProjects.find((p) => p.id === projectId),
    [allProjects, projectId],
  );

  const {
    data: project,
    isLoading: isProjectLoading,
    error: projectError,
  } = useGetProjectDetails(projectId || "");
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");

  const projectTypeLabel = currentProject?.type?.label ?? project?.type?.label;

  const {
    data: stats,
    isLoading: isStatsLoading,
    error: statsError,
  } = useGetProjectStat(projectId || "");

  const isDetailsLoading =
    isProjectFeaturesLoading ||
    getProjectDetailsLoadingState({
      isAuthLoading,
      isProjectLoading,
      isStatsLoading,
      project,
      projectError,
      stats,
      statsError,
    });

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

  const permissions = useMemo(
    () =>
      getProjectPermissions(projectTypeLabel, {
        projectFeatures,
      }),
    [projectTypeLabel, projectFeatures],
  );

  const visibleTabs = useMemo(
    () =>
      isProjectFeaturesLoading
        ? PROJECT_DETAILS_TABS
        : filterProjectDetailsTabsByPermissions(
            PROJECT_DETAILS_TABS,
            permissions,
          ),
    [isProjectFeaturesLoading, permissions],
  );
  const effectiveTab = useMemo(() => {
    const tabIds = visibleTabs.map((tab) => tab.id);
    if (activeTab && tabIds.includes(activeTab)) {
      return activeTab;
    }
    return ProjectDetailsTabId.OVERVIEW;
  }, [activeTab, visibleTabs]);

  useEffect(() => {
    const tabIds = visibleTabs.map((t) => t.id);
    if (activeTab && !tabIds.includes(activeTab)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- reset when permissions hide current tab
      setActiveTab(ProjectDetailsTabId.OVERVIEW);
    }
  }, [visibleTabs, activeTab]);

  const renderContent = () => {
    switch (effectiveTab) {
      case ProjectDetailsTabId.OVERVIEW:
        if (!projectId) {
          return (
            <Box sx={{ p: 3, textAlign: "center" }}>
              <Typography color="error">
                {PROJECT_DETAILS_INVALID_PROJECT_ID_MESSAGE}
              </Typography>
            </Box>
          );
        }

        return (
          <Box>
            <Grid container spacing={3}>
              <Grid size={{ xs: 12, md: 5 }}>
                <ProjectInformationCard
                  project={project}
                  slaStatus={stats?.projectStats?.slaStatus || "--"}
                  isLoading={(isDetailsLoading || !project) && !projectError}
                  isError={!!projectError}
                />
              </Grid>
              <Grid size={{ xs: 12, md: 7 }}>
                <ProjectStatisticsCard
                  stats={stats?.projectStats}
                  isLoading={(isDetailsLoading || !stats) && !statsError}
                  isError={!!statsError}
                  showDeploymentsStat={permissions.hasDeployments}
                />
              </Grid>
              <Grid size={{ xs: 12, md: 5 }}>
                <ContactInfoCard
                  project={project}
                  isLoading={(isDetailsLoading || !project) && !projectError}
                  isError={!!projectError}
                />
              </Grid>
              {permissions.showServiceHoursAllocationsCard && (
                <Grid size={{ xs: 12, md: 7 }}>
                  <ServiceHoursAllocationsCard
                    project={project}
                    isLoading={(isDetailsLoading || !project) && !projectError}
                    isError={!!projectError}
                  />
                </Grid>
              )}
            </Grid>
          </Box>
        );
      case ProjectDetailsTabId.DEPLOYMENTS:
        return (
          <Box>
            <ProjectDeployments projectId={projectId ?? ""} />
          </Box>
        );
      case ProjectDetailsTabId.TIME_TRACKING:
        return (
          <ProjectTimeTracking
            projectId={projectId || ""}
            project={project}
            isProjectLoading={(isDetailsLoading || !project) && !projectError}
            isProjectError={!!projectError}
          />
        );
      default:
        return null;
    }
  };

  return (
    <Box>
      {/* project page tabs */}
      <TabBar
        tabs={visibleTabs}
        activeTab={effectiveTab}
        onTabChange={setActiveTab}
      />

      {/* project page content */}
      <Box>{renderContent()}</Box>
    </Box>
  );
}
