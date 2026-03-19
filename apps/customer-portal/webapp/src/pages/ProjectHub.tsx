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
import { useLogger } from "@hooks/useLogger";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import ProjectCard from "@components/project-hub/project-card/ProjectCard";
import ProjectCardSkeleton from "@components/project-hub/project-card/ProjectCardSkeleton";
import { FolderOpen, Search } from "@wso2/oxygen-ui-icons-react";
import { useAsgardeo } from "@asgardeo/react";
import EmptyIcon from "@components/common/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/common/empty-state/SearchNoResultsIcon";
import ErrorStateIcon from "@components/common/error-state/ErrorStateIcon";

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
  const [searchQuery, setSearchQuery] = useState<string>("");
  // Use debounce hook
  const debouncedSearchQuery = useDebouncedValue(searchQuery, 300);

  // Fetch projects with infinite query
  const {
    data,
    isLoading,
    isError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteProjects({
    searchQuery: debouncedSearchQuery || undefined,
    pageSize: 20,
  });

  const projects = useMemo(() => flattenProjectPages(data), [data]);
  const totalRecords = getTotalRecords(data);

  // Auto-fetch all pages when searching to show all results
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

  const isRedirectingToSingleProject =
    !isLoading &&
    !isAuthLoading &&
    !isError &&
    projects.length === 1 &&
    !searchQuery;

  // Navigate to dashboard if there is only one project
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

  const renderContent = () => {
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

    if (isRedirectingToSingleProject) {
      return centeredLoader("Redirecting, this may take a moment…");
    }

    if (isAuthLoading) return null;

    if (isLoading) {
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
          {[...Array(3)].map((_, index) => (
            <Box
              key={index}
              sx={{
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
              }}
            >
              <ProjectCardSkeleton />
            </Box>
          ))}
        </Box>
      );
    }

    if (isError) {
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
          <ErrorStateIcon />
          <Typography variant="h4">Something Went Wrong</Typography>
          <Typography variant="subtitle2" color="text.secondary">
            We couldn&apos;t load the data right now. Please try again or
            refresh the page.
          </Typography>
        </Box>
      );
    }

    // If totalRecords > 4 and no search query, don't show anything (search bar is above)
    if (totalRecords > 4 && !searchQuery) {
      return null;
    }

    // If no projects found (either from search or initially)
    if (projects.length === 0) {
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
          {searchQuery ? (
            <SearchNoResultsIcon style={{ width: 200, height: "auto" }} />
          ) : (
            <EmptyIcon />
          )}
          <Typography variant={searchQuery ? "subtitle2" : "h4"}>
            {searchQuery ? "No Projects Found" : "No Projects Yet"}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {searchQuery
              ? "Try adjusting your search query"
              : "Projects will appear here once they are created or assigned to you"}
          </Typography>
        </Box>
      );
    }

    // Show project cards (when totalRecords <= 2 or when searching)
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
        {/* project card wrapper */}
        {projects.map((project) => (
          <Box
            key={project.id}
            sx={{
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
            }}
          >
            <ProjectCard
              id={project.id}
              projectKey={project.key}
              slaStatus={project.slaStatus}
              title={project.name}
              subtitle={project.description}
              date={project.createdOn}
              activeCasesCount={project.activeCasesCount}
              activeChatsCount={project.activeChatsCount}
            />
          </Box>
        ))}
      </Box>
    );
  };

  // Determine whether to show the search bar
  const showSearchBar = totalRecords > 4 || searchQuery;
  const showOnlySearchBar =
    totalRecords > 4 &&
    !searchQuery &&
    !isLoading &&
    !isAuthLoading &&
    !isError;

  // Center content when displaying projects (not search-only view) or loading
  const shouldCenterContent =
    showOnlySearchBar ||
    isLoading ||
    isAuthLoading ||
    (!isLoading &&
      !isAuthLoading &&
      projects.length > 0 &&
      projects.length <= 3);

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
          justifyContent: shouldCenterContent ? "center" : "flex-start",
          pt: 0,
        }}
      >
        {!(
          isError ||
          (!isLoading &&
            !isAuthLoading &&
            projects.length === 0 &&
            !searchQuery)
        ) && (
          <Box
            sx={{
              mb: showOnlySearchBar ? 0 : 3,
              mt: showOnlySearchBar ? -15 : 3,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              textAlign: "center",
            }}
          >
            {/* project hub title */}
            <Box
              sx={{
                mb: 1,
                display: "flex",
                alignItems: "center",
                gap: 1.5,
              }}
            >
              <FolderOpen size={28} />
              <Typography variant="h4">
                {totalRecords > 4 && !searchQuery
                  ? `You have ${totalRecords} projects`
                  : "Select Your Project"}
              </Typography>
            </Box>

            {/* project hub subtitle */}
            <Typography
              variant="subtitle2"
              color="text.secondary"
              sx={{ maxWidth: 600 }}
            >
              {totalRecords > 4 && !searchQuery
                ? "Please use the search bar below to find your project"
                : "Choose a project to access your support cases, chat history, and dashboard"}
            </Typography>

            {/* Search bar - show when totalRecords > 4 */}
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
                  placeholder="Search projects..."
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
              ...(isLoading || isAuthLoading || isRedirectingToSingleProject
                ? {
                    flex: 1,
                    minHeight: 0,
                    display: "flex",
                    flexDirection: "column",
                  }
                : {}),
            }}
          >
            {renderContent()}
          </Box>
        )}
      </Box>
    </Box>
  );
}
