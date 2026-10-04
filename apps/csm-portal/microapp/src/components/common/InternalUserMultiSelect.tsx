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

import { useMemo, useState } from "react";
import { Autocomplete, TextField } from "@wso2/oxygen-ui";
import { useInfiniteQuery } from "@tanstack/react-query";
import { adminUsers } from "@src/services/adminUsers";
import { useDebouncedValue } from "@utils/useDebouncedValue";

export interface UserOption {
  id: string;
  name: string;
}

// The Acrylic theme renders popup papers translucent, so a dropdown that opens over a dialog reads
// as see-through — force the opaque `background.default`.
const OPAQUE_POPUP = { sx: { backgroundColor: "background.default", backgroundImage: "none" } };

// Load the next page when the listbox is scrolled within this many pixels of its end.
const SCROLL_LOAD_THRESHOLD_PX = 48;

/**
 * Async, type-to-search multi-select of internal staff, used for assignee filters. Matches are
 * scoped to the internal roles (never customers or external users) and paged: scrolling the
 * options list loads the next page. Requires at least one typed character.
 */
export function InternalUserMultiSelect<T extends UserOption>({
  value,
  onChange,
  label = "Assignee",
}: {
  value: T[];
  onChange: (users: UserOption[]) => void;
  label?: string;
}) {
  const [input, setInput] = useState("");
  const debounced = useDebouncedValue(input, 300).trim();
  const { data, isFetching, hasNextPage, isFetchingNextPage, fetchNextPage } = useInfiniteQuery(
    adminUsers.searchInternal(debounced),
  );

  const options = useMemo<UserOption[]>(() => {
    const results = data?.pages.flatMap((page) => page.users) ?? [];
    return [...value, ...results.filter((r) => !value.some((v) => v.id === r.id))];
  }, [data, value]);

  return (
    <Autocomplete
      multiple
      size="small"
      options={options}
      value={value}
      loading={isFetching || isFetchingNextPage}
      filterOptions={(opts) => opts}
      getOptionLabel={(o) => o.name}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      onChange={(_, next) => onChange(next.map((o) => ({ id: o.id, name: o.name })))}
      onInputChange={(_, next) => setInput(next)}
      slotProps={{
        paper: OPAQUE_POPUP,
        listbox: {
          onScroll: (event: React.UIEvent<HTMLElement>) => {
            const el = event.currentTarget;
            if (
              hasNextPage &&
              !isFetchingNextPage &&
              el.scrollHeight - el.scrollTop - el.clientHeight < SCROLL_LOAD_THRESHOLD_PX
            ) {
              void fetchNextPage();
            }
          },
        },
      }}
      renderInput={(params) => <TextField {...params} label={label} size="small" placeholder="Search engineers…" />}
    />
  );
}
