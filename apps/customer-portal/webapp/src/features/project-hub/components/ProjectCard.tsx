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
import { type JSX, useEffect } from "react";
import { useNavigate } from "react-router";
import { useLoader } from "@context/linear-loader/LoaderContext";
import ProjectCardActions from "@features/project-hub/components/project-card/ProjectCardActions";
import ProjectCardBadges from "@features/project-hub/components/project-card/ProjectCardBadges";
import ProjectCardInfo from "@features/project-hub/components/project-card/ProjectCardInfo";
import ProjectCardStats from "@features/project-hub/components/project-card/ProjectCardStats";
import { setLastSelectedProjectId } from "@features/settings/utils/settingsStorage";
import type { ProjectCardProps } from "@features/project-hub/types/projectHub";
import { ProjectClosureState } from "@/types/permission";

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
  date,
  activeCasesCount,
  activeChatsCount,
  actionRequiredCount,
  closureState,
  onViewDashboard,
}: ProjectCardProps): JSX.Element {
  const navigate = useNavigate();
  const { hideLoader } = useLoader();
  const isSuspended = closureState === ProjectClosureState.SUSPENDED;

  const handleViewDashboard = () => {
    setLastSelectedProjectId(id);
    if (onViewDashboard) {
      onViewDashboard();
    } else {
      navigate(`/projects/${id}/dashboard`);
    }
  };

  useEffect(() => {
    hideLoader();
  }, [hideLoader]);

  return (
    <Form.CardButton
      onClick={handleViewDashboard}
      sx={{
        alignItems: "stretch",
        display: "flex",
        flexDirection: "column",
        width: "100%",
        minHeight: 260,
        opacity: isSuspended ? 0.75 : 1,
        position: "relative",
      }}
    >
      {/* project card badges */}
      <ProjectCardBadges projectKey={projectKey} />
      {/* project card info */}
      <ProjectCardInfo title={title} />
      {/* project card stats */}
      <ProjectCardStats
        activeChatsCount={activeChatsCount}
        date={date}
        activeCasesCount={activeCasesCount}
        actionRequiredCount={actionRequiredCount}
      />
      {/* project card actions */}
      <ProjectCardActions onViewDashboard={handleViewDashboard} />
    </Form.CardButton>
  );
}
