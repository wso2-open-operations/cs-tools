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
  Header as HeaderUI,
  MenuItem,
  Paper,
  Popover,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, FolderOpen, Search } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type JSX,
  type MouseEvent,
  type UIEvent,
} from "react";
import useInfiniteProjects, {
  flattenProjectPages,
  getTotalRecords,
} from "@api/useGetProjects";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { SelectMenuLoadMoreRow } from "@components/select-menu-load-more-row/SelectMenuLoadMoreRow";
import { PAGINATED_SELECT_MENU_MAX_HEIGHT_PX } from "@constants/common";
import {
  PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH,
  PROJECT_HUB_PROJECTS_PAGE_SIZE,
  PROJECT_HUB_SEARCH_DEBOUNCE_MS,
  PROJECT_HUB_SEARCH_PLACEHOLDER,
} from "@features/project-hub/constants/projectHubConstants";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";

interface ProjectSwitcherProps {
  projectId?: string;
  onProjectChange: (projectId: string) => void;
  isAuthLoading?: boolean;
}

const LOAD_MORE_THRESHOLD_PX = 24;
const DROPDOWN_SKELETON_COUNT = 5;

/**
 * Project switcher component for the header.
 * Owns its own paginated + searchable projects query.
 * Keeps the dropdown open while search results are loading.
 *
 * @param {ProjectSwitcherProps} props - The props for the component.
 * @returns {JSX.Element} The ProjectSwitcher component.
 */
export default function ProjectSwitcher({
  projectId,
  onProjectChange,
  isAuthLoading = false,
}: ProjectSwitcherProps): JSX.Element {
  const [searchQuery, setSearchQuery] = useState("");
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const isMenuOpen = Boolean(anchorEl);

  const debouncedSearchQuery = useDebouncedValue(
    searchQuery,
    PROJECT_HUB_SEARCH_DEBOUNCE_MS,
  );

  const {
    data,
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

  // Track the true unfiltered total so the single-project check isn't misled by a search-filtered count.
  const [unfilteredTotal, setUnfilteredTotal] = useState(0);
  useEffect(() => {
    if (!debouncedSearchQuery && !isLoading) {
      setUnfilteredTotal(totalRecords);
    }
  }, [debouncedSearchQuery, isLoading, totalRecords]);

  // Persist the last known selected project name across search queries (projects list empties while loading)
  const [lastFound, setLastFound] = useState<{ id: string; name: string } | undefined>(undefined);
  useEffect(() => {
    const found = projects.find((p) => p.id === projectId);
    if (found) setLastFound({ id: found.id, name: found.name });
  }, [projects, projectId]);

  const selectedProject = useMemo(
    () => projects.find((p) => p.id === projectId),
    [projects, projectId],
  );

  // Project details from React Query cache — populated by the dashboard's useGetProjectDetails call
  const { data: projectDetails } = useGetProjectDetails(projectId || "");

  // Best display name: live list → last found while searching → project details API → fallback
  const displayName =
    selectedProject?.name ??
    (lastFound?.id === projectId ? lastFound?.name : undefined) ??
    projectDetails?.name ??
    "Select Project";

  const handleMenuScroll = useCallback(
    (event: UIEvent<HTMLElement>) => {
      const target = event.currentTarget;
      const remaining =
        target.scrollHeight - target.scrollTop - target.clientHeight;
      if (
        remaining < LOAD_MORE_THRESHOLD_PX &&
        hasNextPage &&
        !isFetchingNextPage
      ) {
        fetchNextPage();
      }
    },
    [hasNextPage, isFetchingNextPage, fetchNextPage],
  );

  const handleOpen = useCallback((event: MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  }, []);

  const handleClose = useCallback(() => {
    setAnchorEl(null);
    setSearchQuery("");
  }, []);

  if (!isMenuOpen && (isAuthLoading || isLoading)) {
    return (
      <HeaderUI.Switchers showDivider={false}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            height: 40,
            px: 1.5,
            border: "1px solid",
            borderColor: "action.disabledBackground",
            borderRadius: 1,
          }}
        >
          <FolderOpen size={16} />
          {displayName !== "Select Project" ? (
            <Typography
              variant="body2"
              noWrap
              sx={{
                maxWidth: 220,
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {displayName}
            </Typography>
          ) : (
            <Skeleton variant="rounded" width={150} height={20} />
          )}
        </Box>
      </HeaderUI.Switchers>
    );
  }

  if (!isMenuOpen && isError) {
    return (
      <HeaderUI.Switchers showDivider={false}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 1,
            height: 40,
            width: 200,
            px: 1.5,
            border: "1px solid",
            borderColor: "error.main",
            borderRadius: 1,
            color: "error.main",
          }}
        >
          <ErrorIndicator entityName="Projects" />
        </Box>
      </HeaderUI.Switchers>
    );
  }

  if (!isMenuOpen && !isLoading && unfilteredTotal <= 1) {
    const project = selectedProject ?? projects[0];

    return (
      <HeaderUI.Switchers showDivider={false}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            height: 40,
            minWidth: 200,
            px: 1.5,
            border: "1px solid",
            borderColor: "divider",
            borderRadius: 1,
            backgroundColor: "background.paper",
          }}
        >
          <FolderOpen size={16} />
          <Tooltip title={project ? project.name : "Select Project"}>
            <Typography
              variant="body2"
              noWrap
              sx={{
                maxWidth: 220,
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {project ? project.name : "Select Project"}
            </Typography>
          </Tooltip>
        </Box>
      </HeaderUI.Switchers>
    );
  }

  return (
    <HeaderUI.Switchers showDivider={false}>
      {/* Trigger button — styled to match the other switcher states */}
      <Paper
        onClick={handleOpen}
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          height: 40,
          minWidth: 200,
          px: 1.5,
          cursor: "pointer",
          border: "1px solid",
          borderColor: isMenuOpen ? "primary.main" : "divider",
          userSelect: "none",
          "&:hover": { borderColor: "action.active" },
        }}
      >
        <FolderOpen size={16} />
        <Tooltip title={displayName}>
          <Typography
            variant="body2"
            noWrap
            sx={{
              flex: 1,
              maxWidth: 220,
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {displayName}
          </Typography>
        </Tooltip>
        {isMenuOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
      </Paper>

      {/* Dropdown — Popover gives us full layout control: fixed search bar + scrollable items */}
      <Popover
        open={isMenuOpen}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
        transformOrigin={{ vertical: "top", horizontal: "left" }}
        disableAutoFocus
        disableEnforceFocus
        slotProps={{
          paper: {
            sx: {
              minWidth: anchorEl ? anchorEl.offsetWidth : 200,
              display: "flex",
              flexDirection: "column",
              overflow: "hidden",
              mt: 0.5,
            },
          },
        }}
      >
        {/* Fixed search bar — never scrolls */}
        {unfilteredTotal > PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH && (
          <Box
            sx={{
              flexShrink: 0,
              px: 1,
              pt: 1,
              pb: 0.5,
              borderBottom: "1px solid",
              borderColor: "divider",
              bgcolor: "background.paper",
            }}
          >
            <TextField
              size="small"
              fullWidth
              autoFocus
              placeholder={PROJECT_HUB_SEARCH_PLACEHOLDER}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              InputProps={{
                startAdornment: (
                  <Search size={16} style={{ marginRight: 8, flexShrink: 0 }} />
                ),
              }}
              inputProps={{ autoComplete: "off" }}
              sx={{
                "& input:-webkit-autofill, & input:-webkit-autofill:hover, & input:-webkit-autofill:focus": {
                  WebkitBoxShadow: "0 0 0 100px transparent inset",
                  WebkitTextFillColor: "inherit",
                  transition: "background-color 5000s ease-in-out 0s",
                },
              }}
            />
          </Box>
        )}

        {/* Scrollable items container — sits below the search bar */}
        <Box
          sx={{ maxHeight: PAGINATED_SELECT_MENU_MAX_HEIGHT_PX, overflowY: "auto" }}
          onScroll={handleMenuScroll}
        >
          {/* Skeleton items shown while a new search query loads */}
          {isLoading &&
            [...Array(DROPDOWN_SKELETON_COUNT)].map((_, i) => (
              <Box key={`skel-${i}`} sx={{ px: 2, py: 0.75, width: "100%" }}>
                <Skeleton variant="rounded" width="100%" height={40} />
              </Box>
            ))}

          {/* Project list items */}
          {!isLoading &&
            projects.map((project) => (
              <MenuItem
                key={project.id}
                selected={project.id === projectId}
                onClick={() => {
                  onProjectChange(project.id);
                  handleClose();
                }}
                sx={{ display: "flex", flexDirection: "column", alignItems: "flex-start" }}
              >
                <Typography variant="body2">{project.name}</Typography>
                <Typography variant="caption" color="text.secondary">
                  {project.key}
                </Typography>
              </MenuItem>
            ))}

          {/* Spinner row while the next page loads on scroll */}
          {!isLoading && (
            <SelectMenuLoadMoreRow
              visible={Boolean(isFetchingNextPage && projects.length > 0)}
            />
          )}

          {/* Empty search result */}
          {!isLoading &&
            !isFetchingNextPage &&
            projects.length === 0 &&
            Boolean(debouncedSearchQuery) && (
              <Box sx={{ px: 2, py: 1.5, textAlign: "center" }}>
                <Typography variant="body2" color="text.secondary">
                  No projects found
                </Typography>
              </Box>
            )}
        </Box>
      </Popover>
    </HeaderUI.Switchers>
  );
}
