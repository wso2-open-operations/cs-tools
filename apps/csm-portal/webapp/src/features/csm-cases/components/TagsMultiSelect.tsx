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

import { Autocomplete, Box, Chip, ListItemText, TextField, Tooltip } from "@wso2/oxygen-ui";
import { Minus, Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

/** A tag's tri-state selection: absent from both lists, required (`tag`
 * op:in), or excluded (`tag` op:notIn). Mirrors `CasesFilters.tags`/
 * `excludeTags` — never present in both at once. */
type TagState = "unselected" | "included" | "excluded";

export interface TriStateTagsValue {
  included: string[];
  excluded: string[];
}

interface TagsMultiSelectProps {
  id?: string;
  label?: string;
  /** Tags the case must carry (`filters.tags`). */
  includedValues: string[];
  /** Tags the case must NOT carry (`filters.excludeTags`). */
  excludedValues: string[];
  onChange: (next: TriStateTagsValue) => void;
}

function tagState(label: string, included: string[], excluded: string[]): TagState {
  if (included.includes(label)) return "included";
  if (excluded.includes(label)) return "excluded";
  return "unselected";
}

/** Unselected -> Include -> Exclude -> Unselected, Linear/Sentry-style. */
function nextTagState(current: TagState): TagState {
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
 * Tri-state tag filter for the cases list: each tag can be Included
 * (`filters.tags`, must carry), Excluded (`filters.excludeTags`, must NOT
 * carry), or left unselected. Replaces the earlier include-only
 * `TagsMultiSelect` -- `excludeTags` used to be settable only via a
 * dashboard widget click-through (see `buildActiveFilterChips` in
 * `CasesFilterBar.tsx`); this is the one bar control for both lists now
 * (digiops-cs#2907).
 *
 * Still searches already-used tag labels from the backend as the user types
 * (`useSearchTags`, the same `/tags/search` type-ahead {@link AddTagDialog}
 * uses), and stays effectively `freeSolo`: a tag is a genuinely free-text
 * label (the backing data source's generic label mechanism, e.g.
 * `micro-gw`, `ws-policy`) with no canonical existence check, so typing one
 * in and pressing Enter must keep working even with no matching suggestion
 * (same reasoning the original component documented).
 *
 * Deliberately does NOT use MUI Autocomplete's own built-in multi-select
 * selection model (binary in/out) -- that can't express a third "excluded"
 * state. Instead the Autocomplete's own `value` is always kept empty; every
 * "user picked option X" signal it reports (click, keyboard Enter, or a
 * freeSolo typed term) is treated as one opaque "cycle X" event, and this
 * component tracks each label's real tri-state itself from
 * `includedValues`/`excludedValues` (passed back to the caller via
 * `onChange`, not stored locally) rather than trusting anything MUI computed
 * about "selection".
 */
export default function TagsMultiSelect({
  id = "cases-filter-tags",
  label = "Tags",
  includedValues,
  excludedValues,
  onChange,
}: TagsMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads a first batch of
  // suggestions on open (no typing needed) and re-queries as the user types.
  const { data, isFetching, isError } = useSearchTags(query, open);

  // Pool = current selection (either state, so their rows still render with
  // the right indicator even once the search term no longer matches them) +
  // the search results' labels, de-duplicated.
  const options: string[] = useMemo(() => {
    const seen = new Set([...includedValues, ...excludedValues]);
    const results = (data ?? [])
      .map((t) => t.label)
      .filter((l) => l.length > 0 && !seen.has(l));
    return [...includedValues, ...excludedValues, ...results];
  }, [data, includedValues, excludedValues]);

  const cycle = (rawLabel: string): void => {
    const value = rawLabel.trim();
    if (!value.length) return;
    const current = tagState(value, includedValues, excludedValues);
    const next = nextTagState(current);
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

  const removeTag = (value: string): void => {
    onChange({
      included: includedValues.filter((v) => v !== value),
      excluded: excludedValues.filter((v) => v !== value),
    });
  };

  // MUI's Autocomplete only calls `renderTags` (and shows a clear/backspace
  // affordance) when its own `value` is non-empty, so this has to be the
  // real combined selection, not an always-empty array -- even though we
  // otherwise ignore what MUI *does* with this value (see `onChange` below
  // and `renderTags`' own comment).
  const comboValue = useMemo(
    () => [...includedValues, ...excludedValues],
    [includedValues, excludedValues],
  );

  return (
    <Autocomplete<string, true, false, true>
      multiple
      freeSolo
      // MUI hides the dropdown chevron by default for any `freeSolo`
      // Autocomplete (`hasPopupIcon` in its source is `!freeSolo` unless
      // this is set) -- every other filter control in this bar (Select- and
      // Autocomplete-based alike) shows one, so force it back on here too.
      forcePopupIcon
      size="small"
      id={id}
      options={options}
      value={comboValue}
      open={open}
      onOpen={() => setOpen(true)}
      // Stay open on a selection/removal (mirrors the old `disableCloseOnSelect`
      // -- this is a multi-pick control, picking/cycling one tag shouldn't
      // close the dropdown on someone about to pick several).
      onClose={(_event, reason) => {
        if (reason === "selectOption" || reason === "removeOption") return;
        setOpen(false);
      }}
      disableCloseOnSelect
      loading={isFetching && (data ?? []).length === 0}
      // The backend already filtered by the typed term; don't re-filter
      // locally (that would also drop the currently-selected values, which
      // must stay in `options` so their rows keep showing their indicator).
      filterOptions={(opts) => opts}
      sx={{ "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 } }}
      onChange={(event, next, reason) => {
        // Mouse clicks on an option row are handled entirely in
        // `renderOption`'s own `onClick` below (MUI's built-in click
        // handler for a row is deliberately never attached -- see there),
        // so this only ever fires for: a freeSolo typed term + Enter/comma
        // ("createOption"/"selectOption"), a keyboard-driven Enter on a
        // highlighted row, or a Backspace/clear removing the last chip
        // ("removeOption"). Diff against the real combined value rather
        // than trusting MUI's own add/remove *meaning* for that reason --
        // it doesn't know about the exclude state, so "already selected,
        // toggle it off" isn't right for an included tag (that should cycle
        // to excluded, not clear).
        if (reason === "removeOption") {
          const removed = comboValue.find((v) => !next.includes(v));
          if (!removed) return;
          // MUI reports the same "removeOption" reason for two different
          // user actions it can't otherwise distinguish here: pressing Enter
          // on an already-selected, keyboard-highlighted option (should
          // cycle it, same as a mouse click on that row), and Backspace/the
          // clear icon removing the last chip (a genuine removal). The
          // triggering key tells them apart -- Enter means "activate the
          // highlighted option".
          if ((event as React.KeyboardEvent)?.key === "Enter") {
            cycle(removed);
          } else {
            removeTag(removed);
          }
          return;
        }
        const added = next.find((v) => !comboValue.includes(v));
        if (typeof added === "string") cycle(added);
      }}
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        // Keep the typed term after a selection (reason "reset") so the user
        // can pick several from one search; clear only on explicit input/clear.
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load tags. Try again."
          : isFetching
            ? "Loading tags…"
            : "No matching tags — press Enter to filter by it anyway"
      }
      renderOption={(props, option) => {
        const { key, onClick: _onClick, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        const state = tagState(option, includedValues, excludedValues);
        return (
          <li
            key={key}
            {...liProps}
            // Fully custom click handling -- cycle the tri-state ourselves
            // rather than letting MUI's own (binary) selection model decide
            // what "clicking this row" means.
            onClick={(event) => {
              event.preventDefault();
              cycle(option);
            }}
            style={{ paddingTop: 2, paddingBottom: 2, display: "flex", alignItems: "center" }}
          >
            <Box
              sx={{
                width: 20,
                height: 20,
                mr: 1,
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
            {/* A tag label often has no spaces to wrap at (e.g.
                "Change-Tracking/Infrastructure") -- without
                `overflowWrap`, a long one only breaks at a hyphen, then
                overflows and gets clipped by the popup's own overflow
                instead of wrapping onto a further line. */}
            <ListItemText
              primary={option}
              slotProps={{
                primary: { style: { fontSize: 13, overflowWrap: "anywhere" } },
              }}
            />
          </li>
        );
      }}
      renderTags={() => {
        // Ignores the (plain-string) `value` array MUI passes here -- it
        // carries no include/exclude distinction. Renders from
        // `includedValues`/`excludedValues` directly instead, so each chip
        // knows which list it belongs to.
        const chips = [
          ...includedValues.map((v) => ({ value: v, excluded: false })),
          ...excludedValues.map((v) => ({ value: v, excluded: true })),
        ];
        if (chips.length === 0) return null;
        const displayText = chips
          .map((c) => `${c.excluded ? "-" : "+"} ${c.value}`)
          .join(", ");
        // The control sits in a narrow filter-bar column (~1/6 row width) --
        // measured in practice, the available box is too narrow to fit any
        // real tag label plus a second chip, however aggressively either is
        // shrunk (a real, reproduced bug: two chips selected, the row's own
        // `overflow: hidden` squeezed the first down to an unreadable "+"
        // sliver and hid the second entirely, even though both filters
        // were genuinely active). Once there's more than one, stop trying
        // to preview any individual label in the row itself -- show one
        // fixed-width summary chip ("2 tags") instead, which always fits
        // regardless of label length. The full include/exclude breakdown
        // is in the tooltip, and each one is still individually visible
        // (with its own +/- indicator) and editable by reopening the
        // dropdown.
        const content =
          chips.length === 1 ? (
            <Chip
              size="small"
              variant="outlined"
              color={chips[0].excluded ? "error" : "default"}
              label={`${chips[0].excluded ? "-" : "+"} ${chips[0].value}`}
              onDelete={() => removeTag(chips[0].value)}
              sx={{ ml: 0.5, maxWidth: "100%" }}
            />
          ) : (
            <Chip
              size="small"
              variant="outlined"
              label={`${chips.length} tags`}
              sx={{ ml: 0.5 }}
            />
          );
        return chips.length === 1 ? (
          content
        ) : (
          <Tooltip title={displayText} placement="top">
            {content}
          </Tooltip>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={
            includedValues.length || excludedValues.length ? undefined : "Search tags…"
          }
        />
      )}
    />
  );
}
