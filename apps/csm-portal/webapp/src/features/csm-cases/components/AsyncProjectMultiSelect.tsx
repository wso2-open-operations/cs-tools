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
  Checkbox,
  Chip,
  ListItemText,
  TextField,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useProjectSearch } from "@features/csm-cases/api/useProjectSearch";

interface ProjectOption {
  id: string;
  name: string;
}

interface AsyncProjectMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected project ids. */
  values: string[];
  onChange: (next: string[]) => void;
  /**
   * Known id → name pairs (e.g. from the cases currently on screen) used to
   * label already-selected projects before any search has run.
   */
  nameSeed?: Map<string, string>;
}

/**
 * Project filter that searches the backend as the user types instead of
 * loading the whole project catalogue up front. Selected project names are
 * remembered (captured at selection time, plus any seed) so the chips stay
 * labelled even after the search results change.
 */
export default function AsyncProjectMultiSelect({
  id = "cases-filter-project",
  label = "Project",
  values,
  onChange,
  nameSeed,
}: AsyncProjectMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  const { data, isFetching } = useProjectSearch(query, query.length > 0);

  // Names captured when the user picks a project, so a chip keeps its label
  // even once the search moves on to a different term.
  const [pickedNames, setPickedNames] = useState<Map<string, string>>(
    () => new Map(),
  );

  const nameById = useMemo(() => {
    const m = new Map<string, string>(nameSeed);
    (data ?? []).forEach((p) => {
      if (p.name) m.set(p.id, p.name);
    });
    pickedNames.forEach((name, pid) => m.set(pid, name));
    return m;
  }, [nameSeed, data, pickedNames]);

  const selectedOptions: ProjectOption[] = useMemo(
    () => values.map((v) => ({ id: v, name: nameById.get(v) ?? v })),
    [values, nameById],
  );

  // Pool = current selection (so the field can render its chips) + the search
  // results, de-duplicated by id.
  const options: ProjectOption[] = useMemo(() => {
    const results = (data ?? []).map((p) => ({ id: p.id, name: p.name || p.id }));
    const seen = new Set(values);
    return [...selectedOptions, ...results.filter((o) => !seen.has(o.id))];
  }, [data, values, selectedOptions]);

  return (
    <Autocomplete<ProjectOption, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={selectedOptions}
      loading={isFetching}
      disableCloseOnSelect
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      onChange={(_event, next) => {
        setPickedNames((prev) => {
          const m = new Map(prev);
          next.forEach((o) => m.set(o.id, o.name));
          return m;
        });
        onChange(next.map((o) => o.id));
      }}
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        // Keep the typed term after a selection (reason "reset") so the user can
        // pick several from one search; clear only on explicit input/clear.
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        query.length === 0
          ? "Type to search projects…"
          : isFetching
            ? "Searching…"
            : "No projects found"
      }
      renderTags={(value, getTagProps) =>
        value.map((option, index) => {
          const { key, ...tagProps } = getTagProps({ index });
          return (
            <Chip key={key} size="small" label={option.name} {...tagProps} />
          );
        })
      }
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option.name}
              slotProps={{ primary: { style: { fontSize: 13 } } }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Type a project…"}
        />
      )}
    />
  );
}
