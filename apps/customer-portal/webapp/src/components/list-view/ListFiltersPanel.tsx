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
  Checkbox,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  Tooltip,
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
  multiSelect?: boolean;
}

export interface FilterOption {
  label: string;
  value: string;
}

export interface ListFiltersPanelProps<
  T extends Record<string, string | string[] | undefined>,
> {
  filterDefinitions: FilterDefinition[];
  filters: T;
  /** Resolves the option list for a given definition — handles any label transforms */
  resolveOptions: (def: FilterDefinition) => FilterOption[];
  onFilterChange: (field: string, value: string | string[]) => void;
  /** Defaults to { xs: 12, sm: 6, md: 3 } */
  gridSize?: { xs?: number; sm?: number; md?: number };
}

/**
 * ListFiltersPanel renders a grid of Select dropdowns driven by a filter
 * definitions array. Callers supply a resolveOptions callback to handle any
 * per-field label transformations. Status-type filters render as multi-select
 * with checkboxes; all other filters remain single-select.
 *
 * @param {ListFiltersPanelProps} props - Filter definitions, values, and handlers.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function ListFiltersPanel<
  T extends Record<string, string | string[] | undefined>,
>({
  filterDefinitions,
  filters,
  resolveOptions,
  onFilterChange,
  gridSize = { xs: 12, sm: 6, md: 3 },
}: ListFiltersPanelProps<T>): JSX.Element {
  const handleSingleChange =
    (field: string) => (event: SelectChangeEvent<string | string[]>) => {
      const val = event.target.value;
      onFilterChange(field, Array.isArray(val) ? (val[0] ?? "") : val);
    };

  const handleMultiChange =
    (field: string) => (event: SelectChangeEvent<string[]>) => {
      const val = event.target.value as string[];
      onFilterChange(field, val);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {filterDefinitions.map((def) => {
        const { label, allLabel } = deriveFilterLabels(def.id);
        const options = resolveOptions(def);

        if (def.multiSelect) {
          const selectedValues = (filters[def.filterKey] as string[] | undefined) ?? [];
          return (
            <Grid key={def.id} size={gridSize}>
              <FormControl fullWidth size="small">
                <InputLabel id={`${def.id}-label`}>{label}</InputLabel>
                <Select
                  multiple
                  labelId={`${def.id}-label`}
                  id={def.id}
                  value={selectedValues}
                  label={label}
                  onChange={handleMultiChange(def.filterKey)}
                  renderValue={(selected) => {
                    if (!Array.isArray(selected) || selected.length === 0) return "";
                    const labels = selected.map((v) => options.find((o) => o.value === v)?.label ?? v);
                    const displayText = labels.join(", ");
                    if (labels.length === 1) return displayText;
                    return (
                      <Tooltip title={displayText} placement="top">
                        <Box
                          component="span"
                          sx={{
                            display: "block",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {displayText}
                        </Box>
                      </Tooltip>
                    );
                  }}
                >
                  {options.map((option) => (
                    <MenuItem key={option.value} value={option.value}>
                      <Checkbox checked={selectedValues.includes(option.value)} size="small" />
                      <Typography variant="body2" sx={{ ml: 1 }}>{option.label}</Typography>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          );
        }

        return (
          <Grid key={def.id} size={gridSize}>
            <FormControl fullWidth size="small">
              <InputLabel id={`${def.id}-label`}>{label}</InputLabel>
              <Select
                labelId={`${def.id}-label`}
                id={def.id}
                value={(filters[def.filterKey] as string | undefined) || ""}
                label={label}
                onChange={handleSingleChange(def.filterKey)}
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
