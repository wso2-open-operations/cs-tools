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
  Paper,
  TextField,
  InputAdornment,
  Divider,
} from "@wso2/oxygen-ui";
import {
  Search,
  ListFilter,
  ChevronDown,
  ChevronUp,
  X,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX, ChangeEvent } from "react";
import { countListSearchAndFilters } from "@features/support/utils/support";
import AnnouncementsFilters from "@features/announcements/components/AnnouncementsFilters";
import type { AnnouncementsSearchBarProps } from "@features/announcements/types/announcements";
import {
  ANNOUNCEMENTS_FILTERS_BUTTON_LABEL,
  ANNOUNCEMENTS_SEARCH_PLACEHOLDER,
} from "@features/announcements/constants/announcementsConstants";
import { formatAnnouncementsClearFiltersButtonLabel } from "@features/announcements/utils/announcements";

/**
 * Search bar and filters for announcements.
 *
 * @param props - Search, filters, and handlers.
 * @returns {JSX.Element} The rendered search bar.
 */
export default function AnnouncementsSearchBar({
  searchTerm,
  onSearchChange,
  isFiltersOpen,
  onFiltersToggle,
  filters,
  filterMetadata,
  onFilterChange,
  onClearFilters,
  filtersDisabled = false,
}: AnnouncementsSearchBarProps): JSX.Element {
  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    onSearchChange(event.target.value);
  };

  const activeFiltersCount = countListSearchAndFilters(searchTerm, filters);
  const hasActiveFilters = activeFiltersCount > 0;

  return (
    <Paper sx={{ p: 3, mb: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
        <Box sx={{ position: "relative", flex: 1 }}>
          <TextField
            fullWidth
            size="small"
            placeholder={ANNOUNCEMENTS_SEARCH_PLACEHOLDER}
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
            ? formatAnnouncementsClearFiltersButtonLabel(activeFiltersCount)
            : ANNOUNCEMENTS_FILTERS_BUTTON_LABEL}
        </Button>
      </Box>

      {isFiltersOpen && (
        <>
          <Divider sx={{ my: 2 }} />
          <AnnouncementsFilters
            filters={filters}
            filterMetadata={filterMetadata}
            onFilterChange={onFilterChange}
            disabled={filtersDisabled}
          />
        </>
      )}
    </Paper>
  );
}
