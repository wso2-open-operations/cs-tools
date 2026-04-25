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

import { Box, LinearProgress, TextField, Typography } from "@wso2/oxygen-ui";
import { useEffect, useMemo, useState, type JSX } from "react";
import { useNavigate } from "react-router";
import useInfiniteProjects, {
  flattenProjectPages,
  getTotalRecords,
} from "@api/useGetProjects";
import useGetMetadata from "@api/useGetMetadata";
import { useLogger } from "@hooks/useLogger";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import ProjectCard from "@features/project-hub/components/ProjectCard";
import ProjectCardSkeleton from "@features/project-hub/components/project-card/ProjectCardSkeleton";
import { FolderOpen, Search } from "@wso2/oxygen-ui-icons-react";
import { useAsgardeo } from "@asgardeo/react";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import ApiErrorState from "@components/error/ApiErrorState";
import AccountSuspendedPage from "@/components/access-control/AccountSuspendedPage";
import {
  PROJECT_HUB_EMPTY_DEFAULT_SUBTITLE,
  PROJECT_HUB_EMPTY_DEFAULT_TITLE,
  PROJECT_HUB_EMPTY_SEARCH_SUBTITLE,
  PROJECT_HUB_EMPTY_SEARCH_TITLE,
  PROJECT_HUB_ERROR_SUBTITLE,
  PROJECT_HUB_ERROR_TITLE,
  PROJECT_HUB_PROJECTS_PAGE_SIZE,
  PROJECT_HUB_REDIRECT_LOADER_MESSAGE,
  PROJECT_HUB_SEARCH_DEBOUNCE_MS,
  PROJECT_HUB_SEARCH_PLACEHOLDER,
  PROJECT_HUB_SKELETON_CARD_COUNT,
} from "@features/project-hub/constants/projectHubConstants";
import { ProjectHubContentView } from "@features/project-hub/types/projectHub";
import { ProjectClosureState } from "@/types/permission";
import {
  resolveProjectHubContentView,
  resolveProjectHubHeaderSubtitle,
  resolveProjectHubHeaderTitle,
  shouldHideProjectHubHeaderBlock,
  shouldShowProjectHubSearchBar,
  shouldShowProjectHubSearchOnlyLayout,
} from "@features/project-hub/utils/projectHub";

/**
 * ProjectHub component.
 *
 * @returns {JSX.Element} The ProjectHub component.
 */
export default function ProjectHub(): JSX.Element {
  const logger = useLogger();
  const navigate = useNavigate();
  const { showLoader, hideLoader } = useLoader();
  const { isLoading: isAuthLoading } = useAsgardeo();
  useGetMetadata();
  const [searchQuery, setSearchQuery] = useState<string>("");
  const debouncedSearchQuery = useDebouncedValue(
    searchQuery,
    PROJECT_HUB_SEARCH_DEBOUNCE_MS,
  );

  const {
    data,
    error,
    isLoading,
    isError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteProjects({
    searchQuery: debouncedSearchQuery || undefined,
    pageSize: PROJECT_HUB_PROJECTS_PAGE_SIZE,
  });

  const projects = useMemo(() => flattenProjectPages(data), [data]);
  const totalRecords = getTotalRecords(data);

  useEffect(() => {
    if (
      debouncedSearchQuery &&
      hasNextPage &&
      !isFetchingNextPage &&
      !isLoading &&
      !isError
    ) {
      fetchNextPage();
    }
  }, [
    debouncedSearchQuery,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    isError,
    fetchNextPage,
  ]);

  useEffect(() => {
    if (isLoading) {
      showLoader();
      return () => hideLoader();
    }
  }, [isLoading, showLoader, hideLoader]);

  const allProjectsSuspended =
    !isLoading &&
    !isAuthLoading &&
    !isError &&
    projects.length > 0 &&
    debouncedSearchQuery === "" &&
    hasNextPage === false &&
    !isFetchingNextPage &&
    projects.every((p) => p.closureState === ProjectClosureState.SUSPENDED);

  const isRedirectingToSingleProject =
    !isLoading &&
    !isAuthLoading &&
    !isError &&
    projects.length === 1 &&
    !searchQuery &&
    !allProjectsSuspended;

  useEffect(() => {
    if (isRedirectingToSingleProject) {
      navigate(`/projects/${projects[0].id}/dashboard`, { replace: true });
    }
  }, [
    isRedirectingToSingleProject,
    projects,
    isLoading,
    isAuthLoading,
    isError,
    navigate,
    searchQuery,
  ]);

  useEffect(() => {
    if (isError) {
      logger.error("Failed to load projects in ProjectHub");
    }
  }, [isError, logger]);

  useEffect(() => {
    if (projects.length > 0) {
      logger.debug(`${projects.length} projects loaded in ProjectHub`);
    }
  }, [projects.length, logger]);

  const hasSearchQuery = Boolean(searchQuery.trim());

  const contentView = useMemo(
    () =>
      resolveProjectHubContentView(
        isRedirectingToSingleProject,
        isAuthLoading,
        isLoading,
        isError,
        totalRecords,
        searchQuery,
        projects.length,
      ),
    [
      isAuthLoading,
      isError,
      isLoading,
      isRedirectingToSingleProject,
      projects.length,
      searchQuery,
      totalRecords,
    ],
  );

  const showSearchBar = shouldShowProjectHubSearchBar(
    totalRecords,
    searchQuery,
  );
  const showOnlySearchBar = shouldShowProjectHubSearchOnlyLayout(
    totalRecords,
    searchQuery,
    isLoading,
    isAuthLoading,
    isError,
  );
  const hideHeaderBlock = shouldHideProjectHubHeaderBlock(
    isError,
    isLoading,
    isAuthLoading,
    projects.length,
    searchQuery,
  );

  const headerTitle = resolveProjectHubHeaderTitle(
    totalRecords,
    hasSearchQuery,
  );
  const headerSubtitle = resolveProjectHubHeaderSubtitle(
    totalRecords,
    hasSearchQuery,
  );

  const centeredLoader = (message: string): JSX.Element => (
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 2,
      }}
    >
      <LinearProgress
        color="warning"
        sx={{ width: "80%", maxWidth: 400, height: 4 }}
      />
      <Typography variant="body2" color="text.secondary">
        {message}
      </Typography>
    </Box>
  );

  const cardGridWrapperSx = {
    flex: {
      xs: "1 1 100%",
      sm: "0 1 calc(50% - 24px)",
      md: "0 1 calc(33.33% - 24px)",
      lg: "0 1 calc(25% - 24px)",
      xl: "0 1 calc(20% - 24px)",
    },
    maxWidth: {
      xs: "100%",
      sm: 400,
    },
    minWidth: 300,
  };

  const renderMainContent = (): JSX.Element | null => {
    switch (contentView) {
      case ProjectHubContentView.REDIRECT_LOADER:
        return centeredLoader(PROJECT_HUB_REDIRECT_LOADER_MESSAGE);
      case ProjectHubContentView.AUTH_PENDING:
        return null;
      case ProjectHubContentView.LOADING_SKELETONS:
        return (
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              justifyContent: "center",
              gap: 3,
              maxWidth: 1800,
              mx: "auto",
              width: "100%",
            }}
          >
            {[...Array(PROJECT_HUB_SKELETON_CARD_COUNT)].map((_, index) => (
              <Box key={index} sx={cardGridWrapperSx}>
                <ProjectCardSkeleton />
              </Box>
            ))}
          </Box>
        );
      case ProjectHubContentView.ERROR:
        return (
          <Box
            sx={{
              display: "flex",
              justifyContent: "center",
              py: 10,
              px: 2,
            }}
          >
            <ApiErrorState
              error={error}
              fallbackMessage={`${PROJECT_HUB_ERROR_TITLE}\n${PROJECT_HUB_ERROR_SUBTITLE}`}
            />
          </Box>
        );
      case ProjectHubContentView.NO_GRID:
        return null;
      case ProjectHubContentView.EMPTY_STATE:
        return (
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 2,
              py: 10,
            }}
          >
            {hasSearchQuery ? (
              <SearchNoResultsIcon style={{ width: 200, height: "auto" }} />
            ) : (
              <EmptyIcon />
            )}
            <Typography variant={hasSearchQuery ? "subtitle2" : "h4"}>
              {hasSearchQuery
                ? PROJECT_HUB_EMPTY_SEARCH_TITLE
                : PROJECT_HUB_EMPTY_DEFAULT_TITLE}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {hasSearchQuery
                ? PROJECT_HUB_EMPTY_SEARCH_SUBTITLE
                : PROJECT_HUB_EMPTY_DEFAULT_SUBTITLE}
            </Typography>
          </Box>
        );
      case ProjectHubContentView.PROJECT_LIST:
        return (
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              justifyContent: "center",
              gap: 3,
              maxWidth: 1800,
              mx: "auto",
              width: "100%",
            }}
          >
            {projects.map((project) => (
              <Box key={project.id} sx={cardGridWrapperSx}>
                <ProjectCard
                  id={project.id}
                  projectKey={project.key}
                  slaStatus={project.slaStatus}
                  title={project.name}
                  date={project.createdOn}
                  activeCasesCount={project.activeCasesCount}
                  activeChatsCount={project.activeChatsCount}
                  actionRequiredCount={project.actionRequiredCount ?? 0}
                  closureState={project.closureState}
                />
              </Box>
            ))}
          </Box>
        );
      default:
        return null;
    }
  };

  if (allProjectsSuspended) {
    return <AccountSuspendedPage />;
  }

  return (
    <Box
      sx={{
        width: "100%",
        minHeight: "100%",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Box
        sx={{
          width: "100%",
          display: "flex",
          flexDirection: "column",
          flex: 1,
          justifyContent: "center",
          alignItems: "center",
          pt: 2,
          px: 2,
        }}
      >
        {!hideHeaderBlock && (
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              textAlign: "center",
              pb: 1,
            }}
          >
            <Box
              sx={{
                mb: 1,
                display: "flex",
                alignItems: "center",
                gap: 1.5,
              }}
            >
              <FolderOpen size={28} />
              <Typography variant="h4">{headerTitle}</Typography>
            </Box>

            <Typography
              variant="subtitle2"
              color="text.secondary"
              sx={{ maxWidth: 600 }}
            >
              {headerSubtitle}
            </Typography>

            {showSearchBar && (
              <Box
                sx={{
                  mt: 2,
                  width: "100%",
                  maxWidth: 500,
                }}
              >
                <TextField
                  fullWidth
                  placeholder={PROJECT_HUB_SEARCH_PLACEHOLDER}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  InputProps={{
                    startAdornment: (
                      <Search size={20} style={{ marginRight: 8 }} />
                    ),
                  }}
                  size="small"
                />
              </Box>
            )}
          </Box>
        )}
        {!showOnlySearchBar && (
          <Box
            sx={{
              width: "100%",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              py: 1,
            }}
          >
            {renderMainContent()}
          </Box>
        )}
      </Box>
    </Box>
  );
}
