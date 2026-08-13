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
import { useInfiniteUserSearch } from "@features/csm-cases/api/useUserSearch";

interface InitiatorOption {
  /** The value stored in the filter: a directory user's email. */
  email: string;
  /** Display label: the user's name. */
  name: string;
}

interface AsyncInitiatorMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected values: initiator emails. */
  values: string[];
  onChange: (next: string[]) => void;
  /**
   * Known email → name pairs used to label already-selected initiators
   * before any search runs.
   */
  nameSeed?: Map<string, string>;
}

/**
 * Initiator filter for the Conversations tab (`filters.createdBy`): searches
 * the user directory from the backend as the user types, the same
 * type-to-search/scroll-to-page UX as `AsyncAssigneeMultiSelect` (the cases
 * assignee filter). A thin sibling rather than a direct reuse of that
 * component: `AsyncAssigneeMultiSelect` bakes in the `@me` assignee sentinel
 * (`ASSIGNEE_ME_TOKEN`), which is assignee-specific wire semantics the
 * conversations `createdBy` filter doesn't understand — "my conversations"
 * already has its own dedicated `createdByMe` checkbox in
 * `ConversationsFilterBar`. The underlying directory search
 * (`useInfiniteUserSearch`, `POST /users/search`) is entity-agnostic and is
 * reused as-is.
 */
export default function AsyncInitiatorMultiSelect({
  id = "conversations-filter-initiator",
  label = "Initiator",
  values,
  onChange,
  nameSeed,
}: AsyncInitiatorMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads the first page of the
  // directory on open (no typing needed) and re-pages as the user types.
  const {
    users,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    isError,
    fetchNextPage,
  } = useInfiniteUserSearch(query, open);

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

  // Names captured when the user picks someone, so a chip keeps its label
  // even once the search moves on to a different term.
  const [pickedNames, setPickedNames] = useState<Map<string, string>>(
    () => new Map(),
  );

  const nameByEmail = useMemo(() => {
    const m = new Map<string, string>(nameSeed);
    users.forEach((u) => m.set(u.email, u.name));
    pickedNames.forEach((name, email) => m.set(email, name));
    return m;
  }, [nameSeed, users, pickedNames]);

  const selectedOptions: InitiatorOption[] = useMemo(
    () => values.map((v) => ({ email: v, name: nameByEmail.get(v) ?? v })),
    [values, nameByEmail],
  );

  // Pool = current selection (so the field renders its chips) + the search
  // results, de-duplicated by email.
  const options: InitiatorOption[] = useMemo(() => {
    const selected = new Set(values);
    const results = users
      .filter((u) => !selected.has(u.email))
      .map((u) => ({ email: u.email, name: u.name }));
    return [...selectedOptions, ...results];
  }, [users, values, selectedOptions]);

  return (
    <Autocomplete<InitiatorOption, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      // Spinner only while the first page loads; later pages append on scroll.
      loading={isFetching && users.length === 0}
      disableCloseOnSelect
      sx={{
        "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 },
      }}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.email === val.email}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => {
        setPickedNames((prev) => {
          const m = new Map(prev);
          next.forEach((o) => m.set(o.email, o.name));
          return m;
        });
        onChange(next.map((o) => o.email));
      }}
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        // Keep the typed term after a selection (reason "reset") so the user
        // can pick several from one search; clear only on explicit
        // input/clear.
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load users. Try again."
          : isFetching
            ? "Loading users…"
            : "No users found"
      }
      renderTags={(value) => {
        const displayText = value.map((o) => o.name).join(", ");
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
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option.name}
              secondary={option.email}
              slotProps={{
                primary: { style: { fontSize: 13 } },
                secondary: { style: { fontSize: 11 } },
              }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Search by name or email…"}
        />
      )}
    />
  );
}
