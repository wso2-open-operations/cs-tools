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

interface CreatedByOption {
  email: string;
  name: string;
}

interface AsyncCreatedByMultiSelectProps {
  /** Selected engineer/reporter emails. */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * The "Created by" advanced-filter row's value input — a type-to-search
 * multi-value email picker over the same user directory
 * ({@link useInfiniteUserSearch}) `AsyncAssigneeMultiSelect` already searches,
 * so the "email is one of" op suggests real reporters instead of requiring
 * them hand-typed. A stripped-down copy of that component's debounce/search
 * shape — no "Me" pinning (that's the row's own separate "is me" op, backed
 * by `BE_CURRENT_USER_FILTER_PLACEHOLDER`, not a selectable option here).
 */
export default function AsyncCreatedByMultiSelect({
  values,
  onChange,
}: AsyncCreatedByMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  const { users, isFetching, isFetchingNextPage, hasNextPage, isError, fetchNextPage } =
    useInfiniteUserSearch(query, open);

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

  const nameByEmail = useMemo(() => {
    const m = new Map<string, string>();
    users.forEach((u) => m.set(u.email, u.name));
    pickedNames.forEach((name, email) => m.set(email, name));
    return m;
  }, [users, pickedNames]);

  const selectedOptions: CreatedByOption[] = useMemo(
    () => values.map((v) => ({ email: v, name: nameByEmail.get(v) ?? v })),
    [values, nameByEmail],
  );

  const options: CreatedByOption[] = useMemo(() => {
    const selected = new Set(values);
    const results = users
      .filter((u) => !selected.has(u.email))
      .map((u) => ({ email: u.email, name: u.name }));
    return [...selectedOptions, ...results];
  }, [users, values, selectedOptions]);

  return (
    <Autocomplete<CreatedByOption, true>
      multiple
      size="small"
      id="advanced-filter-created-by"
      options={options}
      value={selectedOptions}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      loading={isFetching && users.length === 0}
      disableCloseOnSelect
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
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load people. Try again."
          : isFetching
            ? "Loading…"
            : "No matches"
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
              secondary={option.email}
              slotProps={{
                primary: { style: { fontSize: 13, overflowWrap: "anywhere" } },
                secondary: { style: { fontSize: 11, overflowWrap: "anywhere" } },
              }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label="Value(s)"
          placeholder={values.length ? undefined : "Search people…"}
        />
      )}
    />
  );
}
