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

import {
  Box,
  IconButton,
  InputAdornment,
  LinearProgress,
  Skeleton,
  TextField,
  Typography,
  useMediaQuery,
  useTheme,
} from "@wso2/oxygen-ui";
import { createPortal } from "react-dom";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
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
import { ChevronUp, FolderOpen, Search, X } from "@wso2/oxygen-ui-icons-react";
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
} from "@features/project-hub/constants/projectHubConstants";
import { ProjectHubContentView } from "@features/project-hub/types/projectHub";
import { ProjectClosureState } from "@/types/permission";
import {
  resolveProjectHubContentView,
  resolveProjectHubHeaderSubtitle,
  resolveProjectHubHeaderTitle,
  shouldHideProjectHubHeaderBlock,
  shouldShowProjectHubSearchBar,
} from "@features/project-hub/utils/projectHub";

/**
 * ProjectHub component.
 *
 * @returns {JSX.Element} The ProjectHub component.
 */
export default function ProjectHub(): JSX.Element {
  const theme = useTheme();
  const isLgUp = useMediaQuery(theme.breakpoints.up("lg"));
  const isXlUp = useMediaQuery(theme.breakpoints.up("xl"));
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

  // Track the unfiltered total separately so the title never shows a search-result count.
  // Only update while not loading so the skeleton can use the previously known count.
  const totalProjectsRef = useRef(0);
  if (!debouncedSearchQuery && !isLoading) {
    totalProjectsRef.current = totalRecords;
  }
  const titleTotalRecords =
    !isLoading && !debouncedSearchQuery
      ? totalRecords
      : totalProjectsRef.current;

  const [showBackToTop, setShowBackToTop] = useState(false);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const checkScrolled = () => {
      let el: HTMLElement | null = scrollContainerRef.current;
      while (el && el !== document.documentElement) {
        if (el.scrollTop > 200) {
          setShowBackToTop(true);
          return;
        }
        el = el.parentElement;
      }
      setShowBackToTop(false);
    };
    document.addEventListener("scroll", checkScrolled, true);
    return () => document.removeEventListener("scroll", checkScrolled, true);
  }, []);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && hasNextPage && !isFetchingNextPage) {
          fetchNextPage();
        }
      },
      { root: scrollContainerRef.current, rootMargin: "200px" },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  useEffect(() => {
    const allLoadedAreSuspended =
      projects.length > 0 &&
      debouncedSearchQuery === "" &&
      projects.every((p) => p.closureState === ProjectClosureState.SUSPENDED);
    if (allLoadedAreSuspended && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [
    projects,
    debouncedSearchQuery,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
  ]);

  useEffect(() => {
    if (isLoading) {
      showLoader();
      return () => hideLoader();
    }
  }, [isLoading, showLoader, hideLoader]);

  const isCheckingAllSuspended =
    !isLoading &&
    !isAuthLoading &&
    !isError &&
    projects.length > 0 &&
    !debouncedSearchQuery &&
    hasNextPage === true &&
    projects.every((p) => p.closureState === ProjectClosureState.SUSPENDED);

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
    !allProjectsSuspended &&
    !isCheckingAllSuspended;

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
        projects.length,
        isCheckingAllSuspended,
      ),
    [
      isAuthLoading,
      isCheckingAllSuspended,
      isError,
      isLoading,
      isRedirectingToSingleProject,
      projects.length,
    ],
  );

  const showSearchBar = shouldShowProjectHubSearchBar(
    titleTotalRecords,
    searchQuery,
  );
  const hideHeaderBlock = shouldHideProjectHubHeaderBlock(
    isError,
    isLoading,
    isAuthLoading,
    projects.length,
    searchQuery,
  );

  const headerTitle = resolveProjectHubHeaderTitle(titleTotalRecords);
  const headerSubtitle = resolveProjectHubHeaderSubtitle();

  const colsPerRow = isXlUp ? 5 : isLgUp ? 4 : 3;
  // During loading use the cached count so the skeleton mirrors the expected layout.
  const effectiveCount = isLoading ? totalProjectsRef.current : projects.length;
  const isCenteredLayout = effectiveCount <= colsPerRow;
  // When effectiveCount is 0 (first-ever load), fill one full row of skeleton cards.
  const displayCount = effectiveCount === 0 ? colsPerRow : effectiveCount;

  const gridSx = isCenteredLayout
    ? {
        display: "grid",
        gridTemplateColumns: {
          xs: `repeat(${Math.min(displayCount, 3)}, minmax(0, 350px))`,
          lg: `repeat(${Math.min(displayCount, 4)}, minmax(0, 350px))`,
          xl: `repeat(${Math.min(displayCount, 5)}, minmax(0, 350px))`,
        },
        justifyContent: "center",
        gap: 3,
        pt: 1,
        pb: 1,
      }
    : {
        display: "grid",
        gridTemplateColumns: {
          xs: "repeat(3, 1fr)",
          lg: "repeat(4, 1fr)",
          xl: "repeat(5, 1fr)",
        },
        gap: 3,
        width: "100%",
        py: 1,
      };

  const GRID_GHOST_COUNT = 5;
  const gridGhostSx = {
    overflow: "hidden",
    height: 0,
    visibility: "hidden",
  } as const;

  const renderMainContent = (): JSX.Element | null => {
    switch (contentView) {
      case ProjectHubContentView.REDIRECT_LOADER:
        return (
          <Box
            sx={{
              height: "100%",
              minHeight: 300,
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
              {PROJECT_HUB_REDIRECT_LOADER_MESSAGE}
            </Typography>
          </Box>
        );
      case ProjectHubContentView.AUTH_PENDING:
        return null;
      case ProjectHubContentView.LOADING_SKELETONS:
        if (debouncedSearchQuery) {
          return (
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: {
                  xs: "repeat(3, 1fr)",
                  lg: "repeat(4, 1fr)",
                  xl: "repeat(5, 1fr)",
                },
                gap: 3,
                width: "100%",
                py: 1,
              }}
            >
              {[...Array(colsPerRow)].map((_, i) => (
                <ProjectCardSkeleton key={`skeleton-search-${i}`} />
              ))}
              {[...Array(GRID_GHOST_COUNT)].map((_, i) => (
                <Box
                  key={`ghost-search-${i}`}
                  aria-hidden="true"
                  sx={gridGhostSx}
                />
              ))}
            </Box>
          );
        }
        return (
          <Box
            sx={{
              minHeight: 300,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 2,
            }}
          >
            <LinearProgress
              color="inherit"
              sx={{ width: "60%", maxWidth: 400, height: 2 }}
            />
            <Typography variant="body2" color="text.secondary">
              Loading projects...
            </Typography>
          </Box>
        );
      case ProjectHubContentView.ERROR:
        return (
          <Box
            sx={{
              height: "100%",
              minHeight: 300,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
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
              height: "100%",
              minHeight: 300,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 2,
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
      case ProjectHubContentView.PROJECT_LIST: {
        const grid = (
          <Box sx={gridSx}>
            {projects.map((project) => (
              <ProjectCard
                key={project.id}
                id={project.id}
                projectKey={project.key}
                title={project.name}
                date={project.createdOn}
                activeChatsCount={project.activeChatsCount}
                actionRequiredCount={project.actionRequiredCount ?? 0}
                outstandingCount={project.outstandingCount ?? 0}
                closureState={project.closureState}
              />
            ))}
            {isFetchingNextPage && (
              <>
                {[...Array(PROJECT_HUB_PROJECTS_PAGE_SIZE)].map((_, i) => (
                  <ProjectCardSkeleton key={`skeleton-next-${i}`} />
                ))}
                {[...Array(GRID_GHOST_COUNT)].map((_, i) => (
                  <Box
                    key={`ghost-next-${i}`}
                    aria-hidden="true"
                    sx={gridGhostSx}
                  />
                ))}
              </>
            )}
            {/* sentinel: spans all columns so IntersectionObserver detects bottom */}
            <Box ref={sentinelRef} sx={{ gridColumn: "1 / -1", height: 1 }} />
          </Box>
        );

        return grid;
      }
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
        flex: 1,
        minHeight: 0,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
        justifyContent: isCenteredLayout ? "center" : "flex-start",
      }}
    >
      {/* Fixed header: title, subtitle, search bar */}
      {!hideHeaderBlock && (
        <Box
          sx={{
            flexShrink: 0,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            textAlign: "center",
            pt: isCenteredLayout ? 1.5 : 2,
            pb: isCenteredLayout ? 1 : 1,
            px: 2,
          }}
        >
          <Box
            sx={{
              mb: isCenteredLayout ? 0.5 : 1,
              display: "flex",
              alignItems: "center",
              gap: 1.5,
            }}
          >
            <FolderOpen size={28} />
            <Typography
              variant="h4"
              component="div"
              sx={{ display: "flex", alignItems: "center" }}
            >
              {isLoading && !debouncedSearchQuery ? (
                <>
                  Your Projects&nbsp;(
                  <Skeleton
                    variant="text"
                    width={28}
                    sx={{ display: "inline-block" }}
                  />
                  )
                </>
              ) : (
                headerTitle
              )}
            </Typography>
          </Box>

          <Typography
            variant="subtitle2"
            color="text.secondary"
            sx={{ maxWidth: 600 }}
          >
            {headerSubtitle}
          </Typography>

          {showSearchBar && (
            <Box sx={{ mt: 2, width: "100%", maxWidth: 500 }}>
              {isLoading && !debouncedSearchQuery ? (
                <Skeleton variant="rounded" height={40} width="100%" />
              ) : (
                <TextField
                  fullWidth
                  placeholder={PROJECT_HUB_SEARCH_PLACEHOLDER}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  InputProps={{
                    startAdornment: (
                      <Search size={20} style={{ marginRight: 8 }} />
                    ),
                    endAdornment: searchQuery ? (
                      <InputAdornment position="end">
                        <IconButton
                          size="small"
                          aria-label="Clear search"
                          edge="end"
                          onClick={() => setSearchQuery("")}
                        >
                          <X size={16} />
                        </IconButton>
                      </InputAdornment>
                    ) : undefined,
                  }}
                  inputProps={{ autoComplete: "off" }}
                  sx={{
                    "& input:-webkit-autofill, & input:-webkit-autofill:hover, & input:-webkit-autofill:focus":
                      {
                        WebkitBoxShadow: "0 0 0 100px transparent inset",
                        WebkitTextFillColor: "inherit",
                        transition: "background-color 5000s ease-in-out 0s",
                      },
                  }}
                  size="small"
                />
              )}
            </Box>
          )}
        </Box>
      )}

      {/* Scrollable project grid — fills remaining height, triggers pagination on scroll */}
      <Box
        ref={scrollContainerRef}
        sx={{
          flex: isCenteredLayout ? "none" : "1 1 auto",
          minHeight: isCenteredLayout ? 0 : "auto",
          overflowY: "auto",
          overflowX: "hidden",
          px: 2,
          pb: isCenteredLayout ? 0 : 2,
        }}
      >
        {renderMainContent()}
      </Box>

      {showBackToTop &&
        createPortal(
          <IconButton
            aria-label="Back to top"
            onClick={() => {
              let el: HTMLElement | null = scrollContainerRef.current;
              while (el && el !== document.documentElement) {
                if (el.scrollTop > 0)
                  el.scrollTo({ top: 0, behavior: "smooth" });
                el = el.parentElement;
              }
            }}
            sx={{
              position: "fixed",
              bottom: 72,
              right: 24,
              bgcolor: "primary.main",
              color: "primary.contrastText",
              boxShadow: 4,
              zIndex: 1400,
              width: 44,
              height: 44,
              "&:hover": { bgcolor: "primary.dark" },
            }}
          >
            <ChevronUp size={22} />
          </IconButton>,
          document.body,
        )}
    </Box>
  );
}
