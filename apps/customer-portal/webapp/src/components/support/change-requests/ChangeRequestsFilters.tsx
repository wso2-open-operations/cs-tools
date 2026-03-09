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
import type {
  CaseMetadataResponse,
  ChangeRequestFilterValues,
} from "@models/responses";
import {
  CHANGE_REQUEST_FILTER_DEFINITIONS,
  formatImpactLabel,
} from "@constants/supportConstants";
import { deriveFilterLabels } from "@utils/support";

export interface ChangeRequestsFiltersProps {
  filters: ChangeRequestFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string) => void;
}

/**
 * ChangeRequestsFilters component to display filter dropdowns.
 *
 * @param {ChangeRequestsFiltersProps} props - Filter values, metadata, and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function ChangeRequestsFilters({
  filters,
  filterMetadata,
  onFilterChange,
}: ChangeRequestsFiltersProps): JSX.Element {
  const handleSelectChange =
    (field: string) => (event: SelectChangeEvent<string | string[]>) => {
      const val = event.target.value;
      onFilterChange(field, Array.isArray(val) ? (val[0] ?? "") : val);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {CHANGE_REQUEST_FILTER_DEFINITIONS.map((def) => {
        const { label, allLabel } = deriveFilterLabels(def.id);
        const metadataOptions = filterMetadata?.[def.metadataKey];
        const options = Array.isArray(metadataOptions)
          ? metadataOptions.map((item: { label: string; id: string }) => ({
              label: item.label,
              value: item.id,
            }))
          : [];

        const currentValue = filters[def.filterKey] || "";

        return (
          <Grid key={def.id} size={{ xs: 12, sm: 6, md: 4 }}>
            <FormControl fullWidth size="small">
              <InputLabel id={`filter-${def.id}-label`}>{label}</InputLabel>
              <Select
                labelId={`filter-${def.id}-label`}
                id={`filter-${def.id}`}
                value={currentValue}
                label={label}
                onChange={handleSelectChange(def.filterKey)}
              >
                <MenuItem value="">
                  <Typography variant="body2">{allLabel}</Typography>
                </MenuItem>
                {options?.map((option) => {
                  // Format impact labels to remove numeric prefix
                  const displayLabel =
                    def.id === "impact"
                      ? formatImpactLabel(option.label)
                      : option.label;

                  return (
                    <MenuItem key={option.value} value={option.value}>
                      <Typography variant="body2">{displayLabel}</Typography>
                    </MenuItem>
                  );
                })}
              </Select>
            </FormControl>
          </Grid>
        );
      })}
    </Grid>
  );
}
