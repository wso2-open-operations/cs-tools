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

import { Autocomplete, Box, TextField, Tooltip } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

interface AsyncTagMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected tag labels. */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * Plain (non-tri-state) multi-value tag picker for the unified "Advanced
 * filters" builder's `tag` field row — the `in`/`notIn` **op** on the row
 * itself now carries the direction (two separate rows express "includes" and
 * "excludes" at once, see `advancedFilters.ts`'s `tag` field entry), so
 * unlike the old dedicated bar control ({@link TagsMultiSelect}, removed from
 * the Simple grid) this component needs no include/exclude cycling of its
 * own — it is a normal multi-select, the tag-search twin of
 * {@link AsyncCreatedByMultiSelect}. Reuses the same `/tags/search`
 * type-ahead ({@link useSearchTags}) `TagsMultiSelect` and {@link AddTagDialog}
 * already use, and stays `freeSolo`: a tag is a genuinely free-text label
 * with no canonical existence check (see `TagsMultiSelect`'s own doc
 * comment), so typing one with no matching suggestion must still work.
 */
export default function AsyncTagMultiSelect({
  id = "advanced-filter-tag",
  label = "Value(s)",
  values,
  onChange,
}: AsyncTagMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  const { data, isFetching, isError } = useSearchTags(query, open);

  const options: string[] = useMemo(() => {
    const seen = new Set(values);
    const results = (data ?? [])
      .map((t) => t.label)
      .filter((l) => l.length > 0 && !seen.has(l));
    return [...values, ...results];
  }, [data, values]);

  return (
    <Autocomplete<string, true, false, true>
      multiple
      freeSolo
      forcePopupIcon
      size="small"
      id={id}
      options={options}
      value={values}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      disableCloseOnSelect
      loading={isFetching && (data ?? []).length === 0}
      // The backend already filtered by the typed term; don't re-filter
      // locally (that would also drop the currently-selected values).
      filterOptions={(opts) => opts}
      onChange={(_event, next) => onChange(next.map((v) => v.trim()).filter((v) => v.length > 0))}
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load tags. Try again."
          : isFetching
            ? "Loading tags…"
            : "No matching tags — press Enter to filter by it anyway"
      }
      renderTags={(value) => {
        const displayText = value.join(", ");
        const content = (
          <Box
            component="span"
            sx={{ flex: "1 1 0", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
          >
            {displayText}
          </Box>
        );
        return value.length === 1 ? content : (
          <Tooltip title={displayText} placement="top">{content}</Tooltip>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Search tags…"}
        />
      )}
    />
  );
}
