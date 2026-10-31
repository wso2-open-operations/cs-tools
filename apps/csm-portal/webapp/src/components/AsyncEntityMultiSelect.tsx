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
  Autocomplete,
  Box,
  Checkbox,
  ListItemText,
  TextField,
  Tooltip,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type HTMLAttributes, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";

interface AsyncEntityMultiSelectOption {
  id: string;
  label: string;
}

export interface AsyncEntityMultiSelectProps<T> {
  id: string;
  label: string;
  placeholder?: string;
  /**
   * Keeps the label above the field even while it is empty, so the placeholder
   * shows (e.g. a bulk edit's "No change", beside selects that read the same way).
   */
  shrinkLabel?: boolean;
  /** Selected entity ids. */
  values: string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
  /** Marks the field required (asterisk + `aria-required`). Display only: the
   * caller still owns validation. */
  required?: boolean;
  helperText?: string;
  /** Same shape as `AsyncEntitySelect`'s `useSearch` — see its doc comment for
   * the stable-reference requirement (rules of hooks). */
  useSearch: (query: string, enabled: boolean, extra?: string) => {
    data: T[] | undefined;
    isFetching: boolean;
    isError: boolean;
  };
  getId: (item: T) => string;
  getLabel: (item: T) => string;
  /** Labels for `values` that didn't come from a search this component ran
   * itself (e.g. pre-filled from another source, like an incident's existing
   * watch list) — shown until a real search result for the same id replaces
   * them. Same purpose as `AsyncEntitySelect`'s `knownLabel`, plural. */
  knownLabels?: Record<string, string>;
  /** Render selected values as a compact, single-line summary matching
   * Select-based multi-filters. Other form usages retain removable chips by
   * default. */
  compactSelectedValues?: boolean;
}

/**
 * Multi-entity type-ahead picker — the `AsyncEntitySelect` sibling for fields
 * that take several ids (e.g. an incident's watch list). Each picked option's
 * label is cached by id so it stays labelled once the search term (and its
 * result set) moves on to something else, the same problem `AsyncEntitySelect`
 * solves for a single value.
 */
export default function AsyncEntityMultiSelect<T>({
  id,
  label,
  placeholder,
  shrinkLabel = false,
  values,
  onChange,
  disabled,
  required,
  helperText,
  useSearch,
  getId,
  getLabel,
  knownLabels,
  compactSelectedValues = false,
}: AsyncEntityMultiSelectProps<T>): JSX.Element {
  const [searchTerm, setSearchTerm] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(searchTerm, 300);
  const query = debounced.trim();

  const { data, isFetching, isError } = useSearch(query, open);
  const items = useMemo(() => data ?? [], [data]);

  const [labelById, setLabelById] = useState<Record<string, string>>(() => knownLabels ?? {});

  const selectedOptions = useMemo<AsyncEntityMultiSelectOption[]>(
    () =>
      values.map((v) => {
        const match = items.find((item) => getId(item) === v);
        const label = match ? getLabel(match) : (labelById[v] ?? v);
        return { id: v, label };
      }),
    [values, items, labelById, getId, getLabel],
  );

  const options = useMemo<AsyncEntityMultiSelectOption[]>(() => {
    const results = items.map((item) => ({ id: getId(item), label: getLabel(item) }));
    const missing = selectedOptions.filter((o) => !results.some((r) => r.id === o.id));
    return [...missing, ...results];
  }, [items, selectedOptions, getId, getLabel]);

  return (
    <Autocomplete<AsyncEntityMultiSelectOption, true>
      multiple
      fullWidth
      size="small"
      id={id}
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => {
        setSearchTerm("");
        setOpen(true);
      }}
      onClose={() => setOpen(false)}
      disabled={disabled}
      loading={isFetching && items.length === 0}
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.label}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as HTMLAttributes<HTMLLIElement> & { key: string };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option.label}
              slotProps={{ primary: { style: { fontSize: 13 } } }}
            />
          </li>
        );
      }}
      renderTags={
        compactSelectedValues
          ? (tagValues) => {
              const displayText = tagValues.map((option) => option.label).join(", ");
              const content = (
                <Box
                  component="span"
                  sx={{
                    flex: "1 1 0",
                    minWidth: 0,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {displayText}
                </Box>
              );
              return tagValues.length === 1 ? content : (
                <Tooltip title={displayText} placement="top">
                  {content}
                </Tooltip>
              );
            }
          : undefined
      }
      onChange={(_event, next) => {
        setLabelById((prev) => {
          const merged = { ...prev };
          for (const o of next) merged[o.id] = o.label;
          return merged;
        });
        onChange(next.map((o) => o.id));
      }}
      onInputChange={(_event, val, reason) => {
        if (reason === "input") setSearchTerm(val);
        else if (reason === "clear") setSearchTerm("");
      }}
      noOptionsText={
        query.length === 0
          ? "Type to search…"
          : isError
            ? "Search failed. Try again."
            : isFetching
              ? "Searching…"
              : "No matches found"
      }
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          slotProps={shrinkLabel ? { inputLabel: { ...params.InputLabelProps, shrink: true } } : undefined}
          placeholder={values.length ? undefined : placeholder}
          required={required}
          error={isError}
          helperText={isError ? "Search failed." : helperText}
        />
      )}
    />
  );
}
