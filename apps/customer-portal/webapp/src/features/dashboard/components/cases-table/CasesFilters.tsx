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
import { SelectMenuLoadMoreRow } from "@components/select-menu-load-more-row/SelectMenuLoadMoreRow";
import { EMPTY_DROPDOWN_PLACEHOLDER } from "@constants/common";
import { paginatedSelectMenuListProps } from "@utils/common";
import type { CasesFiltersProps } from "@/features/dashboard/types/casesTable";

/**
 * CasesFilters component to display filter dropdowns.
 *
 * @param {CasesFiltersProps} props - Filter values, metadata, and change handler.
 * @returns {JSX.Element} The rendered filter dropdowns.
 */
export default function CasesFilters({
  filters,
  filterFields,
  onFilterChange,
}: CasesFiltersProps): JSX.Element {
  // handle select change
  const handleSelectChange =
    (field: string) => (event: SelectChangeEvent<string | number>) => {
      const val = event.target.value;
      onFilterChange(field, val);
    };

  return (
    <Grid container spacing={2} sx={{ mt: 1 }}>
      {filterFields.map((field) => {
        const options = field.options || [];
        const hasNoOptions = options.length === 0;

        return (
          <Grid key={field.id} size={{ xs: 12, sm: 6, md: 2.4 }}>
            <FormControl fullWidth size="small">
              <InputLabel id={`${field.id}-label`}>{field.label}</InputLabel>
              <Select
                labelId={`${field.id}-label`}
                id={field.id}
                value={filters[field.id] || ""}
                label={field.label}
                onChange={handleSelectChange(field.id)}
                MenuProps={
                  field.onLoadMore
                    ? {
                        MenuListProps: paginatedSelectMenuListProps(
                          (e: UIEvent<HTMLElement>) => {
                            if (
                              !field.onLoadMore ||
                              !field.hasMore ||
                              field.isFetchingMore
                            ) {
                              return;
                            }
                            const el = e.currentTarget;
                            const threshold = 24;
                            const isNearBottom =
                              el.scrollHeight - el.scrollTop - el.clientHeight <
                              threshold;
                            if (isNearBottom) field.onLoadMore();
                          },
                        ),
                      }
                    : undefined
                }
              >
                <MenuItem value="">
                  <Typography variant="body2">
                    {hasNoOptions
                      ? EMPTY_DROPDOWN_PLACEHOLDER
                      : `All ${field.label}`}
                  </Typography>
                </MenuItem>
                {options.map((option) => {
                  const label =
                    typeof option === "string" ? option : option.label;
                  const value =
                    typeof option === "string" ? option : option.value;
                  return (
                    <MenuItem key={value} value={value}>
                      <Typography variant="body2">{label}</Typography>
                    </MenuItem>
                  );
                })}
                <SelectMenuLoadMoreRow
                  visible={Boolean(
                    field.onLoadMore &&
                    field.hasMore &&
                    field.isFetchingMore &&
                    options.length > 0,
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
