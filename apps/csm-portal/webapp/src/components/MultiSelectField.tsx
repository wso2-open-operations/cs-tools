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
  InputLabel,
  ListItemText,
  MenuItem,
  Select,
  Tooltip,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

export interface MultiSelectFieldProps<T extends string> {
  id: string;
  label: string;
  values: T[];
  options: { value: T; label: string }[];
  onChange: (next: T[]) => void;
  disabled?: boolean;
  /**
   * Explains why the field is disabled. Shown as a hover tooltip; ignored
   * when `disabled` is false.
   */
  disabledTooltip?: string;
}

/**
 * Select-based multi-select for a fixed, small set of options (e.g. an enum).
 * Selected values render as comma-separated text — no chips, fixed height.
 * Pairs with async pickers for larger/dynamic option lists.
 */
export default function MultiSelectField<T extends string>({
  id,
  label,
  values,
  options,
  onChange,
  disabled,
  disabledTooltip,
}: MultiSelectFieldProps<T>): JSX.Element {
  const field = (
    <FormControl fullWidth size="small" disabled={disabled}>
      <InputLabel id={`${id}-label`}>{label}</InputLabel>
      <Select
        multiple
        labelId={`${id}-label`}
        id={id}
        value={values}
        label={label}
        onChange={(event) => {
          const val = event.target.value;
          onChange(Array.isArray(val) ? (val as T[]) : []);
        }}
        renderValue={(selected) => {
          if (selected.length === 0) return "";
          const labels = selected.map(
            (v) => options.find((o) => o.value === v)?.label ?? v,
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
        {options.map((option) => (
          <MenuItem key={option.value} value={option.value} sx={{ py: 0.5 }}>
            <Checkbox
              size="small"
              checked={values.includes(option.value)}
              sx={{ mr: 1, p: 0.25 }}
            />
            <ListItemText
              primary={option.label}
              slotProps={{ primary: { style: { fontSize: 13 } } }}
            />
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );

  if (!disabled || !disabledTooltip) return field;

  return (
    <Tooltip title={disabledTooltip}>
      <Box component="span" sx={{ display: "block" }}>
        {field}
      </Box>
    </Tooltip>
  );
}
