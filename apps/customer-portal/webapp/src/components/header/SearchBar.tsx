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
  Paper,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search, X } from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useRef,
  useEffect,
  useLayoutEffect,
  useCallback,
  useMemo,
  type JSX,
} from "react";
import { createPortal } from "react-dom";
import { useLocation, useNavigate, useParams } from "react-router";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import useGetProjectCases from "@api/useGetProjectCases";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetChangeRequestsInfinite } from "@features/operations/api/useGetChangeRequests";
import type { CaseListItem } from "@features/support/types/cases";
import type { ChangeRequestItem } from "@features/operations/types/changeRequests";
import SearchResultSkeleton from "@components/header/SearchResultSkeleton";
import SearchCaseCard from "@components/header/SearchCaseCard";
import SearchChangeRequestCard from "@components/header/SearchChangeRequestCard";
import { getOperationsNavSegment } from "@features/operations/utils/operationsPages";

import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import error500Svg from "@assets/error/error-500.svg";
import { isS0Case } from "@features/support/utils/support";
import { SortOrder } from "@/types/common";

const SEARCH_DEBOUNCE_MS = 300;
const SEARCH_RESULTS_DROPDOWN_MIN_WIDTH_PX = 320;
const SEARCH_RESULTS_DROPDOWN_VIEWPORT_PADDING_PX = 16;

/**
 * Resolves dropdown size and position from the search field anchor rect.
 * Width matches the search bar when wide; uses a minimum width when the input is narrow.
 *
 * @param {DOMRect} anchorRect - Bounding rect of the search input container.
 * @returns {{ top: number; left: number; width: number }} Dropdown layout in viewport coordinates.
 */
function getSearchResultsDropdownLayout(anchorRect: DOMRect): {
  top: number;
  left: number;
  width: number;
} {
  const viewportWidth = window.innerWidth;
  const maxViewportWidth =
    viewportWidth - SEARCH_RESULTS_DROPDOWN_VIEWPORT_PADDING_PX * 2;
  const width = Math.min(
    Math.max(anchorRect.width, SEARCH_RESULTS_DROPDOWN_MIN_WIDTH_PX),
    maxViewportWidth,
  );
  const maxLeft =
    viewportWidth - width - SEARCH_RESULTS_DROPDOWN_VIEWPORT_PADDING_PX;
  const left = Math.min(
    Math.max(anchorRect.left, SEARCH_RESULTS_DROPDOWN_VIEWPORT_PADDING_PX),
    maxLeft,
  );

  return {
    top: anchorRect.bottom + 8,
    left,
    width,
  };
}

export interface SearchBarProps {
  /** Project ID for case search. When absent, search is disabled. */
  projectId?: string;
  excludeS0?: boolean;
  /** Expands to fill remaining header space beside the project switcher. */
  fillAvailableWidth?: boolean;
}

/**
 * Header search bar with debounced case search and results dropdown.
 *
 * @param {SearchBarProps} props - Component props.
 * @returns {JSX.Element} The SearchBar component.
 */
export default function SearchBar({
  projectId,
  excludeS0 = false,
  fillAvailableWidth = false,
}: SearchBarProps): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId: urlProjectId } = useParams<{ projectId?: string }>();
  const effectiveProjectId = projectId ?? urlProjectId ?? "";

  const [searchValue, setSearchValue] = useState("");
  const debouncedQuery = useDebouncedValue(
    searchValue.trim(),
    SEARCH_DEBOUNCE_MS,
  );
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [dropdownRect, setDropdownRect] = useState<DOMRect | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const baseRequest = useMemo(
    () => ({
      filters: { searchQuery: debouncedQuery },
      sortBy: { field: "createdOn" as const, order: SortOrder.DESC },
    }),
    [debouncedQuery],
  );

  const {
    data,
    isLoading,
    isError,
    fetchNextPage: fetchNextCasesPage,
    hasNextPage: hasNextCasesPage,
    isFetchingNextPage: isFetchingNextCasesPage,
  } = useGetProjectCases(effectiveProjectId, baseRequest, {
    enabled: !!effectiveProjectId && debouncedQuery.length > 0,
  });

  const rawCases = useMemo(
    () => (data?.pages?.flatMap((p) => p.cases ?? []) ?? []) as CaseListItem[],
    [data?.pages],
  );
  const cases = useMemo(
    () => (excludeS0 ? rawCases.filter((c) => !isS0Case(c)) : rawCases),
    [rawCases, excludeS0],
  );

  const { data: filterMetadata } = useGetProjectFilters(effectiveProjectId);

  const changeRequestStateKeys = useMemo(() => {
    const states = filterMetadata?.changeRequestStates ?? [];
    return states.map((s) => Number(s.id)).filter((n) => Number.isFinite(n));
  }, [filterMetadata?.changeRequestStates]);

  const changeRequestSearchRequest = useMemo(
    () => ({
      filters: {
        searchQuery: debouncedQuery,
        stateKeys: changeRequestStateKeys,
      },
    }),
    [debouncedQuery, changeRequestStateKeys],
  );

  const {
    data: changeRequestData,
    isLoading: isChangeRequestLoading,
    isError: isChangeRequestError,
    fetchNextPage: fetchNextChangeRequestsPage,
    hasNextPage: hasNextChangeRequestsPage,
    isFetchingNextPage: isFetchingNextChangeRequestsPage,
  } = useGetChangeRequestsInfinite(effectiveProjectId, changeRequestSearchRequest, {
    enabled:
      !!effectiveProjectId &&
      debouncedQuery.length > 0 &&
      changeRequestStateKeys.length > 0,
  });

  const changeRequests = useMemo(
    () =>
      changeRequestData?.pages?.flatMap((p) => p.changeRequests ?? []) ?? [],
    [changeRequestData?.pages],
  );

  const handleCaseClick = useCallback(
    (caseItem: CaseListItem) => {
      if (effectiveProjectId) {
        navigate(
          `/projects/${effectiveProjectId}/support/cases/${caseItem.id}`,
        );
        setSearchValue("");
        setIsDropdownOpen(false);
      }
    },
    [effectiveProjectId, navigate],
  );

  const handleChangeRequestClick = useCallback(
    (item: ChangeRequestItem) => {
      if (!effectiveProjectId) return;
      const navSegment = getOperationsNavSegment(location.pathname);
      navigate(
        `/projects/${effectiveProjectId}/${navSegment}/change-requests/${item.id}`,
      );
      setSearchValue("");
      setIsDropdownOpen(false);
    },
    [effectiveProjectId, location.pathname, navigate],
  );

  const dropdownRef = useRef<HTMLDivElement | null>(null);

  const handleDropdownScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      const { scrollHeight, scrollTop, clientHeight } = event.currentTarget;
      const distanceFromBottom = scrollHeight - scrollTop - clientHeight;
      if (distanceFromBottom > 24) return;
      if (hasNextCasesPage && !isFetchingNextCasesPage) fetchNextCasesPage();
      if (hasNextChangeRequestsPage && !isFetchingNextChangeRequestsPage)
        fetchNextChangeRequestsPage();
    },
    [
      hasNextCasesPage,
      isFetchingNextCasesPage,
      fetchNextCasesPage,
      hasNextChangeRequestsPage,
      isFetchingNextChangeRequestsPage,
      fetchNextChangeRequestsPage,
    ],
  );

  const isFetchingNextPage =
    isFetchingNextCasesPage || isFetchingNextChangeRequestsPage;

  const showDropdown =
    isDropdownOpen && (searchValue.length > 0 || debouncedQuery.length > 0);

  useLayoutEffect(() => {
    if (showDropdown && containerRef.current) {
      const updateRect = () => {
        if (containerRef.current) {
          setDropdownRect(containerRef.current.getBoundingClientRect());
        }
      };
      updateRect();
      window.addEventListener("scroll", updateRect, true);
      window.addEventListener("resize", updateRect);
      return () => {
        window.removeEventListener("scroll", updateRect, true);
        window.removeEventListener("resize", updateRect);
      };
    } else {
      setDropdownRect(null);
    }
  }, [showDropdown]);

  const isAnyLoading = isLoading || isChangeRequestLoading;
  const hasNoResults = cases.length === 0 && changeRequests.length === 0;
  const hasAnyError = isError || isChangeRequestError;
  const shouldShowError = hasAnyError && hasNoResults;

  const dropdownLayout = dropdownRect
    ? getSearchResultsDropdownLayout(dropdownRect)
    : null;

  const dropdownContent = showDropdown && dropdownLayout && (
    <Paper
      data-testid="header-search-dropdown"
      elevation={3}
      onScroll={handleDropdownScroll}
      sx={{
        position: "fixed",
        top: dropdownLayout.top,
        left: dropdownLayout.left,
        width: dropdownLayout.width,
        minWidth: dropdownLayout.width,
        maxHeight: 480,
        overflow: "auto",
        zIndex: 9999,
      }}
    >
      {!effectiveProjectId ? (
        <Box sx={{ p: 3, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            Select a project to search.
          </Typography>
        </Box>
      ) : debouncedQuery.length === 0 ? (
        <Box sx={{ p: 3, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            Type to search cases and change requests...
          </Typography>
        </Box>
      ) : isAnyLoading ? (
        <Box sx={{ p: 2 }}>
          <SearchResultSkeleton count={10} />
        </Box>
      ) : shouldShowError ? (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: 4,
            px: 2,
          }}
        >
          <img
            src={error500Svg}
            alt=""
            aria-hidden="true"
            style={{ width: 120, height: "auto", marginBottom: 16 }}
          />
          <Typography variant="body2" color="text.secondary">
            Failed to load search results.
          </Typography>
        </Box>
      ) : hasNoResults ? (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: 4,
            px: 2,
          }}
        >
          <SearchNoResultsIcon
            style={{ width: 200, height: "auto", marginBottom: 16 }}
          />
          <Typography variant="body2" color="text.secondary">
            No results found. Try adjusting your search query.
          </Typography>
        </Box>
      ) : (
        <Box sx={{ p: 2 }}>
          <Stack spacing={2}>
            {cases.map((caseItem) => (
              <SearchCaseCard
                key={`case-${caseItem.id}`}
                caseItem={caseItem}
                onClick={handleCaseClick}
              />
            ))}
            {changeRequests.map((cr) => (
              <SearchChangeRequestCard
                key={`change-request-${cr.id}`}
                changeRequest={cr}
                onClick={handleChangeRequestClick}
              />
            ))}
          </Stack>
          {isFetchingNextPage && (
            <Box sx={{ pt: 2 }}>
              <SearchResultSkeleton count={10} />
            </Box>
          )}
        </Box>
      )}
    </Paper>
  );

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node;
      const isInsideContainer = containerRef.current?.contains(target);
      const isInsideDropdown = dropdownRef.current?.contains(target);
      if (!isInsideContainer && !isInsideDropdown) {
        setIsDropdownOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <Box
      ref={containerRef}
      sx={{
        position: "relative",
        flex: 1,
        minWidth: 0,
        width: "100%",
        maxWidth: fillAvailableWidth
          ? "100%"
          : { xs: "100%", md: 380, lg: 420, xl: 560 },
        ...(!fillAvailableWidth && {
          minWidth: { md: 200, lg: 240, xl: 360 },
        }),
      }}
    >
      <TextField
        data-testid="header-search-input"
        size="small"
        placeholder="What are you looking for?"
        fullWidth
        value={searchValue}
        onChange={(e) => {
          setSearchValue(e.target.value);
          setIsDropdownOpen(true);
        }}
        onFocus={() => searchValue.length > 0 && setIsDropdownOpen(true)}
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={16} />
              </InputAdornment>
            ),
            endAdornment: searchValue ? (
              <InputAdornment position="end">
                <IconButton
                  size="small"
                  edge="end"
                  onClick={() => {
                    setSearchValue("");
                    setIsDropdownOpen(false);
                  }}
                >
                  <X size={16} />
                </IconButton>
              </InputAdornment>
            ) : undefined,
          },
        }}
        sx={{ "& .MuiInputBase-root": { bgcolor: "background.paper" } }}
      />
      {dropdownContent &&
        createPortal(
          <div ref={dropdownRef}>{dropdownContent}</div>,
          document.body,
        )}
    </Box>
  );
}
