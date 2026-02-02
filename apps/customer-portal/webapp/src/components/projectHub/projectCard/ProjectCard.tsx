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
import { useMemo, type JSX } from "react";
import { useNavigate } from "react-router";
import ProjectCardActions from "@/components/projectHub/projectCard/ProjectCardActions";
import ProjectCardBadges from "@/components/projectHub/projectCard/ProjectCardBadges";
import ProjectCardInfo from "@/components/projectHub/projectCard/ProjectCardInfo";
import ProjectCardStats from "@/components/projectHub/projectCard/ProjectCardStats";
import {
  getMockActiveChats,
  getMockOpenCases,
  getMockStatus,
} from "@/models/mockFunctions";

// Props for the ProjectCard component.
export interface ProjectCardProps {
  activeChats?: number;
  date: string;
  id: string;
  onViewDashboard?: () => void;
  openCases?: number;
  projectKey: string;
  status?: string;
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
  status,
  openCases,
  activeChats,
  date,
  onViewDashboard,
}: ProjectCardProps): JSX.Element {
  // Hook to navigate between routes.
  const navigate = useNavigate();

  const handleViewDashboard = () => {
    if (onViewDashboard) {
      onViewDashboard();
    } else {
      navigate(`/${id}/dashboard`);
    }
  };

  const mockStatus = useMemo(() => getMockStatus(), []);
  const mockOpenCases = useMemo(() => getMockOpenCases(), []);
  const mockActiveChats = useMemo(() => getMockActiveChats(), []);
  const resolvedStatus = status ?? mockStatus;
  const resolvedOpenCases = openCases ?? mockOpenCases;
  const resolvedActiveChats = activeChats ?? mockActiveChats;

  return (
    <Form.CardButton
      onClick={handleViewDashboard}
      sx={{
        alignItems: "stretch",
        display: "flex",
        flexDirection: "column",
      }}
    >
      {/* project card badges */}
      <ProjectCardBadges projectKey={projectKey} status={resolvedStatus} />
      {/* project card info */}
      <ProjectCardInfo subtitle={subtitle} title={title} />
      {/* project card stats */}
      <ProjectCardStats
        activeChats={resolvedActiveChats}
        date={date}
        openCases={resolvedOpenCases}
      />
      {/* project card actions */}
      <ProjectCardActions onViewDashboard={handleViewDashboard} />
    </Form.CardButton>
  );
}
