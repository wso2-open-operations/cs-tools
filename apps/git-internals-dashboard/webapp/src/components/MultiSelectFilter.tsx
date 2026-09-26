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

import { useRef, useState } from "react";
import { Box, Checkbox, FormControl, InputLabel, ListItemText, MenuItem, Select, Tooltip, type SelectChangeEvent } from "@mui/material";

export interface MultiSelectFilterOption {
  value: string;
  label: string;
}

interface MultiSelectFilterProps {
  id: string;
  label: string;
  values: string[];
  options: MultiSelectFilterOption[];
  onChange: (next: string[]) => void;
}

/**
 * Multi-select dropdown with a tick box per option: each MenuItem toggles on
 * click and keeps the menu open, the selected values render as one
 * comma-joined line with a tooltip when more than one is selected, and the
 * popup is pinned to the field's own measured width so a long label wraps
 * instead of widening it.
 */
export function MultiSelectFilter({ id, label, values, options, onChange }: MultiSelectFilterProps) {
  const hasValue = values.length > 0;

  // A value present in the URL but absent from the loaded options (e.g. a
  // team with no current open issues, or options still loading) still
  // renders as a ticked option, so it stays visible and can be unticked —
  // matching FilterSelect's handling of an unknown single value.
  const knownValues = new Set(options.map((o) => o.value));
  const allOptions = [...options, ...values.filter((v) => !knownValues.has(v)).map((v) => ({ value: v, label: v }))];
  const labelFor = (v: string): string => allOptions.find((o) => o.value === v)?.label ?? v;

  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState<number>();

  const handleOpen = (): void => {
    const width = rootRef.current?.getBoundingClientRect().width;
    if (width) setMenuWidth(width);
    setOpen(true);
  };

  return (
    <FormControl ref={rootRef} size="small" fullWidth>
      {/*
       * Oxygen UI's theme (MuiInputLabel styleOverrides) targets
       * `.MuiFormControl-root:has(.MuiSelect-select) &:not(.MuiInputLabel-shrink)`
       * and nudges an unshrunk label up by `top: -7px` — a compound
       * :has()/:not() selector whose specificity beats a plain sx-emitted
       * class, so only `!important` reliably wins the cascade. Without it,
       * this field's empty-state label sits visibly higher than the
       * single-select FilterSelect fields beside it in the same row.
       */}
      <InputLabel id={`${id}-label`} shrink={hasValue} sx={{ top: "0px !important" }}>
        {label}
      </InputLabel>
      <Select<string[]>
        multiple
        notched={hasValue}
        labelId={`${id}-label`}
        id={id}
        value={values}
        label={label}
        open={open}
        onOpen={handleOpen}
        onClose={() => setOpen(false)}
        size="small"
        sx={{ fontSize: 13, "& .MuiSelect-select": { minHeight: "20px !important", py: "8px" } }}
        MenuProps={{ slotProps: { paper: { sx: menuWidth ? { width: menuWidth } : { maxWidth: 280 } } } }}
        onChange={(event: SelectChangeEvent<string[]>) => {
          const val = event.target.value;
          onChange(typeof val === "string" ? val.split(",") : val);
        }}
        renderValue={(selected) => {
          if (selected.length === 0) return "";
          const text = selected.map(labelFor).join(", ");
          if (selected.length === 1) return text;
          return (
            <Tooltip title={text} placement="top">
              <Box component="span" sx={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {text}
              </Box>
            </Tooltip>
          );
        }}
      >
        {allOptions.map((option) => (
          <MenuItem key={option.value} value={option.value} sx={{ py: 0.5, alignItems: "flex-start", whiteSpace: "normal" }}>
            <Checkbox size="small" checked={values.includes(option.value)} sx={{ mr: 1, p: 0.25, mt: "1px" }} />
            <ListItemText
              primary={option.label}
              slotProps={{ primary: { style: { fontSize: 13, whiteSpace: "normal", wordBreak: "break-word" } } }}
            />
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
