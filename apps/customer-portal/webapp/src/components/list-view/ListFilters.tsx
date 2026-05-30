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
import type { JSX, UIEvent } from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import type {
  CaseMetadataResponse,
  AllCasesFilterValues,
} from "@features/support/types/cases";
import type { ProjectDeploymentItem } from "@features/project-details/types/deployments";
import type { ProjectContact } from "@features/settings/types/users";
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
  contacts?: ProjectContact[];
  isContactsLoading?: boolean;
  onFilterChange: (field: string, value: string | string[]) => void;
  excludeS0?: boolean;
  restrictSeverityToLow?: boolean;
  hideSeverityFilter?: boolean;
  hideStatusFilter?: boolean;
  hideDeploymentFilter?: boolean;
  hideCategoryFilter?: boolean;
  hideCreatedByFilter?: boolean;
  onLoadMoreDeployments?: () => void;
  hasMoreDeployments?: boolean;
  isFetchingMoreDeployments?: boolean;
}

/**
 * ListFilters component to display filter dropdowns for case lists.
 * Multi-select filters (status, severity, category, deployment) use checkboxes.
 *
 * @param {ListFiltersProps} props - Filter values, metadata, and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function ListFilters({
  filters,
  filterMetadata,
  deployments,
  contacts,
  isContactsLoading = false,
  onFilterChange,
  excludeS0 = false,
  restrictSeverityToLow = false,
  hideSeverityFilter = false,
  hideStatusFilter = false,
  hideDeploymentFilter = false,
  hideCategoryFilter = false,
  hideCreatedByFilter = false,
  onLoadMoreDeployments,
  hasMoreDeployments = false,
  isFetchingMoreDeployments = false,
}: ListFiltersProps): JSX.Element {
  const handleSingleChange =
    (field: string) => (event: SelectChangeEvent<string | string[]>) => {
      const val = event.target.value;
      const resolved = Array.isArray(val) ? (val[0] ?? "") : val;
      if (resolved === SELECT_MENU_LOAD_MORE_ROW_VALUE) return;
      onFilterChange(field, resolved);
    };

  const handleMultiChange =
    (field: string) => (event: SelectChangeEvent<string[]>) => {
      const val = event.target.value as string[];
      onFilterChange(field, val);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1, flexWrap: { xs: "wrap", md: "nowrap" }, overflowX: { md: "auto" } }}>
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
        if (hideCategoryFilter && def.id === "category") {
          return null;
        }
        if (hideCreatedByFilter && def.id === "createdBy") {
          return null;
        }
        if (def.metadataKey === "severities" && restrictSeverityToLow) {
          return null;
        }
        const { label, allLabel } = deriveFilterLabels(def.id);

        const isDeploymentFilter = def.id === "deployment";
        const isCreatedByFilter = def.id === "createdBy";
        const options = (() => {
          if (isDeploymentFilter) {
            return (
              deployments?.map((deployment) => ({
                label: deployment.type?.label || deployment.name,
                value: deployment.id,
              })) ?? []
            );
          }
          if (isCreatedByFilter) {
            return (
              contacts?.map((contact) => ({
                label: `${contact.firstName} ${contact.lastName}`.trim() || contact.email,
                value: contact.email,
              })) ?? []
            );
          }
          if (!def.metadataKey) return [];
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

        if (def.multiSelect) {
          const selectedValues = (filters[def.filterKey] as string[] | undefined) ?? [];
          return (
            <Grid key={def.id} size={{ xs: 12, sm: 6 }} sx={{ flex: { md: "1 1 0" }, minWidth: { md: 160 } }}>
              <FormControl fullWidth size="small" sx={{ marginTop: { md: 1 } }}>
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
                  {isCreatedByFilter && isContactsLoading ? (
                    <MenuItem disabled>
                      <Typography variant="body2">Loading...</Typography>
                    </MenuItem>
                  ) : hasNoOptions ? (
                    <MenuItem disabled>
                      <Typography variant="body2">{EMPTY_DROPDOWN_PLACEHOLDER}</Typography>
                    </MenuItem>
                  ) : (
                    options.map((option) => (
                      <MenuItem key={option.value} value={option.value}>
                        <Checkbox checked={selectedValues.includes(option.value)} size="small" />
                        <Typography variant="body2" sx={{ ml: 1 }}>{option.label}</Typography>
                      </MenuItem>
                    ))
                  )}
                </Select>
              </FormControl>
            </Grid>
          );
        }

        return (
          <Grid key={def.id} size={{ xs: 12, sm: 6 }} sx={{ flex: { md: "1 1 0" }, minWidth: { md: 160 } }}>
            <FormControl fullWidth size="small">
              <InputLabel id={`${def.id}-label`}>{label}</InputLabel>
              <Select
                labelId={`${def.id}-label`}
                id={def.id}
                value={(filters[def.filterKey] as string | undefined) || ""}
                label={label}
                onChange={handleSingleChange(def.filterKey)}
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
