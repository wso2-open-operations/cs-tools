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

import { useEffect, type JSX, useMemo, useCallback } from "react";
import { Header as HeaderUI } from "@wso2/oxygen-ui";
import { useNavigate, useLocation, useParams } from "react-router";
import useInfiniteProjects, {
  flattenProjectPages,
  getTotalRecords,
} from "@api/useGetProjects";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import { useLogger } from "@hooks/useLogger";
import Brand from "@components/header/Brand";
import Actions from "@components/header/Actions";
import SearchBar from "@components/header/SearchBar";
import ProjectSwitcher from "@components/header/ProjectSwitcher";
import { useAsgardeo } from "@asgardeo/react";
import { shouldExcludeS0 } from "@utils/permission";
import { setLastSelectedProject } from "@features/settings/utils/settingsStorage";
import { useErrorPageContext } from "@context/error-page/ErrorPageContext";

interface HeaderProps {
  onToggleSidebar: () => void;
  collapsed?: boolean;
  hideProjectControls?: boolean;
}

/**
 * Header component for the application.
 *
 * @param {HeaderProps} props - The props for the component.
 * @returns {JSX.Element} The Header component.
 */
export default function Header({ onToggleSidebar, hideProjectControls = false }: HeaderProps): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const logger = useLogger();
  const { projectId } = useParams<{
    projectId?: string;
  }>();
  const { isLoading: isAuthLoading } = useAsgardeo();
  const { isProjectSuspended } = useErrorPageContext();

  const isProjectHub = location.pathname === "/";

  const { data, isLoading: isProjectsLoading, isError } = useInfiniteProjects({
    pageSize: 20,
    enabled: !isProjectHub,
  });

  // Flatten all pages for selected-project lookup and excludeS0 check
  const projects = useMemo(() => flattenProjectPages(data), [data]);

  useEffect(() => {
    if (isError) {
      logger.error("Failed to fetch projects in Header");
    }
  }, [isError, logger]);

  const totalRecords = getTotalRecords(data);

  useEffect(() => {
    if (projects.length > 0) {
      logger.debug(`${projects.length} projects loaded in Header`);
    }
  }, [projects.length, logger]);

  const selectedProject = useMemo(() => {
    if (!projectId) return undefined;
    return projects.find((p) => p.id === projectId);
  }, [projectId, projects]);
  const { data: projectFeatures } = useGetProjectFeatures(projectId || "");

  useEffect(() => {
    const id = projectId?.trim();
    if (!id) return;
    setLastSelectedProject({ id });
  }, [projectId]);

  const excludeS0 = shouldExcludeS0(selectedProject?.type?.label, {
    projectFeatures,
  });

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
      } else {
        logger.debug(`Switching to project: ${id}`);
      }
      setLastSelectedProject({ id });
      navigate(`/projects/${id}/dashboard`);
    },
    [projects, logger, navigate],
  );

  return (
    <HeaderUI>
      {!isProjectHub && !hideProjectControls && (
        /* header sidebar toggle */
        <HeaderUI.Toggle collapsed={false} onToggle={onToggleSidebar} />
      )}
      {/* header brand logo and title */}
      <Brand isNavigationDisabled={totalRecords <= 1} />
      {!isProjectHub && !hideProjectControls && (
        <>
          {/* header project switcher */}
          <ProjectSwitcher
            projectId={projectId}
            onProjectChange={handleProjectChange}
            isAuthLoading={isAuthLoading || isProjectsLoading}
          />
          {/* header search bar */}
          {!isProjectSuspended && (
            <SearchBar projectId={projectId} excludeS0={excludeS0} />
          )}
        </>
      )}
      {/* header spacer */}
      <HeaderUI.Spacer />
      {/* header action buttons */}
      <Actions />
    </HeaderUI>
  );
}
