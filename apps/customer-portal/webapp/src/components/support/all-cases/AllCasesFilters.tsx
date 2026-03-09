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
  AllCasesFilterValues,
  ProjectDeploymentItem,
} from "@models/responses";
import { ALL_CASES_FILTER_DEFINITIONS } from "@constants/supportConstants";
import { isS0SeverityLabel } from "@constants/dashboardConstants";
import { deriveFilterLabels, mapSeverityToDisplay } from "@utils/support";

export interface AllCasesFiltersProps {
  filters: AllCasesFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  deployments?: ProjectDeploymentItem[];
  onFilterChange: (field: string, value: string) => void;
  excludeS0?: boolean;
}

/**
 * AllCasesFilters component to display filter dropdowns.
 *
 * @param {AllCasesFiltersProps} props - Filter values, metadata, and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function AllCasesFilters({
  filters,
  filterMetadata,
  deployments,
  onFilterChange,
  excludeS0 = false,
}: AllCasesFiltersProps): JSX.Element {
  const handleSelectChange =
    (field: string) => (event: SelectChangeEvent<string | string[]>) => {
      const val = event.target.value;
      onFilterChange(field, Array.isArray(val) ? (val[0] ?? "") : val);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {ALL_CASES_FILTER_DEFINITIONS.map((def) => {
        const { label, allLabel } = deriveFilterLabels(def.id);

        const isDeploymentFilter = def.id === "deployment";
        const options = isDeploymentFilter && deployments
          ? deployments.map((deployment) => ({
              label: deployment.type?.label || deployment.name,
              value: deployment.id,
            }))
          : (() => {
              const metadataOptions = filterMetadata?.[def.metadataKey];
              if (!Array.isArray(metadataOptions)) return [];
              const filtered =
                def.metadataKey === "severities" && excludeS0
                  ? metadataOptions.filter(
                      (item: { label: string }) =>
                        !isS0SeverityLabel(item.label),
                    )
                  : metadataOptions;
              return filtered.map((item: { label: string; id: string }) => ({
                label:
                  def.metadataKey === "severities"
                    ? mapSeverityToDisplay(item.label)
                    : item.label,
                value: def.useLabelAsValue ? item.label : item.id,
              }));
            })();

        return (
          <Grid key={def.id} size={{ xs: 12, sm: 6, md: 3 }}>
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
