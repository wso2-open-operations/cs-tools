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
  Checkbox,
  Chip,
  ListItemText,
  TextField,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type * as React from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useInfiniteUserSearch } from "@features/csm-cases/api/useUserSearch";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/utils/assignee";

interface AssigneeOption {
  /** The value stored in the filter: an engineer email, or the `@me` sentinel. */
  email: string;
  /** Display label: the engineer's name, "Me" for the sentinel. */
  name: string;
}

/** True when the `@me` ("Me") option should show for the current typed term. */
function meMatches(input: string): boolean {
  const t = input.trim().toLowerCase();
  return t === "" || "me".includes(t);
}

interface AsyncAssigneeMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected values: engineer emails plus the optional `@me` sentinel. */
  values: string[];
  onChange: (next: string[]) => void;
  /**
   * Known email → name pairs (e.g. from the directory prefetch or the cases on
   * screen) used to label already-selected engineers before any search runs.
   */
  nameSeed?: Map<string, string>;
}

/**
 * Assignee filter that searches the user directory from the backend as the user
 * types — the assignee-side twin of {@link AsyncProjectMultiSelect}. Replaces
 * the old client-side filter over a one-shot, capped user list (which could only
 * find people in the first page it loaded). A pinned "Me" option maps to the
 * `@me` sentinel; selected engineer names are remembered (captured at selection
 * time, plus any seed) so the chips stay labelled even after results change.
 */
export default function AsyncAssigneeMultiSelect({
  id = "cases-filter-assignee",
  label = "Assignee",
  values,
  onChange,
  nameSeed,
}: AsyncAssigneeMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads the first page of engineers
  // on open (no typing needed) and re-pages as the user types.
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

  // Names captured when the user picks an engineer, so a chip keeps its label
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

  const selectedOptions: AssigneeOption[] = useMemo(
    () =>
      values.map((v) => ({
        email: v,
        name: v === ASSIGNEE_ME_TOKEN ? "Me" : (nameByEmail.get(v) ?? v),
      })),
    [values, nameByEmail],
  );

  // Pool = current selection (so the field renders its chips) + the search
  // results, de-duplicated by email, with "Me" pinned first when it matches the
  // typed term and isn't already selected.
  const options: AssigneeOption[] = useMemo(() => {
    const selected = new Set(values);
    const results = users
      .filter((u) => !selected.has(u.email))
      .map((u) => ({ email: u.email, name: u.name }));
    const base = [...selectedOptions, ...results];
    const showMe = !selected.has(ASSIGNEE_ME_TOKEN) && meMatches(input);
    const meOption: AssigneeOption = { email: ASSIGNEE_ME_TOKEN, name: "Me" };
    return showMe ? [meOption, ...base] : base;
  }, [users, values, selectedOptions, input]);

  return (
    <Autocomplete<AssigneeOption, true>
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
      // The backend already filtered by the typed term; don't re-filter locally
      // (that would also drop the manually-pinned "Me" option).
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt.name}
      isOptionEqualToValue={(opt, val) => opt.email === val.email}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => {
        setPickedNames((prev) => {
          const m = new Map(prev);
          next.forEach((o) => {
            if (o.email !== ASSIGNEE_ME_TOKEN) m.set(o.email, o.name);
          });
          return m;
        });
        onChange(next.map((o) => o.email));
      }}
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        // Keep the typed term after a selection (reason "reset") so the user can
        // pick several from one search; clear only on explicit input/clear.
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load engineers. Try again."
          : isFetching
            ? "Loading engineers…"
            : "No engineers found"
      }
      renderTags={(value, getTagProps) =>
        value.map((option, index) => {
          const { key, ...tagProps } = getTagProps({ index });
          return (
            <Chip key={key} size="small" label={option.name} {...tagProps} />
          );
        })
      }
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        const secondary =
          option.email === ASSIGNEE_ME_TOKEN ? undefined : option.email;
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option.name}
              secondary={secondary}
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
          placeholder={values.length ? undefined : "Search engineers…"}
        />
      )}
    />
  );
}
