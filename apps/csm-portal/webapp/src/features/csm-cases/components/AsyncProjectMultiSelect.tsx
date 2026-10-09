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
import { useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useInfiniteProjectSearch } from "@features/csm-cases/api/useProjectSearch";

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
 * labelled even after the search results change. Input handling follows
 * SearchableMultiSelect's own uncontrolled pattern (see onInputChange below).
 */
export default function AsyncProjectMultiSelect({
  id = "cases-filter-project",
  label = "Project",
  values,
  onChange,
  nameSeed,
}: AsyncProjectMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads the first page of projects
  // on open (no typing needed) and re-pages as the user types. Closed → the
  // query idles (cached pages stay for an instant re-open).
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

  // Names captured when the user picks a project, so a chip keeps its label
  // even once the search moves on to a different term.
  const [pickedNames, setPickedNames] = useState<Map<string, string>>(
    () => new Map(),
  );

  const nameById = useMemo(() => {
    const m = new Map<string, string>(nameSeed);
    projects.forEach((p) => {
      if (p.name) m.set(p.id, p.name);
    });
    pickedNames.forEach((name, pid) => m.set(pid, name));
    return m;
  }, [nameSeed, projects, pickedNames]);

  // Stable identity unless the selected ids themselves, or one of their
  // resolved names, actually changes — deliberately NOT keyed on `values`/
  // `nameById` directly, since `nameById` gets a new Map identity on every
  // new page of search results (see its own useMemo above), including ones
  // about other, not-yet-selected projects. MUI's Autocomplete resets its
  // own (uncontrolled) input text whenever the `value` prop's reference
  // changes while focused (see useAutocomplete's own value-changed effect,
  // `if (focused && !valueChange) return;`) — so without this, typing a
  // search term got wiped out from under the user mid-search the moment a
  // new page of results came back, found live.
  const selectedKey = values.join("\u0000");
  const selectedNamesKey = values.map((v) => nameById.get(v) ?? v).join("\u0000");
  const selectedOptions: ProjectOption[] = useMemo(
    () => values.map((v) => ({ id: v, name: nameById.get(v) ?? v })),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- selectedKey/selectedNamesKey capture everything that should trigger a recompute; depending on values/nameById directly would defeat the point of this memo (see comment above).
    [selectedKey, selectedNamesKey],
  );

  // The dropdown's own pool is just the live search results — not the
  // current selection prepended in front of them (a previous version did
  // this "so the field can render its chips", but renderTags/nameById
  // already resolve a chip's label independently of what's in `options`,
  // via `pickedNames`/`nameSeed`, so nothing actually needed it). Prepending
  // the selection meant option 0 — what Enter/the default highlight acts on
  // — was always an already-picked project instead of the top real search
  // match, which is what made Enter appear to "select the wrong item."
  const options: ProjectOption[] = useMemo(
    () => projects.map((p) => ({ id: p.id, name: p.name || p.id })),
    [projects],
  );

  return (
    <Autocomplete<ProjectOption, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      // Spinner only while the first page loads; later pages append on scroll.
      loading={isFetching && projects.length === 0}
      disableCloseOnSelect
      sx={{
        "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 },
      }}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      // Fixed max-height (rather than relying on the default 40vh popper
      // sizing) so the listbox is reliably scrollable as soon as the first
      // page of results loads, on any screen size. Without this, a tall or
      // otherwise short-content viewport can let the default sizing fit the
      // whole first PROJECT_PAGE_SIZE page (10 rows) with room to spare --
      // nothing overflows, so onScroll's near-the-bottom check
      // (handleListboxScroll) never fires and fetchNextPage never runs,
      // making the picker look hard-capped at 10 projects even though the
      // pagination itself supports the full catalogue.
      slotProps={{ listbox: { onScroll: handleListboxScroll, style: { maxHeight: 280 } } }}
      onChange={(_event, next) => {
        setPickedNames((prev) => {
          const m = new Map(prev);
          next.forEach((o) => m.set(o.id, o.name));
          return m;
        });
        onChange(next.map((o) => o.id));
      }}
      // Deliberately NOT a controlled `inputValue` (see SearchableMultiSelect,
      // mirrored here) — letting Autocomplete own the input lets it reset
      // itself after each pick same as any plain search box. A previous
      // version controlled it to keep a typed term across picks, which
      // caused the leftover text, the stray cursor, and Enter's unpredictable
      // target all at once. `onInputChange` just observes the typed query
      // for the debounced search below; it's never fed back as `inputValue`.
      onInputChange={(_event, value) => setInput(value)}
      noOptionsText={
        isError
          ? "Couldn't load projects. Try again."
          : isFetching
            ? "Loading projects…"
            : "No projects found"
      }
      renderTags={(value) => {
        // Suppressed while the user has live text in the box — found live,
        // after the previous commit made this unconditional again: picking
        // a *first* project is fine (the box empties right back out, so
        // there's nothing for the summary to collide with), but picking a
        // *second* one meant typing a new query right next to an
        // already-shown summary, on the same non-wrapping line — the exact
        // overlap this component was first fixed for, just reached a
        // different way. Keyed on `input` itself, not `open`: opening the
        // dropdown to browse without typing anything should still show the
        // summary immediately (that's the whole point of the previous
        // commit), it's only live typed text that needs to hide it.
        if (input) return null;
        const displayText = value.map((o) => o.name).join(", ");
        const content = (
          <Box
            component="span"
            sx={{ flex: "1 1 0", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", pl: 1 }}
          >
            {displayText}
          </Box>
        );
        return value.length === 1 ? content : (
          <Tooltip title={displayText} placement="top">{content}</Tooltip>
        );
      }}
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            {/* A project name can run long with no space near the end
                (e.g. a slash-joined account/subscription name) --
                without `overflowWrap`, it only breaks at a space/hyphen,
                then overflows and gets clipped by the popup's own
                overflow instead of wrapping onto a further line. */}
            <ListItemText
              primary={option.name}
              slotProps={{
                primary: { style: { fontSize: 13, overflowWrap: "anywhere" } },
              }}
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
