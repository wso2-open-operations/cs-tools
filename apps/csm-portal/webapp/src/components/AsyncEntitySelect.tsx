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

import { Autocomplete, Box, TextField, Typography } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX, type ReactNode } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";

interface AsyncEntitySelectOption {
  id: string;
  label: string;
  /** Set only on the pinned option (see `pinnedOption`). */
  caption?: string;
}

/** An option always listed first, with a short caption saying why. */
export interface AsyncEntitySelectPinnedOption {
  id: string;
  label: string;
  /** e.g. "service's support group" — shown under the label in the list. */
  caption: string;
}

export interface AsyncEntitySelectProps<T> {
  id: string;
  label: string;
  placeholder?: string;
  /** Selected entity id ("" when none). */
  value: string;
  /** Called with the selected entity id, plus (when available) the full
   * matching item `T` from the current search results — e.g. so a caller can
   * read a field off the selected entity beyond just its id/label. `item` is
   * `undefined` when cleared, or when the selection was seeded from `value`/
   * `knownLabel` rather than a live search result. */
  onChange: (next: string, item?: T) => void;
  disabled?: boolean;
  /** Marks the field required (asterisk + `aria-required`). Display only: the
   * caller still owns validation. */
  required?: boolean;
  helperText?: ReactNode;
  /** Shows the field in its error state (e.g. the backend refused the value);
   * the caller supplies the words through `helperText`. */
  error?: boolean;
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
  /** Ids to drop from the results — for pickers that add to a list the entity
   * is already on (e.g. a watch list), so an already-added person can't be
   * offered again. The currently selected `value` is never excluded, so the
   * field stays labelled. */
  excludeIds?: readonly string[];
  /** Listed first whenever the typed term is empty or matches its label, and
   * not repeated among the search results below it. Picking it reports its id,
   * with `item` only when the same entity is also among the current results. */
  pinnedOption?: AsyncEntitySelectPinnedOption | null;
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
  required,
  helperText,
  useSearch,
  getId,
  getLabel,
  knownLabel,
  searchExtra,
  excludeIds,
  error,
  pinnedOption,
}: AsyncEntitySelectProps<T>): JSX.Element {
  const [searchTerm, setSearchTerm] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(searchTerm, 300);
  const query = debounced.trim();

  const { data, isFetching, isError } = useSearch(query, open, searchExtra);
  const items = useMemo(() => {
    const all = data ?? [];
    if (!excludeIds?.length) return all;
    const skip = new Set(excludeIds);
    return all.filter((item) => getId(item) === value || !skip.has(getId(item)));
  }, [data, excludeIds, getId, value]);

  // Captured at selection time so the field stays labelled once the search
  // term (and its results) move on to something else.
  const [picked, setPicked] = useState<AsyncEntitySelectOption | null>(null);

  const selectedOption = useMemo<AsyncEntitySelectOption | null>(() => {
    if (!value) return null;
    if (picked && picked.id === value) return picked;
    if (pinnedOption && pinnedOption.id === value) {
      return { id: value, label: pinnedOption.label };
    }
    const match = items.find((item) => getId(item) === value);
    if (match) return { id: value, label: getLabel(match) };
    return { id: value, label: knownLabel ?? value };
  }, [value, picked, items, getId, getLabel, knownLabel, pinnedOption]);

  const options = useMemo<AsyncEntitySelectOption[]>(() => {
    let results: AsyncEntitySelectOption[] = items.map((item) => ({
      id: getId(item),
      label: getLabel(item),
    }));
    const pinned =
      pinnedOption &&
      (query.length === 0 || pinnedOption.label.toLowerCase().includes(query.toLowerCase()))
        ? pinnedOption
        : null;
    if (pinnedOption) results = results.filter((o) => o.id !== pinnedOption.id);
    if (
      selectedOption &&
      selectedOption.id !== pinned?.id &&
      !results.some((o) => o.id === selectedOption.id)
    ) {
      results = [selectedOption, ...results];
    }
    return pinned ? [{ ...pinned }, ...results] : results;
  }, [items, selectedOption, getId, getLabel, pinnedOption, query]);

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
      renderOption={
        pinnedOption
          ? (props, opt) => {
              const { key, ...optionProps } = props as typeof props & { key: string };
              return (
                <Box component="li" key={key} {...optionProps}>
                  <Box>
                    <Typography variant="body2">{opt.label}</Typography>
                    {opt.caption && (
                      <Typography variant="caption" color="text.secondary">
                        {opt.caption}
                      </Typography>
                    )}
                  </Box>
                </Box>
              );
            }
          : undefined
      }
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      onChange={(_event, next) => {
        setPicked(next);
        const matched = next ? items.find((item) => getId(item) === next.id) : undefined;
        onChange(next ? next.id : "", matched);
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
          required={required}
          placeholder={value ? undefined : placeholder}
          error={isError || !!error}
          helperText={isError ? "Search failed." : helperText}
        />
      )}
    />
  );
}
