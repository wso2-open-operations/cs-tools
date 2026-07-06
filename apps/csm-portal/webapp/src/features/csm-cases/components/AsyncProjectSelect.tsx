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
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useInfiniteProjectSearch } from "@features/csm-cases/api/useProjectSearch";

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
 * Single-project picker. Mirrors the cases filter's project control
 * ({@link useInfiniteProjectSearch}): on open it loads the first page of
 * projects (no typing needed) and pages through the rest on scroll, narrowing
 * as the user types. The name of the picked project is remembered so the field
 * keeps its label even after the search term — and its results — move on.
 */
export default function AsyncProjectSelect({
  id = "case-project",
  label = "Project",
  value,
  onChange,
  required,
  disabled,
}: AsyncProjectSelectProps): JSX.Element {
  // Search term tracked separately from the displayed input value (which MUI
  // manages and shows the selected project's name) so the type-ahead works.
  const [searchTerm, setSearchTerm] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(searchTerm, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads the first page of projects
  // on open and re-pages as the user types.
  const {
    projects,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    isError,
    fetchNextPage,
  } = useInfiniteProjectSearch(query, open);

  // Lazy-load the next page when the listbox is scrolled near its end.
  const handleListboxScroll = (event: React.UIEvent<HTMLElement>): void => {
    const el = event.currentTarget;
    if (
      hasNextPage &&
      !isFetchingNextPage &&
      el.scrollHeight - el.scrollTop - el.clientHeight < 80
    ) {
      fetchNextPage();
    }
  };

  // Captured at selection time so the field stays labelled once the search
  // results change to a different term.
  const [picked, setPicked] = useState<ProjectOption | null>(null);

  const selectedOption = useMemo<ProjectOption | null>(() => {
    if (!value) return null;
    if (picked && picked.id === value) return picked;
    const match = projects.find((p) => p.id === value);
    if (match) return { id: match.id, name: match.name || match.id };
    return { id: value, name: value };
  }, [value, picked, projects]);

  // Pool = the current selection (so it can render) + the search results,
  // de-duplicated by id.
  const options = useMemo<ProjectOption[]>(() => {
    const results = projects.map((p) => ({ id: p.id, name: p.name || p.id }));
    if (selectedOption && !results.some((o) => o.id === selectedOption.id)) {
      return [selectedOption, ...results];
    }
    return results;
  }, [projects, selectedOption]);

  return (
    <Autocomplete<ProjectOption>
      fullWidth
      size="small"
      id={id}
      options={options}
      value={selectedOption}
      open={open}
      onOpen={() => {
        // Start each open from the default first page rather than a stale term.
        setSearchTerm("");
        setOpen(true);
      }}
      onClose={() => setOpen(false)}
      disabled={disabled}
      // Spinner only while the first page loads; later pages append on scroll.
      loading={isFetching && projects.length === 0}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => {
        setPicked(next);
        onChange(next ? next.id : "");
      }}
      onInputChange={(_event, val, reason) => {
        if (reason === "input") setSearchTerm(val);
        else if (reason === "clear") setSearchTerm("");
      }}
      noOptionsText={
        isError
          ? "Couldn't load projects. Try again."
          : isFetching
            ? "Loading projects…"
            : "No projects found"
      }
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          required={required}
          placeholder={value ? undefined : "Search projects…"}
          error={isError}
          helperText={isError ? "Project search failed." : undefined}
        />
      )}
    />
  );
}
