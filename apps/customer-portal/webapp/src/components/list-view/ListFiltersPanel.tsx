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
import { deriveFilterLabels } from "@features/support/utils/support";

export interface FilterDefinition {
  id: string;
  filterKey: string;
  metadataKey: string;
  useLabelAsValue?: boolean;
}

export interface FilterOption {
  label: string;
  value: string;
}

export interface ListFiltersPanelProps<
  T extends Record<string, string | undefined>,
> {
  filterDefinitions: FilterDefinition[];
  filters: T;
  /** Resolves the option list for a given definition — handles any label transforms */
  resolveOptions: (def: FilterDefinition) => FilterOption[];
  onFilterChange: (field: string, value: string) => void;
  /** Defaults to { xs: 12, sm: 6, md: 3 } */
  gridSize?: { xs?: number; sm?: number; md?: number };
}

/**
 * ListFiltersPanel renders a grid of Select dropdowns driven by a filter
 * definitions array. Callers supply a resolveOptions callback to handle any
 * per-field label transformations.
 *
 * @param {ListFiltersPanelProps} props - Filter definitions, values, and handlers.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function ListFiltersPanel<
  T extends Record<string, string | undefined>,
>({
  filterDefinitions,
  filters,
  resolveOptions,
  onFilterChange,
  gridSize = { xs: 12, sm: 6, md: 3 },
}: ListFiltersPanelProps<T>): JSX.Element {
  const handleSelectChange =
    (field: string) => (event: SelectChangeEvent<string | string[]>) => {
      const val = event.target.value;
      onFilterChange(field, Array.isArray(val) ? (val[0] ?? "") : val);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {filterDefinitions.map((def) => {
        const { label, allLabel } = deriveFilterLabels(def.id);
        const options = resolveOptions(def);

        return (
          <Grid key={def.id} size={gridSize}>
            <FormControl fullWidth size="small">
              <InputLabel id={`${def.id}-label`}>{label}</InputLabel>
              <Select
                labelId={`${def.id}-label`}
                id={def.id}
                value={filters[def.filterKey] || ""}
                label={label}
                onChange={handleSelectChange(def.filterKey)}
              >
                <MenuItem value="">
                  <Typography variant="body2">{allLabel}</Typography>
                </MenuItem>
                {options.map((option) => (
                  <MenuItem key={option.value} value={option.value}>
                    <Typography variant="body2">{option.label}</Typography>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>
        );
      })}
    </Grid>
  );
}
