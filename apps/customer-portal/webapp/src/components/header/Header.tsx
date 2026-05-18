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
import { Box, Header as HeaderUI } from "@wso2/oxygen-ui";
import { useIsStackedHeaderLayout } from "@hooks/useResponsiveLayout";
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
export default function Header({
  onToggleSidebar,
  collapsed = false,
  hideProjectControls = false,
}: HeaderProps): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const logger = useLogger();
  const { projectId } = useParams<{
    projectId?: string;
  }>();
  const { isLoading: isAuthLoading } = useAsgardeo();
  const { isProjectSuspended } = useErrorPageContext();
  const isStackedHeader = useIsStackedHeaderLayout();

  const isProjectHub = location.pathname === "/";
  const showProjectToolbar =
    !isProjectHub && !hideProjectControls;
  const showSearchBar =
    showProjectToolbar && !isProjectSuspended;

  const {
    data,
    isLoading: isProjectsLoading,
    isError,
  } = useInfiniteProjects({
    pageSize: 20,
    enabled: !isProjectHub && !hideProjectControls,
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

  const projectSwitcher = (
    <ProjectSwitcher
      projectId={projectId}
      onProjectChange={handleProjectChange}
      isAuthLoading={isAuthLoading || isProjectsLoading}
      stackedHeaderRow={isStackedHeader}
    />
  );

  const searchBar = (
    <SearchBar
      projectId={projectId}
      excludeS0={excludeS0}
      fillAvailableWidth={isStackedHeader}
    />
  );

  return (
    <HeaderUI
      sx={{
        width: "100%",
        maxWidth: "100%",
        overflow: "hidden",
        ...(isStackedHeader && {
          "& .MuiToolbar-root": {
            flexDirection: "column",
            alignItems: "stretch",
            minHeight: "auto",
            py: 1,
            gap: 1,
          },
        }),
      }}
    >
      {isStackedHeader ? (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            width: "100%",
            minWidth: 0,
            gap: 1,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              minWidth: 0,
              gap: 1,
            }}
          >
            {!isProjectHub && !hideProjectControls && (
              <HeaderUI.Toggle
                collapsed={collapsed}
                onToggle={onToggleSidebar}
              />
            )}
            <Box sx={{ minWidth: 0, flex: "0 1 auto" }}>
              <Brand isNavigationDisabled={totalRecords <= 1} />
            </Box>
            <HeaderUI.Spacer />
            <Actions hideGetHelp={hideProjectControls} />
          </Box>
          {!isProjectHub && !hideProjectControls && (
            <Box
              data-testid="header-stacked-controls-row"
              sx={{
                display: "flex",
                alignItems: "center",
                width: "100%",
                minWidth: 0,
                gap: { xs: 1, sm: 1.5 },
                px: { xs: 1, sm: 2 },
                boxSizing: "border-box",
              }}
            >
              <Box
                sx={{
                  flex: showSearchBar
                    ? { xs: "1 1 42%", sm: "1 1 40%", md: "1 1 38%" }
                    : "1 1 100%",
                  minWidth: 0,
                  display: "flex",
                }}
              >
                {projectSwitcher}
              </Box>
              {showSearchBar ? (
                <Box
                  sx={{
                    flex: { xs: "1 1 58%", sm: "1 1 60%", md: "1 1 62%" },
                    minWidth: 0,
                    display: "flex",
                  }}
                >
                  {searchBar}
                </Box>
              ) : null}
            </Box>
          )}
        </Box>
      ) : (
        <>
          {!isProjectHub && !hideProjectControls && (
            <HeaderUI.Toggle collapsed={collapsed} onToggle={onToggleSidebar} />
          )}
          <Box sx={{ minWidth: 0, flex: "0 1 auto" }}>
            <Brand isNavigationDisabled={totalRecords <= 1} />
          </Box>
          {!isProjectHub && !hideProjectControls && (
            <>
              {projectSwitcher}
              {showSearchBar ? (
                <HeaderUI.Switchers
                  showDivider={false}
                  sx={{
                    flex: "1 1 auto",
                    minWidth: 0,
                    maxWidth: { lg: 640, xl: 720 },
                    justifyContent: "flex-end",
                  }}
                >
                  {searchBar}
                </HeaderUI.Switchers>
              ) : null}
            </>
          )}
          <HeaderUI.Spacer />
          <Actions hideGetHelp={hideProjectControls} />
        </>
      )}
    </HeaderUI>
  );
}
