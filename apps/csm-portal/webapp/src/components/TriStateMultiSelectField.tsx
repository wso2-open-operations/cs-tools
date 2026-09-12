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
  FormControl,
  InputLabel,
  ListItemText,
  MenuItem,
  Select,
  Tooltip,
} from "@wso2/oxygen-ui";
import { Minus, Plus } from "@wso2/oxygen-ui-icons-react";
import { useRef, useState, type JSX } from "react";

export interface TriStateMultiSelectFieldProps<T extends string> {
  id: string;
  label: string;
  /** Values that must be present (op `in`). */
  includedValues: T[];
  /** Values that must NOT be present (op `notIn`). Only pass this prop (and
   * let a caller reach the `excluded` cycle state) for a field whose
   * downstream search contract actually supports `notIn` on this field —
   * this component has no opinion on that, it just renders whatever
   * `includedValues`/`excludedValues` it's given. */
  excludedValues: T[];
  options: { value: T; label: string }[];
  onChange: (next: { included: T[]; excluded: T[] }) => void;
  disabled?: boolean;
  disabledTooltip?: string;
}

type TriState = "unselected" | "included" | "excluded";

function stateOf<T extends string>(value: T, included: T[], excluded: T[]): TriState {
  if (included.includes(value)) return "included";
  if (excluded.includes(value)) return "excluded";
  return "unselected";
}

function nextStateOf(current: TriState): TriState {
  switch (current) {
    case "unselected":
      return "included";
    case "included":
      return "excluded";
    case "excluded":
    default:
      return "unselected";
  }
}

/**
 * Fixed-option tri-state multi-select: each option cycles Unselected ->
 * Include (`+`) -> Exclude (`-`) -> Unselected on click (same Linear/Sentry
 * pattern, and the same underlying tri-state model, as the free-text
 * {@link TagsMultiSelect} — this is the fixed-small-enum counterpart of
 * that component, built on `Select` rather than `Autocomplete` since the
 * option list here is small and known upfront, not searched).
 *
 * Unlike `TagsMultiSelect`, this renders its selection as plain
 * comma-joined text (mirroring `MultiSelectField`'s own `renderValue`),
 * not individual Chip pills — a fixed-option field's labels are typically
 * short enough that a text summary with the existing ellipsis+Tooltip
 * pattern never needs the "collapse to a count" fallback `TagsMultiSelect`
 * needed for arbitrary-length free-text tags.
 */
export default function TriStateMultiSelectField<T extends string>({
  id,
  label,
  includedValues,
  excludedValues,
  options,
  onChange,
  disabled,
  disabledTooltip,
}: TriStateMultiSelectFieldProps<T>): JSX.Element {
  const hasValue = includedValues.length > 0 || excludedValues.length > 0;
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState<number>();

  const handleOpen = (): void => {
    const width = rootRef.current?.getBoundingClientRect().width;
    if (width) setMenuWidth(width);
    setOpen(true);
  };

  const cycle = (value: T): void => {
    const current = stateOf(value, includedValues, excludedValues);
    const next = nextStateOf(current);
    const withoutValue = {
      included: includedValues.filter((v) => v !== value),
      excluded: excludedValues.filter((v) => v !== value),
    };
    if (next === "included") {
      onChange({ ...withoutValue, included: [...withoutValue.included, value] });
    } else if (next === "excluded") {
      onChange({ ...withoutValue, excluded: [...withoutValue.excluded, value] });
    } else {
      onChange(withoutValue);
    }
  };

  const field = (
    <FormControl ref={rootRef} fullWidth size="small" disabled={disabled}>
      <InputLabel id={`${id}-label`} shrink={hasValue} sx={{ top: "0px !important" }}>
        {label}
      </InputLabel>
      <Select
        multiple
        notched={hasValue}
        labelId={`${id}-label`}
        id={id}
        // A plain array of values has no include/exclude distinction of its
        // own -- `renderValue` below ignores it and reads from
        // `includedValues`/`excludedValues` directly, same reasoning as
        // `TagsMultiSelect`'s own `comboValue`. Combined value only exists
        // so MUI knows the field is "non-empty" for its own internals.
        value={[...includedValues, ...excludedValues]}
        label={label}
        open={open}
        onOpen={handleOpen}
        onClose={() => setOpen(false)}
        MenuProps={{
          slotProps: {
            paper: { sx: menuWidth ? { width: menuWidth } : { maxWidth: 280 } },
          },
        }}
        // Selection is handled entirely by each MenuItem's own onClick
        // (cycling tri-state) -- MUI's default onChange here would just
        // toggle binary membership, which can't express "excluded".
        onChange={() => {}}
        renderValue={() => {
          const chips = [
            ...includedValues.map((v) => ({ value: v, excluded: false })),
            ...excludedValues.map((v) => ({ value: v, excluded: true })),
          ];
          if (chips.length === 0) return "";
          const labelOf = (v: T): string => options.find((o) => o.value === v)?.label ?? v;
          const displayText = chips
            .map((c) => `${c.excluded ? "-" : "+"} ${labelOf(c.value)}`)
            .join(", ");
          if (chips.length === 1) return displayText;
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
        {options.map((option) => {
          const state = stateOf(option.value, includedValues, excludedValues);
          return (
            <MenuItem
              key={option.value}
              value={option.value}
              onClick={(event) => {
                event.preventDefault();
                cycle(option.value);
              }}
              sx={{ py: 0.5, alignItems: "flex-start", whiteSpace: "normal" }}
            >
              <Box
                sx={{
                  width: 20,
                  height: 20,
                  mr: 1,
                  mt: "1px",
                  flexShrink: 0,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  color: state === "excluded" ? "error.main" : "primary.main",
                }}
                aria-hidden="true"
              >
                {state === "included" && <Plus size={14} />}
                {state === "excluded" && <Minus size={14} />}
              </Box>
              <ListItemText
                primary={option.label}
                slotProps={{
                  primary: { style: { fontSize: 13, whiteSpace: "normal", wordBreak: "break-word" } },
                }}
              />
            </MenuItem>
          );
        })}
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
