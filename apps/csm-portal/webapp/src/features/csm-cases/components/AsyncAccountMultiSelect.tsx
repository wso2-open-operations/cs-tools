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
import { useInfiniteAccountSearch } from "@features/csm-accounts/api/useSearchAccounts";

interface AccountOption {
  id: string;
  name: string;
}

interface AsyncAccountMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected account ids. */
  values: string[];
  onChange: (next: string[]) => void;
  /**
   * Known id → name pairs (e.g. from the cases currently on screen) used to
   * label already-selected accounts before any search has run.
   */
  nameSeed?: Map<string, string>;
}

/**
 * Account filter that searches the backend as the user types instead of
 * loading the whole account catalogue up front. Selected account names are
 * remembered (captured at selection time, plus any seed) so the chips stay
 * labelled even after the search results change. Mirrors
 * `AsyncProjectMultiSelect` (the `projectId` row's own value input) — same
 * debounce/search/paginate shape, pointed at `POST /accounts/search` instead.
 */
export default function AsyncAccountMultiSelect({
  id = "cases-filter-account",
  label = "Account",
  values,
  onChange,
  nameSeed,
}: AsyncAccountMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads the first page of accounts
  // on open (no typing needed) and re-pages as the user types. Closed → the
  // query idles (cached pages stay for an instant re-open).
  const {
    accounts,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    isError,
    fetchNextPage,
  } = useInfiniteAccountSearch(query, open);

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

  // Names captured when the user picks an account, so a chip keeps its label
  // even once the search moves on to a different term.
  const [pickedNames, setPickedNames] = useState<Map<string, string>>(
    () => new Map(),
  );

  const nameById = useMemo(() => {
    const m = new Map<string, string>(nameSeed);
    accounts.forEach((a) => {
      if (a.name) m.set(a.id, a.name);
    });
    pickedNames.forEach((name, aid) => m.set(aid, name));
    return m;
  }, [nameSeed, accounts, pickedNames]);

  const selectedOptions: AccountOption[] = useMemo(
    () => values.map((v) => ({ id: v, name: nameById.get(v) ?? v })),
    [values, nameById],
  );

  // Pool = current selection (so the field can render its chips) + the search
  // results, de-duplicated by id.
  const options: AccountOption[] = useMemo(() => {
    const results = accounts.map((a) => ({ id: a.id, name: a.name || a.id }));
    const seen = new Set(values);
    return [...selectedOptions, ...results.filter((o) => !seen.has(o.id))];
  }, [accounts, values, selectedOptions]);

  return (
    <Autocomplete<AccountOption, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      // Spinner only while the first page loads; later pages append on scroll.
      loading={isFetching && accounts.length === 0}
      disableCloseOnSelect
      sx={{
        "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 },
      }}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => {
        setPickedNames((prev) => {
          const m = new Map(prev);
          // A preselected id absent from both nameSeed and the current search
          // results falls back to using its own id as `name` (see
          // selectedOptions above) -- never cache that fallback here, or a
          // later real name for the same id (from a fresh search) would be
          // shadowed by the stale id-as-name value forever.
          next.forEach((o) => {
            if (o.name !== o.id) m.set(o.id, o.name);
          });
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
        isError
          ? "Couldn't load accounts. Try again."
          : isFetching
            ? "Loading accounts…"
            : "No accounts found"
      }
      renderTags={(value) => {
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
            {/* An account name can run long with no space near the end --
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
          placeholder={values.length ? undefined : "Type an account…"}
        />
      )}
    />
  );
}
