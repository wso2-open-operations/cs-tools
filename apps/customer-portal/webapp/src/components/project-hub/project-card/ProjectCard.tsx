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

import { Form } from "@wso2/oxygen-ui";
import { useMemo, type JSX, useEffect } from "react";
import { useNavigate } from "react-router";
import { useGetProjectStat } from "@api/useGetProjectStat";
import { useLoader } from "@context/linear-loader/LoaderContext";
import ProjectCardActions from "@components/project-hub/project-card/ProjectCardActions";
import ProjectCardBadges from "@components/project-hub/project-card/ProjectCardBadges";
import ProjectCardInfo from "@components/project-hub/project-card/ProjectCardInfo";
import ProjectCardStats from "@components/project-hub/project-card/ProjectCardStats";
import {
  getMockActiveChats,
  getMockOpenCases,
  getMockStatus,
} from "@models/mockFunctions";

// Props for the ProjectCard component.
export interface ProjectCardProps {
  date: string;
  id: string;
  onViewDashboard?: () => void;
  projectKey: string;
  subtitle: string;
  title: string;
}

/**
 * ProjectCard component to display an overview of a project.
 *
 * @param {ProjectCardProps} props - The props for the component.
 * @returns {JSX.Element} The rendered Project Card.
 */
export default function ProjectCard({
  id,
  projectKey,
  title,
  subtitle,
  date,
  onViewDashboard,
}: ProjectCardProps): JSX.Element {
  // Hook to navigate between routes.
  const navigate = useNavigate();
  const { showLoader, hideLoader } = useLoader();

  const handleViewDashboard = () => {
    if (onViewDashboard) {
      onViewDashboard();
    } else {
      navigate(`/${id}/dashboard`);
    }
  };

  // Hook to fetch project statistics.
  const {
    data: statsData,
    isLoading: isStatsLoading,
    isError: isStatsQueryError,
  } = useGetProjectStat(id);

  useEffect(() => {
    if (isStatsLoading) {
      showLoader();
      return () => hideLoader();
    }
  }, [isStatsLoading, showLoader, hideLoader]);

  const mockStatus = useMemo(() => getMockStatus(), []);
  const mockOpenCases = useMemo(() => getMockOpenCases(), []);
  const mockActiveChats = useMemo(() => getMockActiveChats(), []);

  const resolvedStatus = statsData?.projectStats?.slaStatus ?? mockStatus;
  const resolvedOpenCases = statsData?.projectStats?.openCases ?? mockOpenCases;
  const resolvedActiveChats =
    statsData?.projectStats?.activeChats ?? mockActiveChats;

  const resolvedIsStatsError = isStatsQueryError;

  return (
    <Form.CardButton
      onClick={handleViewDashboard}
      sx={{
        alignItems: "stretch",
        display: "flex",
        flexDirection: "column",
        width: "100%",
      }}
    >
      {/* project card badges */}
      <ProjectCardBadges
        projectKey={projectKey}
        status={resolvedStatus}
        isError={resolvedIsStatsError}
        isLoading={isStatsLoading}
      />
      {/* project card info */}
      <ProjectCardInfo subtitle={subtitle} title={title} />
      {/* project card stats */}
      <ProjectCardStats
        activeChats={resolvedActiveChats}
        date={date}
        openCases={resolvedOpenCases}
        isError={resolvedIsStatsError}
        isLoading={isStatsLoading}
      />
      {/* project card actions */}
      <ProjectCardActions onViewDashboard={handleViewDashboard} />
    </Form.CardButton>
  );
}
