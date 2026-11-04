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

import type { CallsStatusFilterProps } from "@features/support/types/supportComponents";
import { FormControl, InputLabel, MenuItem, Select } from "@wso2/oxygen-ui";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Dropdown that narrows the Calls tab to one call request status.
 *
 * @param {CallsStatusFilterProps} props - Selected value, options and change handler.
 * @returns {JSX.Element} The rendered status filter.
 */
export default function CallsStatusFilter({
  value,
  options,
  onChange,
  disabled = false,
}: CallsStatusFilterProps): JSX.Element {
  const handleChange = (event: SelectChangeEvent<string>) => {
    onChange(event.target.value ?? "");
  };

  return (
    <FormControl size="small" sx={{ minWidth: 200 }} disabled={disabled}>
      <InputLabel id="calls-status-filter-label">Status</InputLabel>
      <Select
        labelId="calls-status-filter-label"
        id="calls-status-filter"
        value={value}
        label="Status"
        onChange={handleChange}
      >
        <MenuItem value="">All statuses</MenuItem>
        {options.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
