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

import { Autocomplete, Checkbox, Chip, ListItemText, TextField } from "@wso2/oxygen-ui";
import type * as React from "react";
import type { JSX } from "react";

export interface MultiSelectFieldProps<T extends string> {
  id: string;
  label: string;
  values: T[];
  options: { value: T; label: string }[];
  onChange: (next: T[]) => void;
  disabled?: boolean;
}

/**
 * Autocomplete-based multi-select for a fixed, small set of options (e.g. an
 * enum) — selected values render as chips, options are type-to-search +
 * checkboxes. Pairs with {@link SearchableMultiSelect} for larger/dynamic
 * option lists. Extracted from `CasesFilterBar.tsx` so other feature filter
 * bars (e.g. time cards) can reuse the same look and behavior.
 */
export default function MultiSelectField<T extends string>({
  id,
  label,
  values,
  options,
  onChange,
  disabled,
}: MultiSelectFieldProps<T>): JSX.Element {
  const selected = options.filter((o) => values.includes(o.value));
  return (
    <Autocomplete<{ value: T; label: string }, true>
      multiple
      size="small"
      id={id}
      disabled={disabled}
      options={options}
      value={selected}
      disableCloseOnSelect
      getOptionLabel={(o) => o.label}
      isOptionEqualToValue={(o, v) => o.value === v.value}
      onChange={(_event, next) => onChange(next.map((o) => o.value))}
      renderTags={(value, getTagProps) =>
        value.map((option, index) => {
          const { key, ...tagProps } = getTagProps({ index });
          return (
            <Chip key={key} size="small" label={option.label} {...tagProps} />
          );
        })
      }
      renderOption={(props, option, { selected: isSelected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={isSelected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option.label}
              slotProps={{ primary: { style: { fontSize: 13 } } }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Select…"}
        />
      )}
    />
  );
}
