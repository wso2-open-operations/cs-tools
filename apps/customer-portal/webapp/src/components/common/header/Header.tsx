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
import useGetProjects from "@/api/useGetProjects";
import { useLogger } from "@/hooks/useLogger";
import type { ProjectListItem } from "@/models/responses";
import Brand from "@/components/common/header/Brand";
import Actions from "@/components/common/header/Actions";
import SearchBar from "@/components/common/header/SearchBar";
import ProjectSwitcher from "@/components/common/header/ProjectSwitcher";

/**
 * Props for the Header component.
 */
interface HeaderProps {
  /**
   * Callback function to toggle the sidebar.
   */
  onToggleSidebar: () => void;
}

/**
 * Header component for the application.
 *
 * @param {HeaderProps} props - The props for the component.
 * @returns {JSX.Element} The Header component.
 */
export default function Header({ onToggleSidebar }: HeaderProps): JSX.Element {
  /**
   * Navigation hook.
   */
  const navigate = useNavigate();
  /**
   * Location hook.
   */
  const location = useLocation();
  /**
   * Logger hook.
   */
  const logger = useLogger();
  /**
   * Project ID from URL parameters.
   */
  const { projectId } = useParams<{
    projectId?: string;
  }>();

  /**
   * Check if the current location is the project hub.
   */
  const isProjectHub = location.pathname === "/";

  /**
   * Fetch projects.
   */
  const {
    data: projectsResponse,
    fetchNextPage,
    hasNextPage,
    isLoading,
    isFetchingNextPage,
    isError,
  } = useGetProjects({}, true);

  /**
   * Fetch next page of projects if available.
   */
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage && !isError) {
      fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, isError, fetchNextPage]);

  /**
   * Flatten the projects response.
   */
  const projects = useMemo(
    () => projectsResponse?.pages.flatMap((page) => page.projects) || [],
    [projectsResponse?.pages],
  );

  /**
   * Find the project from the URL parameters.
   */
  const projectFromUrl = projects.find((project) => project.id === projectId);

  /**
   * Use effect to log errors when they occur.
   */
  useEffect(() => {
    if (isError) {
      logger.error("Failed to fetch projects in Header");
    }
  }, [isError, logger]);

  /**
   * Use effect to log projects loaded.
   */
  useEffect(() => {
    if (projects.length > 0) {
      logger.debug(`${projects.length} projects loaded in Header`);
    }
  }, [projects.length, logger]);

  /**
   * State for the selected project.
   */
  const [selectedProject, setProject] = useState<ProjectListItem | undefined>(
    projectFromUrl,
  );

  /**
   * Effect to update the selected project when the URL parameters change.
   */
  useEffect(() => {
    if (projectId) {
      /**
       * Find the project from the URL parameters.
       */
      const project = projects.find((p) => p.id === projectId);
      /**
       * Set the selected project if it is different from the current selected project.
       * If no matching project is found, clear the selection.
       */
      if (project) {
        if (project.id !== selectedProject?.id) {
          setProject(project);
        }
      } else {
        setProject(undefined);
      }
    } else if (selectedProject) {
      /**
       * If projectId is missing (e.g., on the project hub), clear the selection.
       */
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
        /**
         * Set the selected project.
         */
        setProject(project);
        /**
         * Get the sub path from the current location.
         */
        const subPath = location.pathname.split("/").slice(2).join("/");
        /**
         * Navigate to the new project.
         */
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
      <Brand />
      {!isProjectHub && (
        <>
          {/* header project switcher */}
          <ProjectSwitcher
            projects={projects}
            selectedProject={selectedProject}
            onProjectChange={handleProjectChange}
            isLoading={isLoading}
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
