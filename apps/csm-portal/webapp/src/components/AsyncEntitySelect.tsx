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

interface AsyncEntitySelectOption {
  id: string;
  label: string;
}

export interface AsyncEntitySelectProps<T> {
  id: string;
  label: string;
  placeholder?: string;
  /** Selected entity id ("" when none). */
  value: string;
  onChange: (next: string) => void;
  disabled?: boolean;
  helperText?: string;
  /** Type-ahead search hook — disabled externally while the dropdown is
   * closed or nothing has been typed yet. Must be passed as a stable
   * reference (e.g. `useSearch={useSearchGroups}`), never wrapped in an
   * inline arrow function — that would call a hook from inside a closure
   * and break the rules of hooks. `extra` threads one optional caller-scoped
   * value through to hooks that need it (e.g. narrowing service offerings to
   * an already-picked service); hooks that don't need it just ignore it. */
  useSearch: (query: string, enabled: boolean, extra?: string) => {
    data: T[] | undefined;
    isFetching: boolean;
    isError: boolean;
  };
  getId: (item: T) => string;
  getLabel: (item: T) => string;
  /** Label for `value` when it didn't come from a search this component ran
   * itself (e.g. pre-filled from another source) — shown until a real
   * search result for the same id replaces it. */
  knownLabel?: string;
  /** Forwarded as `useSearch`'s third argument — see `useSearch` above. */
  searchExtra?: string;
}

/**
 * Generic single-entity type-ahead picker: type a name, get real portal-UUID
 * options back from `useSearch`. Built for the change-request create form's
 * ServiceNow reference fields (Requested by / Assigned to / Assignment group
 * / Service / Service offering / Configuration item), each of which reuses
 * this with its own search hook and id/label accessors rather than five
 * near-identical components. Mirrors AsyncProjectSelect's shape, minus
 * pagination — a single page of matches is enough for a type-ahead field.
 */
export default function AsyncEntitySelect<T>({
  id,
  label,
  placeholder,
  value,
  onChange,
  disabled,
  helperText,
  useSearch,
  getId,
  getLabel,
  knownLabel,
  searchExtra,
}: AsyncEntitySelectProps<T>): JSX.Element {
  const [searchTerm, setSearchTerm] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(searchTerm, 300);
  const query = debounced.trim();

  const { data, isFetching, isError } = useSearch(query, open, searchExtra);
  const items = useMemo(() => data ?? [], [data]);

  // Captured at selection time so the field stays labelled once the search
  // term (and its results) move on to something else.
  const [picked, setPicked] = useState<AsyncEntitySelectOption | null>(null);

  const selectedOption = useMemo<AsyncEntitySelectOption | null>(() => {
    if (!value) return null;
    if (picked && picked.id === value) return picked;
    const match = items.find((item) => getId(item) === value);
    if (match) return { id: value, label: getLabel(match) };
    return { id: value, label: knownLabel ?? value };
  }, [value, picked, items, getId, getLabel, knownLabel]);

  const options = useMemo<AsyncEntitySelectOption[]>(() => {
    const results = items.map((item) => ({ id: getId(item), label: getLabel(item) }));
    if (selectedOption && !results.some((o) => o.id === selectedOption.id)) {
      return [selectedOption, ...results];
    }
    return results;
  }, [items, selectedOption, getId, getLabel]);

  return (
    <Autocomplete<AsyncEntitySelectOption>
      fullWidth
      size="small"
      id={id}
      options={options}
      value={selectedOption}
      open={open}
      onOpen={() => {
        setSearchTerm("");
        setOpen(true);
      }}
      onClose={() => setOpen(false)}
      disabled={disabled}
      loading={isFetching && items.length === 0}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.label}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      onChange={(_event, next) => {
        setPicked(next);
        onChange(next ? next.id : "");
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
          placeholder={value ? undefined : placeholder}
          error={isError}
          helperText={isError ? "Search failed." : helperText}
        />
      )}
    />
  );
}
