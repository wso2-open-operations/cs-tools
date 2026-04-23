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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import {
  PRODUCT_VULNERABILITIES_ALL_PRODUCTS_LABEL,
  PRODUCT_VULNERABILITIES_ALL_VERSIONS_LABEL,
  PRODUCT_VULNERABILITIES_CLEAR_FILTERS_LABEL,
  PRODUCT_VULNERABILITIES_PRODUCT_LABEL,
  PRODUCT_VULNERABILITIES_PRODUCT_VERSION_LABEL,
  PRODUCT_VULNERABILITIES_SEVERITY_ALL_LABEL,
  PRODUCT_VULNERABILITIES_SEVERITY_LABEL,
} from "@features/security/constants/securityConstants";
import type { ProductVulnerabilitiesFiltersProps } from "@features/security/types/security";

/**
 * ProductVulnerabilitiesFilters component to display filter dropdowns for
 * severity, product name, and product version.
 *
 * @param {ProductVulnerabilitiesFiltersProps} props - Filter values and change handlers.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function ProductVulnerabilitiesFilters({
  filters,
  severityOptions = [],
  productOptions = [],
  productVersionOptions = [],
  onFilterChange,
  onClearFilters,
}: ProductVulnerabilitiesFiltersProps): JSX.Element {
  const handleSeverityChange = (event: SelectChangeEvent<string | number>) => {
    onFilterChange("severityId", event.target.value);
  };

  const handleProductChange = (event: SelectChangeEvent<string | number>) => {
    // Changing product resets version
    onFilterChange("productVersion", "");
    onFilterChange("productName", event.target.value);
  };

  const handleVersionChange = (event: SelectChangeEvent<string | number>) => {
    onFilterChange("productVersion", event.target.value);
  };

  return (
    <Box sx={{ display: "flex", gap: 2, alignItems: "flex-end", flexWrap: "wrap" }}>
      {/* Severity */}
      <FormControl size="small" sx={{ flex: 1, minWidth: 160 }}>
        <InputLabel id="severity-label">
          {PRODUCT_VULNERABILITIES_SEVERITY_LABEL}
        </InputLabel>
        <Select
          labelId="severity-label"
          value={filters.severityId || ""}
          label={PRODUCT_VULNERABILITIES_SEVERITY_LABEL}
          onChange={handleSeverityChange}
        >
          <MenuItem value="">
            <Typography variant="body2">
              {PRODUCT_VULNERABILITIES_SEVERITY_ALL_LABEL}
            </Typography>
          </MenuItem>
          {severityOptions.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              <Typography variant="body2">{option.label}</Typography>
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      {/* Product */}
      <FormControl size="small" sx={{ flex: 1, minWidth: 180 }}>
        <InputLabel id="product-label">
          {PRODUCT_VULNERABILITIES_PRODUCT_LABEL}
        </InputLabel>
        <Select
          labelId="product-label"
          value={filters.productName || ""}
          label={PRODUCT_VULNERABILITIES_PRODUCT_LABEL}
          onChange={handleProductChange}
        >
          <MenuItem value="">
            <Typography variant="body2">
              {PRODUCT_VULNERABILITIES_ALL_PRODUCTS_LABEL}
            </Typography>
          </MenuItem>
          {productOptions.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              <Typography variant="body2">{option.label}</Typography>
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      {/* Product Version */}
      <FormControl
        size="small"
        sx={{ flex: 1, minWidth: 200 }}
        disabled={!filters.productName}
      >
        <InputLabel id="product-version-label">
          {PRODUCT_VULNERABILITIES_PRODUCT_VERSION_LABEL}
        </InputLabel>
        <Select
          labelId="product-version-label"
          value={filters.productVersion || ""}
          label={PRODUCT_VULNERABILITIES_PRODUCT_VERSION_LABEL}
          onChange={handleVersionChange}
        >
          <MenuItem value="">
            <Typography variant="body2">
              {PRODUCT_VULNERABILITIES_ALL_VERSIONS_LABEL}
            </Typography>
          </MenuItem>
          {productVersionOptions.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              <Typography variant="body2">{option.label}</Typography>
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      {/* Clear Filters */}
      <Button
        variant="outlined"
        color="warning"
        size="small"
        onClick={onClearFilters}
        sx={{ whiteSpace: "nowrap", height: 40 }}
      >
        {PRODUCT_VULNERABILITIES_CLEAR_FILTERS_LABEL}
      </Button>
    </Box>
  );
}
