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
import type { JSX, UIEvent } from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import type {
  CaseMetadataResponse,
  AllCasesFilterValues,
} from "@features/support/types/cases";
import type { ProjectDeploymentItem } from "@features/project-details/types/deployments";
import {
  SelectMenuLoadMoreRow,
  SELECT_MENU_LOAD_MORE_ROW_VALUE,
} from "@components/select-menu-load-more-row/SelectMenuLoadMoreRow";
import { EMPTY_DROPDOWN_PLACEHOLDER } from "@constants/common";
import { paginatedSelectMenuListProps } from "@utils/common";
import { ALL_CASES_FILTER_DEFINITIONS } from "@features/support/constants/supportConstants";
import { isS0SeverityLabel } from "@features/dashboard/utils/dashboard";
import {
  deriveFilterLabels,
  mapSeverityToDisplay,
} from "@features/support/utils/support";

export interface ListFiltersProps {
  filters: AllCasesFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  deployments?: ProjectDeploymentItem[];
  onFilterChange: (field: string, value: string) => void;
  excludeS0?: boolean;
  restrictSeverityToLow?: boolean;
  hideSeverityFilter?: boolean;
  hideStatusFilter?: boolean;
  hideDeploymentFilter?: boolean;
  onLoadMoreDeployments?: () => void;
  hasMoreDeployments?: boolean;
  isFetchingMoreDeployments?: boolean;
}

/**
 * ListFilters component to display filter dropdowns for case lists.
 *
 * @param {ListFiltersProps} props - Filter values, metadata, and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function ListFilters({
  filters,
  filterMetadata,
  deployments,
  onFilterChange,
  excludeS0 = false,
  restrictSeverityToLow = false,
  hideSeverityFilter = false,
  hideStatusFilter = false,
  hideDeploymentFilter = false,
  onLoadMoreDeployments,
  hasMoreDeployments = false,
  isFetchingMoreDeployments = false,
}: ListFiltersProps): JSX.Element {
  const handleSelectChange =
    (field: string) => (event: SelectChangeEvent<string | string[]>) => {
      const val = event.target.value;
      const resolved = Array.isArray(val) ? (val[0] ?? "") : val;
      if (resolved === SELECT_MENU_LOAD_MORE_ROW_VALUE) return;
      onFilterChange(field, resolved);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {ALL_CASES_FILTER_DEFINITIONS.map((def) => {
        if (hideSeverityFilter && def.id === "severity") {
          return null;
        }
        if (hideStatusFilter && def.id === "status") {
          return null;
        }
        if (hideDeploymentFilter && def.id === "deployment") {
          return null;
        }
        if (def.metadataKey === "severities" && restrictSeverityToLow) {
          return null;
        }
        const { label, allLabel } = deriveFilterLabels(def.id);

        const isDeploymentFilter = def.id === "deployment";
        const options = (() => {
          if (isDeploymentFilter) {
            return (
              deployments?.map((deployment) => ({
                label: deployment.type?.label || deployment.name,
                value: deployment.id,
              })) ?? []
            );
          }
          const metadataOptions = filterMetadata?.[def.metadataKey];
          if (!Array.isArray(metadataOptions)) return [];
          const filtered =
            def.metadataKey === "severities" && excludeS0
              ? metadataOptions.filter(
                  (item: { label: string }) => !isS0SeverityLabel(item.label),
                )
              : metadataOptions;
          const severityFiltered =
            def.metadataKey === "severities" && restrictSeverityToLow
              ? filtered.filter((item: { label: string }) =>
                  mapSeverityToDisplay(item.label).startsWith("S4"),
                )
              : filtered;
          return severityFiltered.map(
            (item: { label: string; id: string }) => ({
              label:
                def.metadataKey === "severities"
                  ? mapSeverityToDisplay(item.label)
                  : item.label,
              value: def.useLabelAsValue ? item.label : item.id,
            }),
          );
        })();

        const hasNoOptions = (options?.length ?? 0) === 0;

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
                MenuProps={
                  isDeploymentFilter
                    ? {
                        MenuListProps: paginatedSelectMenuListProps(
                          onLoadMoreDeployments && hasMoreDeployments
                            ? (e: UIEvent<HTMLElement>) => {
                                if (
                                  !onLoadMoreDeployments ||
                                  !hasMoreDeployments ||
                                  isFetchingMoreDeployments
                                ) {
                                  return;
                                }
                                const el = e.currentTarget;
                                const threshold = 24;
                                const isNearBottom =
                                  el.scrollHeight -
                                    el.scrollTop -
                                    el.clientHeight <
                                  threshold;
                                if (isNearBottom) onLoadMoreDeployments();
                              }
                            : undefined,
                        ),
                      }
                    : undefined
                }
              >
                <MenuItem value="">
                  <Typography variant="body2">
                    {hasNoOptions ? EMPTY_DROPDOWN_PLACEHOLDER : allLabel}
                  </Typography>
                </MenuItem>
                {options?.map((option) => (
                  <MenuItem key={option.value} value={option.value}>
                    <Typography variant="body2">{option.label}</Typography>
                  </MenuItem>
                ))}
                <SelectMenuLoadMoreRow
                  visible={Boolean(
                    isDeploymentFilter &&
                    onLoadMoreDeployments &&
                    hasMoreDeployments &&
                    isFetchingMoreDeployments &&
                    (options?.length ?? 0) > 0,
                  )}
                />
              </Select>
            </FormControl>
          </Grid>
        );
      })}
    </Grid>
  );
}
