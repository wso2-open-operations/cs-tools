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
import { useInfiniteUserSearch } from "@features/csm-cases/api/useUserSearch";

interface UserIdOption {
  id: string;
  name: string;
}

interface AsyncUserIdSingleSelectProps {
  id?: string;
  label?: string;
  /** Selected value: a platform user UUID, or `""` for "no selection" —
   * matches the singular, single-value `approverId` search filter this
   * feeds (unlike {@link AsyncUserIdMultiSelect}'s `userIds`, a genuinely
   * plural filter). */
  value: string;
  onChange: (next: string) => void;
  /** Known id -> name pairs so an already-selected user stays labelled
   * before any search has run (e.g. after switching tabs — see
   * `engineerNameCache` in `CsmTimeCardsPage`, which this mirrors). */
  nameSeed?: Map<string, string>;
  /** Server-side role scoping passed straight through to
   * {@link useInfiniteUserSearch}'s {@link UserSearchScope} — e.g. restrict
   * the directory search to accounts holding the timecard-approver role, so
   * this never offers someone who could never actually be picked as an
   * approver. */
  roleIds?: string[];
  /** See {@link roleIds}; restrict to active accounts only. */
  active?: boolean;
  /** Called whenever this instance resolves an id -> name pair (a directory
   * search result, or an existing selection) — lets a caller that persists
   * its own id -> name cache across this component's unmount (a tab switch)
   * learn names this instance discovered. Mirrors
   * `AsyncUserIdMultiSelect`'s own `onNamesResolved`. */
  onNameResolved?: (id: string, name: string) => void;
  placeholder?: string;
}

/**
 * Single-select twin of {@link AsyncUserIdMultiSelect}: the same
 * backend-search Autocomplete (`useInfiniteUserSearch`, the shared
 * user-directory search every assignee/approver-style picker in this app
 * already uses) but stores at most one platform id, matching a singular
 * search filter (e.g. `approverId`) rather than a plural one.
 */
export default function AsyncUserIdSingleSelect({
  id = "user-id-single-select",
  label = "User",
  value,
  onChange,
  nameSeed,
  roleIds,
  active,
  onNameResolved,
  placeholder,
}: AsyncUserIdSingleSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  const { users: searchResults, isFetching, isFetchingNextPage, hasNextPage, isError, fetchNextPage } =
    useInfiniteUserSearch(query, open, { roleIds, active });
  // `id` is optional on `UserSearchOption` (`POST /users/search` doesn't
  // guarantee it) — this picker filters on the id, so a row without one is
  // unusable here and dropped.
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

  const [pickedName, setPickedName] = useState<[string, string] | undefined>(undefined);

  const nameById = useMemo(() => {
    const m = new Map<string, string>(nameSeed);
    users.forEach((u) => m.set(u.id, u.name));
    if (pickedName) m.set(pickedName[0], pickedName[1]);
    return m;
  }, [nameSeed, users, pickedName]);

  const selectedOption: UserIdOption | null = useMemo(
    () => (value ? { id: value, name: nameById.get(value) ?? value } : null),
    [value, nameById],
  );

  // Pool = current selection (so the field renders its value even before a
  // search has run) + the search results, de-duplicated by id.
  const options: UserIdOption[] = useMemo(() => {
    const results = users.filter((u) => u.id !== value).map((u) => ({ id: u.id, name: u.name }));
    return selectedOption ? [selectedOption, ...results] : results;
  }, [users, value, selectedOption]);

  return (
    <Autocomplete<UserIdOption, false>
      size="small"
      id={id}
      options={options}
      value={selectedOption}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      loading={isFetching && users.length === 0}
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => {
        if (next) {
          setPickedName([next.id, next.name]);
          onNameResolved?.(next.id, next.name);
        }
        onChange(next?.id ?? "");
      }}
      inputValue={input}
      onInputChange={(_event, val, reason) => {
        // "reset" fires when Autocomplete syncs the visible text to the
        // current `value` (on selection, or on mount/re-render with an
        // already-set `value`) — needed here since `inputValue` is
        // controlled; without it, a preset/selected value would never
        // actually show its label in the text field.
        if (reason === "input" || reason === "clear" || reason === "reset") setInput(val);
      }}
      noOptionsText={
        isError
          ? "Couldn't load approvers. Try again."
          : isFetching
            ? "Loading approvers…"
            : "No approvers found"
      }
      renderInput={(params) => (
        <TextField {...params} label={label} placeholder={value ? undefined : placeholder} />
      )}
    />
  );
}
