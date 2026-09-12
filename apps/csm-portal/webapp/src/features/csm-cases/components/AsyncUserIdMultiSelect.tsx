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

interface UserIdOption {
  id: string;
  name: string;
}

/** True when the "Me" option should show for the current typed term. */
function meMatches(input: string): boolean {
  const t = input.trim().toLowerCase();
  return t === "" || "me".includes(t);
}

interface AsyncUserIdMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected values: platform user UUIDs (not emails) — for a filter field
   * whose downstream contract is UUID-typed (e.g. `/call-requests/search`'s
   * `assignedUserIds`), unlike {@link AsyncAssigneeMultiSelect}'s
   * email-keyed `assignees`, which is what the cases list filters on
   * instead. */
  values: string[];
  onChange: (next: string[]) => void;
  /** Known id -> name pairs so an already-selected user stays labelled
   * before any search has run (e.g. seeded from a dashboard widget's own
   * filters). */
  nameSeed?: Map<string, string>;
  /**
   * The signed-in user's own platform id, so a selected value equal to it
   * can render as "Me" instead of its raw UUID, and picking "Me" from the
   * dropdown stores this id. This field's own contract has no `@me`
   * sentinel of its own (unlike {@link AsyncAssigneeMultiSelect}'s
   * `ASSIGNEE_ME_TOKEN`) — the caller resolves `__current_user__`/`@me` to
   * this real UUID upstream, before this component ever sees the value
   * (see `resolveCurrentUserSentinels`), so recognizing "Me" here means
   * comparing selected UUIDs against this id rather than a literal token.
   * Omit while the current user hasn't resolved yet; already-selected
   * values just render their raw UUID (or a `nameSeed` label) until it has.
   */
  currentUserId?: string;
  /**
   * Server-side role scoping passed straight through to
   * {@link useInfiniteUserSearch}'s {@link UserSearchScope} — e.g. a picker
   * that should only ever offer internal staff (time-card/case-assignment
   * style filters), not a customer contact. Left unset, the search stays
   * unscoped (this component's original behavior).
   */
  roleIds?: string[];
  /** See {@link roleIds}; restrict to active accounts only. */
  active?: boolean;
  /**
   * Called with every id -> name pair this instance resolves (from a
   * directory search result or an existing selection), so a caller that
   * persists its own id -> name cache across this component's unmount
   * (e.g. a tab switch) can learn names this instance discovered but the
   * caller's own data never would have — {@link nameSeed} alone only ever
   * flows one way (caller to component).
   */
  onNamesResolved?: (entries: [string, string][]) => void;
}

/**
 * UUID-keyed twin of {@link AsyncAssigneeMultiSelect}: same backend-search
 * Autocomplete (`useInfiniteUserSearch`, the shared user-directory search
 * every assignee-style picker in this app already uses), but stores each
 * selection's platform id instead of its email.
 */
export default function AsyncUserIdMultiSelect({
  id = "user-id-multi-select",
  label = "Assignee",
  values,
  onChange,
  nameSeed,
  currentUserId,
  roleIds,
  active,
  onNamesResolved,
}: AsyncUserIdMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  const { users: searchResults, isFetching, isFetchingNextPage, hasNextPage, isError, fetchNextPage } =
    useInfiniteUserSearch(query, open, { roleIds, active });
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
    const m = new Map<string, string>(nameSeed);
    users.forEach((u) => m.set(u.id, u.name));
    pickedNames.forEach((name, uid) => m.set(uid, name));
    return m;
  }, [nameSeed, users, pickedNames]);

  const selectedOptions: UserIdOption[] = useMemo(
    () =>
      values.map((v) => ({
        id: v,
        name: v === currentUserId ? "Me" : (nameById.get(v) ?? v),
      })),
    [values, nameById, currentUserId],
  );

  // Pool = current selection (so the field renders its chips) + the search
  // results, de-duplicated by id, with "Me" pinned first when it matches
  // the typed term and isn't already selected — mirrors
  // `AsyncAssigneeMultiSelect`'s own pinned "Me" option, keyed by this
  // field's own id (`currentUserId`) rather than a literal sentinel token.
  const options: UserIdOption[] = useMemo(() => {
    const selected = new Set(values);
    const results = users
      .filter((u) => !selected.has(u.id))
      .map((u) => ({ id: u.id, name: u.name }));
    const base = [...selectedOptions, ...results];
    const showMe = Boolean(currentUserId) && !selected.has(currentUserId!) && meMatches(input);
    const meOption: UserIdOption = { id: currentUserId ?? "", name: "Me" };
    return showMe ? [meOption, ...base] : base;
  }, [users, values, selectedOptions, input, currentUserId]);

  return (
    <Autocomplete<UserIdOption, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      loading={isFetching && users.length === 0}
      disableCloseOnSelect
      sx={{ "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 } }}
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.id === val.id}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => {
        setPickedNames((prev) => {
          const m = new Map(prev);
          next.forEach((o) => {
            if (o.id !== currentUserId) m.set(o.id, o.name);
          });
          return m;
        });
        // Bubble the picked name(s) up before the caller's `values` change
        // takes effect, so a persistent id -> name cache the caller keeps
        // (to survive this component unmounting on e.g. a tab switch)
        // learns names this search just resolved, not only names the
        // caller's own data already knew.
        onNamesResolved?.(
          next.filter((o) => o.id !== currentUserId).map((o) => [o.id, o.name]),
        );
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
            sx={{
              flex: "1 1 0",
              minWidth: 0,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              pl: 1,
            }}
          >
            {displayText}
          </Box>
        );
        return value.length === 1 ? (
          content
        ) : (
          <Tooltip title={displayText} placement="top">
            {content}
          </Tooltip>
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
          label={label}
          placeholder={values.length ? undefined : "Search engineers…"}
        />
      )}
    />
  );
}
