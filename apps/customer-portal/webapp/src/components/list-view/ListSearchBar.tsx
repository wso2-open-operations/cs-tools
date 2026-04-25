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
  Button,
  Divider,
  InputAdornment,
  Paper,
  Skeleton,
  TextField,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  ChevronUp,
  ListFilter,
  Search,
  X,
} from "@wso2/oxygen-ui-icons-react";
import type { ChangeEvent, JSX, ReactNode } from "react";

export interface ListSearchBarProps {
  searchPlaceholder: string;
  searchTerm: string;
  onSearchChange: (value: string) => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  /** Pre-computed count of active filters + non-empty search */
  activeFiltersCount: number;
  onClearFilters: () => void;
  /** JSX rendered inside the collapsible filter section */
  filtersContent: ReactNode;
  /** Show a skeleton placeholder while context is loading */
  isLoading?: boolean;
  /** Hide the filters/clear-filters button entirely */
  hideFiltersButton?: boolean;
}

/**
 * ListSearchBar renders the shared search input and collapsible filter panel
 * used across all list pages.
 *
 * @param {ListSearchBarProps} props - Search and filter configuration.
 * @returns {JSX.Element} The rendered search bar.
 */
export default function ListSearchBar({
  searchPlaceholder,
  searchTerm,
  onSearchChange,
  isFiltersOpen,
  onFiltersToggle,
  activeFiltersCount,
  onClearFilters,
  filtersContent,
  isLoading = false,
  hideFiltersButton = false,
}: ListSearchBarProps): JSX.Element {
  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    onSearchChange(event.target.value);
  };

  const hasActiveFilters = activeFiltersCount > 0;

  if (isLoading) {
    return (
      <Paper sx={{ p: 3, mb: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
          <Skeleton variant="rounded" height={40} sx={{ flex: 1 }} />
          <Skeleton variant="rounded" width={100} height={36} />
        </Box>
      </Paper>
    );
  }

  return (
    <Paper sx={{ p: 3, mb: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
        <Box sx={{ position: "relative", flex: 1 }}>
          <TextField
            fullWidth
            size="small"
            placeholder={searchPlaceholder}
            value={searchTerm}
            onChange={handleSearchChange}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
        </Box>
        {!hideFiltersButton && (
          <Button
            variant="outlined"
            size="small"
            onClick={hasActiveFilters ? onClearFilters : onFiltersToggle}
            startIcon={
              hasActiveFilters ? <X size={16} /> : <ListFilter size={16} />
            }
            endIcon={
              !hasActiveFilters &&
              (isFiltersOpen ? (
                <ChevronUp size={16} />
              ) : (
                <ChevronDown size={16} />
              ))
            }
          >
            {hasActiveFilters
              ? `Clear Filters (${activeFiltersCount})`
              : "Filters"}
          </Button>
        )}
      </Box>

      {isFiltersOpen && (
        <>
          <Divider sx={{ my: 2 }} />
          {filtersContent}
        </>
      )}
    </Paper>
  );
}
