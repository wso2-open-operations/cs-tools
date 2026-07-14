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

import { Autocomplete, TextField } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";

interface AsyncEntityMultiSelectOption {
  id: string;
  label: string;
}

export interface AsyncEntityMultiSelectProps<T> {
  id: string;
  label: string;
  placeholder?: string;
  /** Selected entity ids. */
  values: string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
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
  values,
  onChange,
  disabled,
  helperText,
  useSearch,
  getId,
  getLabel,
}: AsyncEntityMultiSelectProps<T>): JSX.Element {
  const [searchTerm, setSearchTerm] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(searchTerm, 300);
  const query = debounced.trim();

  const { data, isFetching, isError } = useSearch(query, open);
  const items = useMemo(() => data ?? [], [data]);

  const [labelById, setLabelById] = useState<Record<string, string>>({});

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
          placeholder={values.length ? undefined : placeholder}
          error={isError}
          helperText={isError ? "Search failed." : helperText}
        />
      )}
    />
  );
}
