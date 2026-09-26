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

import { Autocomplete, Box, Checkbox, ListItemText, TextField, Tooltip } from "@wso2/oxygen-ui";
import type { JSX } from "react";

export interface KBMultiSelectOption {
  value: string;
  label: string;
}

interface KBMultiSelectFilterProps {
  label: string;
  options: KBMultiSelectOption[];
  values: string[];
  onChange: (next: string[]) => void;
  minWidth?: number;
}

/**
 * Multi-select filter dropdown with checkboxes, matching the visual pattern
 * of csm-cases' ProductNameMultiSelect (used on the Support page, per
 * Sajith's Sep 10 request to match that style). Simpler than that
 * component on purpose: this backs bounded, already-loaded option sets
 * (knowledge bases, the 4 fixed states, authors present on the current
 * page) rather than a large server-searched catalogue, so there's no
 * server-side paging/typeahead here -- just a plain, fully client-side
 * list of checkboxes.
 */
export default function KBMultiSelectFilter({
  label,
  options,
  values,
  onChange,
  minWidth = 200,
}: KBMultiSelectFilterProps): JSX.Element {
  return (
    <Autocomplete<KBMultiSelectOption, true>
      multiple
      size="small"
      options={options}
      value={options.filter((o) => values.includes(o.value))}
      disableCloseOnSelect
      sx={{ minWidth, "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 } }}
      getOptionLabel={(opt) => opt.label}
      isOptionEqualToValue={(opt, val) => opt.value === val.value}
      onChange={(_event, next) => onChange(next.map((o) => o.value))}
      renderTags={(value) => {
        const displayText = value.map((v) => v.label).join(", ");
        const content = (
          <Box
            component="span"
            sx={{ flex: "1 1 0", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
          >
            {displayText}
          </Box>
        );
        return value.length > 1 ? <Tooltip title={displayText} placement="top">{content}</Tooltip> : content;
      }}
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & { key: string };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText primary={option.label} slotProps={{ primary: { style: { fontSize: 13 } } }} />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField {...params} label={label} placeholder={values.length ? undefined : `All ${label.toLowerCase()}`} />
      )}
    />
  );
}
