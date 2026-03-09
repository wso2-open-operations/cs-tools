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
  InputAdornment,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
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
import { useNavigate, useParams } from "react-router";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import useGetProjectCases from "@api/useGetProjectCases";
import type { CaseListItem } from "@models/responses";
import AllCasesList from "@components/support/all-cases/AllCasesList";
import AllCasesListSkeleton from "@components/support/all-cases/AllCasesListSkeleton";
import SearchNoResultsIcon from "@components/common/empty-state/SearchNoResultsIcon";
import ErrorStateIcon from "@components/common/error-state/ErrorStateIcon";
import { isS0Case } from "@utils/support";

const SEARCH_DEBOUNCE_MS = 300;

export interface SearchBarProps {
  /** Project ID for case search. When absent, search is disabled. */
  projectId?: string;
  excludeS0?: boolean;
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
}: SearchBarProps): JSX.Element {
  const navigate = useNavigate();
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
      sortBy: { field: "createdOn" as const, order: "desc" as const },
    }),
    [debouncedQuery],
  );

  const { data, isLoading, isError } = useGetProjectCases(
    effectiveProjectId,
    baseRequest,
    {
      enabled: !!effectiveProjectId && debouncedQuery.length > 0,
    },
  );

  const rawCases = useMemo(
    () => (data?.pages?.flatMap((p) => p.cases ?? []) ?? []) as CaseListItem[],
    [data?.pages],
  );
  const cases = useMemo(
    () => (excludeS0 ? rawCases.filter((c) => !isS0Case(c)) : rawCases),
    [rawCases, excludeS0],
  );

  const handleCaseClick = useCallback(
    (caseItem: CaseListItem) => {
      if (effectiveProjectId) {
        navigate(`/${effectiveProjectId}/support/cases/${caseItem.id}`);
        setSearchValue("");
        setIsDropdownOpen(false);
      }
    },
    [effectiveProjectId, navigate],
  );

  const dropdownRef = useRef<HTMLDivElement | null>(null);

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

  const dropdownContent = showDropdown && dropdownRect && (
    <Paper
      data-testid="header-search-dropdown"
      elevation={3}
      sx={{
        position: "fixed",
        top: dropdownRect.bottom + 8,
        left: dropdownRect.left,
        width: dropdownRect.width,
        maxHeight: 480,
        overflow: "auto",
        zIndex: 9999,
      }}
    >
      {!effectiveProjectId ? (
        <Box sx={{ p: 3, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            Select a project to search cases.
          </Typography>
        </Box>
      ) : debouncedQuery.length === 0 ? (
        <Box sx={{ p: 3, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            Type to search cases...
          </Typography>
        </Box>
      ) : isLoading ? (
        <Box sx={{ p: 2 }}>
          <AllCasesListSkeleton />
        </Box>
      ) : isError ? (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: 4,
            px: 2,
          }}
        >
          <ErrorStateIcon
            style={{ width: 120, height: "auto", marginBottom: 16 }}
          />
          <Typography variant="body2" color="text.secondary">
            Failed to load search results.
          </Typography>
        </Box>
      ) : cases.length === 0 ? (
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
            No cases found.
          </Typography>
        </Box>
      ) : (
        <Box sx={{ p: 2 }}>
          <AllCasesList
            cases={cases}
            isLoading={false}
            onCaseClick={handleCaseClick}
          />
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
        minWidth: 460,
        flexShrink: 0,
      }}
    >
      <TextField
        data-testid="header-search-input"
        size="small"
        placeholder="Search cases, tickets, or users"
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
