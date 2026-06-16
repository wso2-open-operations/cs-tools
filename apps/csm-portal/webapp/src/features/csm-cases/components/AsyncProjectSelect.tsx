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
import { useProjectSearch } from "@features/csm-cases/api/useProjectSearch";

interface ProjectOption {
  id: string;
  name: string;
}

interface AsyncProjectSelectProps {
  id?: string;
  label?: string;
  /** Selected project id ("" when none). */
  value: string;
  onChange: (next: string) => void;
  required?: boolean;
  disabled?: boolean;
}

/**
 * Single-project picker that searches the backend as the user types instead of
 * loading the whole project catalogue up front (see {@link useProjectSearch}).
 * The name of the picked project is remembered so the field keeps its label
 * even after the search term — and its results — move on.
 */
export default function AsyncProjectSelect({
  id = "case-project",
  label = "Project",
  value,
  onChange,
  required,
  disabled,
}: AsyncProjectSelectProps): JSX.Element {
  // Tracked separately from the displayed input value (which MUI manages and
  // shows the selected project's name) so the type-ahead query keeps working.
  const [searchTerm, setSearchTerm] = useState("");
  const debounced = useDebouncedValue(searchTerm, 300);
  const query = debounced.trim();

  const { data, isFetching, isError } = useProjectSearch(
    query,
    query.length > 0,
  );

  // Captured at selection time so the field stays labelled once the search
  // results change to a different term.
  const [picked, setPicked] = useState<ProjectOption | null>(null);

  const selectedOption = useMemo<ProjectOption | null>(() => {
    if (!value) return null;
    if (picked && picked.id === value) return picked;
    const match = (data ?? []).find((p) => p.id === value);
    if (match) return { id: match.id, name: match.name || match.id };
    return { id: value, name: value };
  }, [value, picked, data]);

  // Pool = the current selection (so it can render) + the search results,
  // de-duplicated by id.
  const options = useMemo<ProjectOption[]>(() => {
    const results = (data ?? []).map((p) => ({ id: p.id, name: p.name || p.id }));
    if (selectedOption && !results.some((o) => o.id === selectedOption.id)) {
      return [selectedOption, ...results];
    }
    return results;
  }, [data, selectedOption]);

  return (
    <Autocomplete<ProjectOption>
      fullWidth
      size="small"
      id={id}
      options={options}
      value={selectedOption}
      disabled={disabled}
      loading={isFetching}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
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
          ? "Type to search projects…"
          : isError
            ? "Could not load projects"
            : isFetching
              ? "Searching…"
              : "No projects found"
      }
      renderInput={(params) => {
        const failed = query.length > 0 && isError;
        return (
          <TextField
            {...params}
            label={label}
            required={required}
            placeholder={value ? undefined : "Type a project…"}
            error={failed}
            helperText={
              failed ? "Project search failed. Type to retry." : undefined
            }
          />
        );
      }}
    />
  );
}
