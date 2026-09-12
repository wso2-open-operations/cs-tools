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
import { useRef, useState, type JSX } from "react";

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
  // Label sits centered in the box like a placeholder when nothing is
  // selected, and shrinks into the outline notch once a value is chosen —
  // matching the async pickers (e.g. AsyncAssigneeMultiSelect's Autocomplete)
  // rather than MUI's focus-driven default, since a disabled field can never
  // be focused and would otherwise be stuck looking different from its
  // enabled, unselected siblings.
  const hasValue = values.length > 0;

  // MUI's Select already pins the popup's `min-width` to the field's own
  // rendered width, but never caps its `max-width` -- a long option label
  // (e.g. a team name) otherwise stretches the popup far past the field it
  // dropped down from, wider than every other filter's popup. Measuring the
  // field's own width on open and pinning the popup to exactly that (not
  // just a generic cap) is what makes it match, whatever width this
  // particular instance happens to render at in its own grid slot; long
  // labels wrap onto a second line instead of widening the popup.
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState<number>();

  const handleOpen = (): void => {
    const width = rootRef.current?.getBoundingClientRect().width;
    if (width) setMenuWidth(width);
    setOpen(true);
  };

  const field = (
    <FormControl ref={rootRef} fullWidth size="small" disabled={disabled}>
      {/*
       * oxygen-ui's own theme (MuiInputLabel styleOverrides) targets
       * `.MuiFormControl-root:has(.MuiSelect-select) &:not(.MuiInputLabel-shrink)`
       * and shifts an unshrunk label up by `top: -7px` — a compound
       * `:has()`/`:not()` selector whose specificity beats a plain sx-emitted
       * class, so a plain `sx={{ top: 0 }}` silently loses the cascade.
       * `!important` is the only way to reliably win here. Without it, this
       * field's empty-state label sits visibly higher than the async pickers
       * (e.g. AsyncAssigneeMultiSelect's Autocomplete), which get no such
       * adjustment.
       */}
      <InputLabel id={`${id}-label`} shrink={hasValue} sx={{ top: "0px !important" }}>
        {label}
      </InputLabel>
      <Select
        multiple
        notched={hasValue}
        labelId={`${id}-label`}
        id={id}
        value={values}
        label={label}
        open={open}
        onOpen={handleOpen}
        onClose={() => setOpen(false)}
        MenuProps={{
          slotProps: {
            // Falls back to a generic cap for the one frame before a width
            // is ever measured (and in a non-layout environment like jsdom,
            // where `getBoundingClientRect` always reports 0).
            paper: { sx: menuWidth ? { width: menuWidth } : { maxWidth: 280 } },
          },
        }}
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
          // MUI's `MenuItem` sets `white-space: nowrap` by default (it
          // expects a single-line label) — with the popup now pinned to
          // the field's own width (see `menuWidth` above), a label longer
          // than that width needs to wrap onto a second line instead of
          // just getting clipped at the edge. `alignItems: "flex-start"`
          // keeps the checkbox pinned to the first line's height instead
          // of centering against the item's full two-line height.
          <MenuItem
            key={option.value}
            value={option.value}
            sx={{ py: 0.5, alignItems: "flex-start", whiteSpace: "normal" }}
          >
            <Checkbox
              size="small"
              checked={values.includes(option.value)}
              sx={{ mr: 1, p: 0.25, mt: "1px" }}
            />
            <ListItemText
              primary={option.label}
              slotProps={{
                primary: { style: { fontSize: 13, whiteSpace: "normal", wordBreak: "break-word" } },
              }}
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
