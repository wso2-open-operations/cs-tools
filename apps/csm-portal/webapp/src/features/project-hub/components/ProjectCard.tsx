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

import { Card, useTheme } from "@wso2/oxygen-ui";
import { type JSX, type KeyboardEvent, useEffect } from "react";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
import { useLoader } from "@context/linear-loader/LoaderContext";
import ProjectCardActions from "@features/project-hub/components/project-card/ProjectCardActions";
import ProjectCardBadges from "@features/project-hub/components/project-card/ProjectCardBadges";
import ProjectCardInfo from "@features/project-hub/components/project-card/ProjectCardInfo";
import ProjectCardStats from "@features/project-hub/components/project-card/ProjectCardStats";
import { setLastSelectedProject } from "@features/settings/utils/settingsStorage";
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
  activeChatsCount,
  actionRequiredCount,
  outstandingCount,
  closureState,
  onViewDashboard,
}: ProjectCardProps): JSX.Element {
  const navigate = useModifierAwareNavigate();
  const { hideLoader } = useLoader();
  const theme = useTheme();
  const isSuspended = closureState === ProjectClosureState.SUSPENDED;

  const handleViewDashboard = () => {
    setLastSelectedProject({ id });
    if (onViewDashboard) {
      onViewDashboard();
    } else {
      navigate(`/projects/${id}/dashboard`);
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      handleViewDashboard();
    }
  };

  useEffect(() => {
    hideLoader();
  }, [hideLoader]);

  // Renders as a focusable div (not a <button>) so the inner action <Button>
  // is not nested inside another button. Matches StatCard's accessible-card pattern.
  return (
    <Card
      role="button"
      tabIndex={0}
      aria-label={title}
      onClick={handleViewDashboard}
      onKeyDown={handleKeyDown}
      sx={{
        alignItems: "stretch",
        display: "flex",
        flexDirection: "column",
        width: "100%",
        minHeight: 260,
        opacity: isSuspended ? 0.75 : 1,
        position: "relative",
        cursor: "pointer",
        p: 2,
        transition: "box-shadow 0.2s ease, transform 0.15s ease",
        "&:hover": {
          boxShadow: `0 0 0 1px ${theme.palette.primary.main}, 0 4px 16px rgba(0,0,0,0.12)`,
          transform: "translateY(-2px)",
        },
        "&:focus-visible": {
          outline: `2px solid ${theme.palette.primary.main}`,
          outlineOffset: 2,
        },
      }}
    >
      {/* project card badges */}
      <ProjectCardBadges projectKey={projectKey} isSuspended={isSuspended} />
      {/* project card info */}
      <ProjectCardInfo title={title} />
      {/* project card stats */}
      <ProjectCardStats
        activeChatsCount={activeChatsCount}
        date={date}
        outstandingCount={outstandingCount ?? 0}
        actionRequiredCount={actionRequiredCount}
      />
      {/* project card actions */}
      <ProjectCardActions onViewDashboard={handleViewDashboard} />
    </Card>
  );
}
