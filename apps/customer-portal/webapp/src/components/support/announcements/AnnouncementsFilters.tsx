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
import type { CaseMetadataResponse } from "@models/responses";
import {
  ANNOUNCEMENT_FILTER_DEFINITIONS,
  type AnnouncementFilterValues,
} from "@constants/supportConstants";
import { deriveFilterLabels, mapSeverityToDisplay } from "@utils/support";

export interface AnnouncementsFiltersProps {
  filters: AnnouncementFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string) => void;
  disabled?: boolean;
}

/**
 * AnnouncementsFilters component for status and severity filter dropdowns.
 *
 * @param {AnnouncementsFiltersProps} props - Filter values, metadata, and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function AnnouncementsFilters({
  filters,
  filterMetadata,
  onFilterChange,
  disabled = false,
}: AnnouncementsFiltersProps): JSX.Element {
  const handleSelectChange =
    (field: string) => (event: SelectChangeEvent<string>) => {
      onFilterChange(field, event.target.value ?? "");
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {ANNOUNCEMENT_FILTER_DEFINITIONS.map((def) => {
        const { label, allLabel } = deriveFilterLabels(def.id);
        const metadataOptions = filterMetadata?.[def.metadataKey];
        let options = Array.isArray(metadataOptions)
          ? metadataOptions.map((item: { label: string; id: string }) => ({
              label:
                def.metadataKey === "severities"
                  ? mapSeverityToDisplay(item.label)
                  : item.label,
              value: def.useLabelAsValue ? item.label : item.id,
            }))
          : [];

        if (def.metadataKey === "caseStates") {
          options = options.filter(
            (option) => option.value === "1" || option.value === "3",
          );
        }

        return (
          <Grid key={def.id} size={{ xs: 12, sm: 6, md: 3 }}>
            <FormControl fullWidth size="small" disabled={disabled}>
              <InputLabel id={`${def.id}-label`}>{label}</InputLabel>
              <Select
                labelId={`${def.id}-label`}
                id={def.id}
                value={filters[def.filterKey] || ""}
                label={label}
                onChange={handleSelectChange(def.filterKey)}
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
