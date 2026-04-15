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
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";

export interface FilterOption {
  value: string | number;
  label: string;
}

export interface ProductVulnerabilitiesFiltersProps {
  filters: Record<string, string | number>;
  severityOptions?: FilterOption[];
  onFilterChange: (field: string, value: string | number) => void;
}

/**
 * ProductVulnerabilitiesFilters component to display filter dropdowns.
 *
 * @param {ProductVulnerabilitiesFiltersProps} props - Filter values and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function ProductVulnerabilitiesFilters({
  filters,
  severityOptions = [],
  onFilterChange,
}: ProductVulnerabilitiesFiltersProps): JSX.Element {
  const handleSelectChange = (event: SelectChangeEvent<string | number>) => {
    const val = event.target.value;
    onFilterChange("severityId", val);
  };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      <Grid size={{ xs: 12, sm: 6, md: 3 }}>
        <FormControl fullWidth size="small">
          <InputLabel id="severity-label">Severity</InputLabel>
          <Select
            labelId="severity-label"
            id="severityId"
            value={filters.severityId || ""}
            label="Severity"
            onChange={handleSelectChange}
          >
            <MenuItem value="">
              <Typography variant="body2">All Severity</Typography>
            </MenuItem>
            {severityOptions.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                <Typography variant="body2">{option.label}</Typography>
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Grid>
    </Grid>
  );
}
