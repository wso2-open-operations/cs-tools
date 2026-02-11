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

import { useState, useEffect, type JSX, useMemo, useCallback } from "react";
import { Header as HeaderUI } from "@wso2/oxygen-ui";
import { useNavigate, useLocation, useParams } from "react-router";
import useGetProjects from "@api/useGetProjects";
import { useLogger } from "@hooks/useLogger";
import type { ProjectListItem } from "@models/responses";
import Brand from "@components/common/header/Brand";
import Actions from "@components/common/header/Actions";
import SearchBar from "@components/common/header/SearchBar";
import ProjectSwitcher from "@components/common/header/ProjectSwitcher";
import { useAsgardeo } from "@asgardeo/react";

interface HeaderProps {
  onToggleSidebar: () => void;
  collapsed?: boolean;
}

/**
 * Header component for the application.
 *
 * @param {HeaderProps} props - The props for the component.
 * @returns {JSX.Element} The Header component.
 */
export default function Header({ onToggleSidebar }: HeaderProps): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const logger = useLogger();
  const { projectId } = useParams<{
    projectId?: string;
  }>();
  const { isLoading: isAuthLoading } = useAsgardeo();

  const isProjectHub = location.pathname === "/";
  const {
    data: projectsResponse,
    isLoading,
    isError,
  } = useGetProjects({}, true);

  const projects = useMemo(
    () => projectsResponse?.projects || [],
    [projectsResponse?.projects],
  );

  const projectFromUrl = projects.find((project) => project.id === projectId);

  useEffect(() => {
    if (isError) {
      logger.error("Failed to fetch projects in Header");
    }
  }, [isError, logger]);

  useEffect(() => {
    if (projects.length > 0) {
      logger.debug(`${projects.length} projects loaded in Header`);
    }
  }, [projects.length, logger]);

  const [selectedProject, setProject] = useState<ProjectListItem | undefined>(
    projectFromUrl,
  );

  useEffect(() => {
    if (projectId) {
      const project = projects.find((p) => p.id === projectId);

      if (project) {
        if (project.id !== selectedProject?.id) {
          setProject(project);
        }
      } else {
        setProject(undefined);
      }
    } else if (selectedProject) {
      setProject(undefined);
    }
  }, [projectId, selectedProject?.id, projects]);

  /**
   * Handles the project change.
   *
   * @param {string} projectId - ID of the project to switch to.
   */
  const handleProjectChange = useCallback(
    (id: string) => {
      const project = projects.find((p) => p.id === id);
      if (project) {
        logger.debug(`Switching to project: ${project.name} (${project.id})`);

        setProject(project);

        const subPath = location.pathname.split("/").slice(2).join("/");

        navigate(`/${project.id}/${subPath || "dashboard"}`);
      } else {
        logger.warn(`Project with ID ${id} not found for switching`);
      }
    },
    [projects, logger, location.pathname, navigate],
  );

  return (
    <HeaderUI>
      {!isProjectHub && (
        /* header sidebar toggle */
        <HeaderUI.Toggle collapsed={false} onToggle={onToggleSidebar} />
      )}
      {/* header brand logo and title */}
      <Brand isNavigationDisabled={projects.length <= 1} />
      {!isProjectHub && (
        <>
          {/* header project switcher */}
          <ProjectSwitcher
            projects={projects}
            selectedProject={selectedProject}
            onProjectChange={handleProjectChange}
            isLoading={isLoading || isAuthLoading}
            isError={isError}
          />
          {/* header search bar */}
          <SearchBar />
        </>
      )}
      {/* header spacer */}
      <HeaderUI.Spacer />
      {/* header action buttons */}
      <Actions />
    </HeaderUI>
  );
}
