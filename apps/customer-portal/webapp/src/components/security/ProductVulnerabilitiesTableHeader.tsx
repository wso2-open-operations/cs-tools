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
  Typography,
  TextField,
  InputAdornment,
  Divider,
  IconButton,
} from "@wso2/oxygen-ui";
import { Search, ListFilter } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ChangeEvent } from "react";
import ActiveFilters, {
  type ActiveFilterConfig,
} from "@components/common/filter-panel/ActiveFilters";

interface ProductVulnerabilitiesTableHeaderProps {
  searchValue: string;
  onSearchChange: (value: string) => void;
  onFilterIconClick?: () => void;
  activeFiltersCount: number;
  appliedFilters: Record<string, string>;
  filterFields: ActiveFilterConfig[];
  onRemoveFilter: (field: string) => void;
  onClearAll: () => void;
  onUpdateFilter: (field: string, value: string) => void;
}

/**
 * Header for the Product Vulnerabilities table with title, description, search bar, and filters.
 * @returns {JSX.Element}
 */
const ProductVulnerabilitiesTableHeader = ({
  searchValue,
  onSearchChange,
  onFilterIconClick,
  activeFiltersCount,
  appliedFilters,
  filterFields,
  onRemoveFilter,
  onClearAll,
  onUpdateFilter,
}: ProductVulnerabilitiesTableHeaderProps): JSX.Element => {
  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    onSearchChange(e.target.value);
  };

  return (
    <Box
      sx={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        mb: 3,
        flexWrap: "wrap",
        gap: 2,
      }}
    >
      <Box sx={{ flexGrow: 1 }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            width: "100%",
            mb: activeFiltersCount > 0 ? 2 : 0,
          }}
        >
          <Box>
            <Typography variant="h6">Component Analysis</Typography>
            <Typography variant="body2" color="text.secondary">
              Third-party components with known vulnerabilities and remediation status
            </Typography>
          </Box>
        </Box>
        <ActiveFilters
          appliedFilters={appliedFilters}
          filterFields={filterFields}
          onRemoveFilter={onRemoveFilter}
          onClearAll={onClearAll}
          onUpdateFilter={onUpdateFilter}
        />
      </Box>
      <Box
        sx={{
          flexGrow: 1,
          display: "flex",
          gap: 2,
          alignItems: "center",
          width: { xs: "100%", lg: 410 },
          maxWidth: { lg: 410 },
        }}
      >
        <Box sx={{ width: "100%" }}>
          <TextField
            sx={{
              width: "100%",
              "& .MuiInputBase-root": {
                pr: 0.5,
              },
            }}
            value={searchValue}
            onChange={handleSearchChange}
            placeholder="Search CVE or component"
            size="small"
            fullWidth
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
                endAdornment: onFilterIconClick ? (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      height: "100%",
                    }}
                  >
                    <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
                    <IconButton
                      aria-label="Open advanced search"
                      onClick={onFilterIconClick}
                      size="small"
                      sx={{ mx: 0.5 }}
                    >
                      <ListFilter size={16} />
                    </IconButton>
                  </Box>
                ) : null,
              },
            }}
          />
        </Box>
      </Box>
    </Box>
  );
};

export default ProductVulnerabilitiesTableHeader;
