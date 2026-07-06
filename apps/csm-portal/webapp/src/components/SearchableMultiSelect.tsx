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

export interface SearchableMultiSelectProps {
  id: string;
  label: string;
  placeholder?: string;
  values: string[];
  options: string[];
  /** Optional renderer for option labels (e.g. an `@me` sentinel → "Me"). */
  formatOption?: (value: string) => string;
  /** Optional secondary line shown beneath each option (e.g. email). */
  getOptionSecondary?: (value: string) => string | undefined;
  /** Optional filter against synthetic text (e.g. include email in the query). */
  getOptionSearchText?: (value: string) => string;
  onChange: (next: string[]) => void;
  disabled?: boolean;
}

/**
 * Multi-select autocomplete. Users type to filter the option list and
 * select multiple. Picked values render as removable chips inside the field.
 * Internally a thin wrapper over MUI Autocomplete so we keep oxygen-ui
 * theming via the re-exported components. Extracted from `CasesFilterBar.tsx`
 * so other feature filter bars (e.g. time cards) can reuse the same look and
 * behavior instead of a plain free-text field.
 */
export default function SearchableMultiSelect({
  id,
  label,
  placeholder,
  values,
  options,
  formatOption,
  getOptionSecondary,
  getOptionSearchText,
  onChange,
  disabled,
}: SearchableMultiSelectProps): JSX.Element {
  const format = formatOption ?? ((v: string) => v);
  const searchText = getOptionSearchText ?? format;
  return (
    <Autocomplete
      multiple
      size="small"
      disabled={disabled}
      id={id}
      options={options}
      value={values}
      onChange={(_event, next) => onChange(next as string[])}
      disableCloseOnSelect
      // MUI's own available-space heuristic sometimes flips the list above
      // the field even when there's clearly room below (e.g. sitting above a
      // data table) — pin it open downward instead of guessing.
      slotProps={{
        popper: {
          placement: "bottom-start",
          modifiers: [{ name: "flip", enabled: false }],
        },
      }}
      getOptionLabel={(opt) => format(opt as string)}
      isOptionEqualToValue={(opt, val) => opt === val}
      filterOptions={(opts, state) => {
        const q = state.inputValue.trim().toLowerCase();
        if (!q) return opts;
        return opts.filter((o) => searchText(o as string).toLowerCase().includes(q));
      }}
      renderTags={(value, getTagProps) =>
        value.map((option, index) => {
          const { key, ...tagProps } = getTagProps({ index });
          return (
            <Chip
              key={key}
              size="small"
              label={format(option as string)}
              {...tagProps}
            />
          );
        })
      }
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        const primary = format(option as string);
        const secondary = getOptionSecondary?.(option as string);
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox
              size="small"
              checked={selected}
              sx={{ mr: 1, p: 0.25 }}
            />
            <ListItemText
              primary={primary}
              secondary={secondary}
              slotProps={{
                primary: { style: { fontSize: 13 } },
                secondary: { style: { fontSize: 11 } },
              }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : placeholder ?? "Type to search…"}
        />
      )}
    />
  );
}
