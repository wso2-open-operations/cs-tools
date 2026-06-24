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
} from "@wso2/oxygen-ui";
import { ArrowLeft, FolderOpen, Search, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import useInfiniteProjects, {
  flattenProjectPages,
  getTotalRecords,
} from "@api/useGetProjects";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import ProjectListTable from "@features/project-hub/components/ProjectListTable";
import { PROJECT_HUB_SEARCH_DEBOUNCE_MS } from "@features/project-hub/constants/projectHubConstants";

const PAGE_SIZE = 25;

/**
 * Full-page projects search for partner users.
 * Navigated to via "View More" on the partner global search page.
 * Pre-fills the search bar from the `?q=` URL parameter.
 */
export default function PartnerProjectsPage(): JSX.Element {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const urlQuery = searchParams.get("q") ?? "";

  const [searchQuery, setSearchQuery] = useState(urlQuery);
  const debouncedSearchQuery = useDebouncedValue(searchQuery, PROJECT_HUB_SEARCH_DEBOUNCE_MS);

  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Sync debounced value back to URL so the link is shareable.
  useEffect(() => {
    const params: Record<string, string> = {};
    if (debouncedSearchQuery.trim()) params.q = debouncedSearchQuery.trim();
    setSearchParams(params, { replace: true });
  }, [debouncedSearchQuery, setSearchParams]);

  const {
    data,
    isLoading,
    isError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteProjects({
    pageSize: PAGE_SIZE,
    searchQuery: debouncedSearchQuery || undefined,
  });

  const projects = useMemo(() => flattenProjectPages(data), [data]);
  const totalRecords = getTotalRecords(data);

  // Infinite scroll: load next page when sentinel enters view.
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

  return (
    <Box
      sx={{
        display: "flex",
        flex: 1,
        flexDirection: "column",
        minHeight: 0,
        overflow: "hidden",
        width: "100%",
      }}
    >
      {/* Header */}
      <Box
        sx={{
          alignItems: "center",
          display: "flex",
          flexShrink: 0,
          gap: 1.5,
          pb: 1.5,
          pt: 2,
          px: 2,
        }}
      >
        <IconButton
          aria-label="Back to search"
          onClick={() => navigate("/")}
          size="small"
        >
          <ArrowLeft size={20} />
        </IconButton>
        <FolderOpen size={22} />
        <Typography sx={{ flex: 1 }} variant="h5">
          Projects
          {!isLoading && (
            <Typography color="text.secondary" component="span" variant="h5">
              {" "}({totalRecords})
            </Typography>
          )}
          {isLoading && (
            <Skeleton
              sx={{ display: "inline-block", ml: 0.5 }}
              variant="text"
              width={36}
            />
          )}
        </Typography>
      </Box>

      {/* Search bar */}
      <Box sx={{ flexShrink: 0, px: 2, pb: 1.5 }}>
        <TextField
          fullWidth
          inputProps={{ autoComplete: "off" }}
          InputProps={{
            endAdornment: searchQuery ? (
              <InputAdornment position="end">
                <IconButton
                  aria-label="Clear search"
                  edge="end"
                  onClick={() => setSearchQuery("")}
                  size="small"
                >
                  <X size={16} />
                </IconButton>
              </InputAdornment>
            ) : undefined,
            startAdornment: <Search size={20} style={{ marginRight: 8 }} />,
          }}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search projects..."
          size="small"
          sx={{
            maxWidth: 560,
            "& input:-webkit-autofill, & input:-webkit-autofill:focus, & input:-webkit-autofill:hover":
              {
                WebkitBoxShadow: "0 0 0 100px transparent inset",
                WebkitTextFillColor: "inherit",
                transition: "background-color 5000s ease-in-out 0s",
              },
          }}
          value={searchQuery}
        />
      </Box>

      {/* Loading progress bar */}
      {isLoading && (
        <LinearProgress
          color="inherit"
          sx={{ flexShrink: 0, height: 2, mx: 2 }}
        />
      )}

      {/* Error state */}
      {isError && !isLoading && (
        <Box sx={{ flex: 1, px: 2, pt: 2 }}>
          <Typography color="text.secondary" variant="body2">
            Failed to load projects. Please try again.
          </Typography>
        </Box>
      )}

      {/* Scrollable table */}
      {!isError && (
        <Box
          ref={scrollContainerRef}
          sx={{
            flex: "1 1 auto",
            overflowX: "hidden",
            overflowY: "auto",
            px: 2,
            pb: 3,
          }}
        >
          <ProjectListTable
            isFetchingNextPage={isFetchingNextPage}
            projects={projects}
          />
          {/* Sentinel triggers the next page when scrolled into view */}
          <Box ref={sentinelRef} sx={{ height: 1 }} />
        </Box>
      )}
    </Box>
  );
}
