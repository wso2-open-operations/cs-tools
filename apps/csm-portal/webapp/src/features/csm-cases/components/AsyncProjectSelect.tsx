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
import { useEffect, useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useInfiniteProjectSearch } from "@features/csm-cases/api/useProjectSearch";
import type { BeProject } from "@api/backend/types";

interface ProjectOption {
  id: string;
  name: string;
}

interface AsyncProjectSelectProps {
  id?: string;
  label?: string;
  /** Selected project id ("" when none). */
  value: string;
  /** Called with the picked project's id ("" when cleared) and, when a
   * project was picked from the list, its display name. */
  onChange: (next: string, name?: string) => void;
  /** Label for `value` when it didn't come from a search this component ran
   * itself (e.g. restored from a draft or an existing record) — shown until
   * a real search result for the same id replaces it. */
  knownLabel?: string;
  required?: boolean;
  /** Text under the field (e.g. why it is required). A failed search shows its own message instead. */
  helperText?: React.ReactNode;
  disabled?: boolean;
  /** Hides the clear button, for places where "no project" is not a valid edit. */
  disableClearable?: boolean;
  /**
   * Restricts the searchable/selectable options to projects matching this
   * predicate (e.g. by `subscriptionType`) — the backend search endpoint has
   * no such filter, so this narrows the already-fetched page client-side.
   * Absent means "no restriction" (the default, used by the case/security
   * report/engagement creation flows).
   */
  filterProject?: (project: BeProject) => boolean;
  /**
   * Narrows the search to one customer's (account's) projects, server-side
   * (e.g. a change request raised from a standard change template, which is
   * used only for its own customer's projects). The current selection always
   * stays shown.
   */
  accountId?: string;
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
  knownLabel,
  required,
  helperText,
  disabled,
  disableClearable,
  filterProject,
  accountId,
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
  } = useInfiniteProjectSearch(query, open, accountId);

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
    return { id: value, name: knownLabel || value };
  }, [value, picked, projects, knownLabel]);

  // Pool = the current selection (so it can render) + the search results,
  // de-duplicated by id. `filterProject` (when given) is applied to the
  // search results only — the current selection always stays shown so an
  // already-picked project never disappears out from under the field.
  const filteredProjects = useMemo(
    () => (filterProject ? projects.filter(filterProject) : projects),
    [projects, filterProject],
  );
  const options = useMemo<ProjectOption[]>(() => {
    const results = filteredProjects.map((p) => ({ id: p.id, name: p.name || p.id }));
    if (selectedOption && !results.some((o) => o.id === selectedOption.id)) {
      return [selectedOption, ...results];
    }
    return results;
  }, [filteredProjects, selectedOption]);

  // With `filterProject` narrowing the loaded page(s) client-side, a page can
  // come back with zero matches even though a later page has some — and with
  // nothing rendered, the listbox can't be scrolled to trigger
  // handleListboxScroll's fetchNextPage. Keep loading pages automatically in
  // that case until a match appears or hasNextPage runs out.
  useEffect(() => {
    if (
      !open ||
      !filterProject ||
      isFetching ||
      isFetchingNextPage ||
      !hasNextPage ||
      filteredProjects.length > 0
    ) {
      return;
    }
    fetchNextPage();
  }, [
    open,
    filterProject,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    filteredProjects,
    fetchNextPage,
  ]);

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
      // Typed as `false` so `value`/`onChange` keep their nullable shape; the
      // runtime flag still hides the clear button.
      disableClearable={disableClearable as false | undefined}
      // Spinner only while the first page loads; later pages append on scroll.
      loading={isFetching && projects.length === 0}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => {
        setPicked(next);
        onChange(next ? next.id : "", next?.name);
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
          helperText={isError ? "Project search failed." : helperText}
        />
      )}
    />
  );
}
