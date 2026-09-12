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

import { Autocomplete, Box, Checkbox, ListItemText, TextField, Tooltip } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useInfiniteUserSearch } from "@features/csm-cases/api/useUserSearch";

interface AssignedUserOption {
  id: string;
  name: string;
}

interface AsyncAssignedUserIdMultiSelectProps {
  /** Selected engineer platform ids (not emails — `assignedUserId` filters
   * on the id directly, so no email→id resolution step is needed at
   * request-build time). */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * The `anyOf` branch "Assignee" row's value input — the id-valued twin of
 * {@link AsyncCreatedByMultiSelect}: same directory search
 * ({@link useInfiniteUserSearch}), but stores the picked user's id (what
 * `assignedUserId` actually filters on) rather than their email. No `@me`
 * sentinel here (unlike the top-level "Assignee" bar control) — an `anyOf`
 * branch is a comparatively rare, deliberate cross-field OR construction,
 * and "assigned to me" is already reachable via the bar's own Assignee
 * control outside any branch.
 */
export default function AsyncAssignedUserIdMultiSelect({
  values,
  onChange,
}: AsyncAssignedUserIdMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  const { users: searchResults, isFetching, isFetchingNextPage, hasNextPage, isError, fetchNextPage } =
    useInfiniteUserSearch(query, open);
  // `id` is optional on `UserSearchOption` (`POST /users/search` doesn't
  // guarantee it) — this picker filters on the id, so a row without one is
  // unusable here and dropped, unlike the email-keyed pickers.
  const users = useMemo(
    () => searchResults.filter((u): u is typeof u & { id: string } => Boolean(u.id)),
    [searchResults],
  );

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

  const [pickedNames, setPickedNames] = useState<Map<string, string>>(() => new Map());

  const nameById = useMemo(() => {
    const m = new Map<string, string>();
    users.forEach((u) => m.set(u.id, u.name));
    pickedNames.forEach((name, id) => m.set(id, name));
    return m;
  }, [users, pickedNames]);

  const selectedOptions: AssignedUserOption[] = useMemo(
    () => values.map((v) => ({ id: v, name: nameById.get(v) ?? v })),
    [values, nameById],
  );

  const options: AssignedUserOption[] = useMemo(() => {
    const selected = new Set(values);
    const results = users
      .filter((u) => !selected.has(u.id))
      .map((u) => ({ id: u.id, name: u.name }));
    return [...selectedOptions, ...results];
  }, [users, values, selectedOptions]);

  return (
    <Autocomplete<AssignedUserOption, true>
      multiple
      size="small"
      id="any-of-filter-assigned-user"
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      loading={isFetching && users.length === 0}
      disableCloseOnSelect
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
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
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load engineers. Try again."
          : isFetching
            ? "Loading engineers…"
            : "No engineers found"
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
            <ListItemText
              primary={option.name}
              slotProps={{ primary: { style: { fontSize: 13, overflowWrap: "anywhere" } } }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label="Value(s)"
          placeholder={values.length ? undefined : "Search engineers…"}
        />
      )}
    />
  );
}
