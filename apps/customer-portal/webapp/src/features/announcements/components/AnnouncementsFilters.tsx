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
import { ANNOUNCEMENT_FILTER_DEFINITIONS } from "@features/support/constants/supportConstants";
import { deriveFilterLabels } from "@features/support/utils/support";
import type { AnnouncementsFiltersProps } from "@features/announcements/types/announcements";
import { buildAnnouncementsFilterSelectOptions } from "@features/announcements/utils/announcements";

/**
 * AnnouncementsFilters component for status and severity filter dropdowns.
 * Status-type filters render as multi-select with checkboxes; all others are single-select.
 *
 * @param props - Filter values, metadata, and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function AnnouncementsFilters({
  filters,
  filterMetadata,
  onFilterChange,
  disabled = false,
}: AnnouncementsFiltersProps): JSX.Element {
  const handleSingleChange =
    (field: string) => (event: SelectChangeEvent<string>) => {
      onFilterChange(field, event.target.value ?? "");
    };

  const handleMultiChange =
    (field: string) => (event: SelectChangeEvent<string[]>) => {
      onFilterChange(field, event.target.value as string[]);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {ANNOUNCEMENT_FILTER_DEFINITIONS.map((def) => {
        const { label, allLabel } = deriveFilterLabels(def.id);
        const options = buildAnnouncementsFilterSelectOptions(
          def,
          filterMetadata,
        );

        if (def.multiSelect) {
          const selectedValues = (filters[def.filterKey] as string[] | undefined) ?? [];
          return (
            <Grid key={def.id} size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={disabled}>
                <InputLabel id={`${def.id}-label`}>{label}</InputLabel>
                <Select<string[]>
                  multiple
                  labelId={`${def.id}-label`}
                  id={def.id}
                  value={selectedValues}
                  label={label}
                  disabled={disabled}
                  onChange={handleMultiChange(def.filterKey)}
                  renderValue={(selected) => {
                    if (!Array.isArray(selected) || selected.length === 0) return "";
                    const labels = selected.map(
                      (v) => options?.find((o) => o.value === v)?.label ?? v,
                    );
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
                  {options?.map((option) => (
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
          <Grid key={def.id} size={{ xs: 12, sm: 6, md: 3 }}>
            <FormControl fullWidth size="small" disabled={disabled}>
              <InputLabel id={`${def.id}-label`}>{label}</InputLabel>
              <Select
                labelId={`${def.id}-label`}
                id={def.id}
                value={(filters[def.filterKey] as string | undefined) || ""}
                label={label}
                onChange={handleSingleChange(def.filterKey)}
                disabled={disabled}
              >
                <MenuItem value="">
                  <Typography variant="body2">{allLabel}</Typography>
                </MenuItem>
                {options?.map((option) => (
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
