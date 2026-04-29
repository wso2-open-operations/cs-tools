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
  ComplexSelect,
  Header as HeaderUI,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { FolderOpen, Search } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useMemo,
  useRef,
  useState,
  type JSX,
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
  const [isMenuOpen, setIsMenuOpen] = useState(false);
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
  const unfilteredTotalRef = useRef(0);
  if (!debouncedSearchQuery) {
    unfilteredTotalRef.current = totalRecords;
  }

  // Persist the last known selected project name across search queries (projects list empties while loading)
  const lastFoundRef = useRef<{ id: string; name: string } | undefined>(undefined);
  const selectedProject = useMemo(() => {
    const found = projects.find((p) => p.id === projectId);
    if (found) lastFoundRef.current = { id: found.id, name: found.name };
    return found;
  }, [projects, projectId]);

  // Project details from React Query cache — populated by the dashboard's useGetProjectDetails call
  const { data: projectDetails } = useGetProjectDetails(projectId || "");

  // Best display name: live list → last found while searching → project details API → fallback
  const displayName =
    selectedProject?.name ??
    (lastFoundRef.current?.id === projectId
      ? lastFoundRef.current?.name
      : undefined) ??
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

  const handleOpen = useCallback(() => setIsMenuOpen(true), []);
  const handleClose = useCallback(() => {
    setIsMenuOpen(false);
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
            borderRadius: 0,
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
            borderRadius: 0,
            color: "error.main",
          }}
        >
          <ErrorIndicator entityName="Projects" />
        </Box>
      </HeaderUI.Switchers>
    );
  }

  if (!isMenuOpen && !isLoading && unfilteredTotalRef.current <= 1) {
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
            borderRadius: 0,
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
      <ComplexSelect
        value={projectId || ""}
        onChange={(event: any) => onProjectChange(event.target.value)}
        onOpen={handleOpen}
        onClose={handleClose}
        size="small"
        sx={{
          minWidth: 200,
          "& .MuiOutlinedInput-root": {
            "& fieldset": { borderColor: "divider" },
            "&:hover fieldset": { borderColor: "action.active" },
            "&.Mui-focused fieldset": { borderColor: "primary.main" },
          },
        }}
        MenuProps={{
          PaperProps: {
            sx: { overflow: "hidden" },
          },
          MenuListProps: {
            sx: { p: 0, overflow: "hidden" },
          },
        }}
        renderValue={() => (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <FolderOpen size={16} />
            <Tooltip title={displayName}>
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
            </Tooltip>
          </Box>
        )}
      >
        {/* Search box — non-scrollable header, stops events from reaching the Select */}
        <Box
          sx={{
            bgcolor: "background.paper",
            px: 1,
            pt: 1,
            pb: 0.5,
            borderBottom: "1px solid",
            borderColor: "divider",
          }}
          onKeyDown={(e) => e.stopPropagation()}
          onMouseDown={(e) => e.stopPropagation()}
          onClick={(e) => e.stopPropagation()}
        >
          <TextField
            size="small"
            fullWidth
            placeholder={PROJECT_HUB_SEARCH_PLACEHOLDER}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            InputProps={{
              startAdornment: (
                <Search size={16} style={{ marginRight: 8, flexShrink: 0 }} />
              ),
            }}
          />
        </Box>

        {/* Scrollable items container — sits below the search bar */}
        <Box
          sx={{ maxHeight: PAGINATED_SELECT_MENU_MAX_HEIGHT_PX, overflowY: "auto" }}
          onScroll={handleMenuScroll}
        >
          {/* Skeleton items shown while a new search query loads (keeps dropdown open) */}
          {isLoading &&
            [...Array(DROPDOWN_SKELETON_COUNT)].map((_, i) => (
              <Box key={`skel-${i}`} sx={{ px: 2, py: 0.75, width: "100%" }}>
                <Skeleton variant="rounded" width="100%" height={40} />
              </Box>
            ))}

          {/* Project list items */}
          {!isLoading &&
            projects.map((project) => (
              <ComplexSelect.MenuItem key={project.id} value={project.id}>
                <ComplexSelect.MenuItem.Text
                  primary={project.name}
                  secondary={project.key}
                />
              </ComplexSelect.MenuItem>
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
      </ComplexSelect>
    </HeaderUI.Switchers>
  );
}
