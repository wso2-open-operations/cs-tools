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
  Button,
} from "@wso2/oxygen-ui";
import {
  Search,
  ListFilter,
  ChevronDown,
  ChevronUp,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ChangeEvent } from "react";

interface ProductVulnerabilitiesTableHeaderProps {
  searchValue: string;
  onSearchChange: (value: string) => void;
  onFilterToggle: () => void;
  isFiltersOpen: boolean;
  activeFiltersCount: number;
}

/**
 * Header for the Product Vulnerabilities table with title, description, search bar, and filters.
 * @returns {JSX.Element}
 */
const ProductVulnerabilitiesTableHeader = ({
  searchValue,
  onSearchChange,
  onFilterToggle,
  isFiltersOpen,
  activeFiltersCount,
}: ProductVulnerabilitiesTableHeaderProps): JSX.Element => {
  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    onSearchChange(e.target.value);
  };

  const hasActiveFilters = activeFiltersCount > 0;

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          mb: 3,
          flexWrap: "wrap",
          rowGap: 2,
          columnGap: { xs: 3, sm: 4, md: 6, lg: 8 },
        }}
      >
        <Box
          sx={{
            minWidth: 0,
            flex: { xs: "1 1 100%", lg: "0 1 auto" },
            pr: { lg: 1 },
          }}
        >
          <Typography variant="h6">Component Analysis</Typography>
          <Typography variant="body2" color="text.secondary">
            Third-party components with known vulnerabilities and remediation
            status
          </Typography>
        </Box>
        <Box
          sx={{
            display: "flex",
            gap: 1,
            alignItems: "center",
            width: { xs: "100%", lg: "auto" },
            flex: { xs: "1 1 100%", lg: "1 1 auto" },
            minWidth: 0,
            pl: { xs: 0, lg: 0.5 },
          }}
        >
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <TextField
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
                },
              }}
            />
          </Box>

          <Button
            variant="outlined"
            color="warning"
            size="small"
            onClick={onFilterToggle}
            sx={{
              whiteSpace: "nowrap",
              minWidth: "fit-content",
            }}
            startIcon={
              hasActiveFilters ? (
                <X size={16} />
              ) : (
                <ListFilter size={16} />
              )
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
        </Box>
      </Box>
    </Box>
  );
};

export default ProductVulnerabilitiesTableHeader;
