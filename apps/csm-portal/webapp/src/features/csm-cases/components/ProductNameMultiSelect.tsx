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
import { useInfiniteProductSearch } from "@features/csm-cases/api/useProductSearch";

interface ProductNameMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected product family names (the filter values themselves). */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * Product filter for the cases list — the product-name twin of
 * {@link AsyncProjectMultiSelect}. Loads the first page of distinct product
 * family names on open (no typing needed); {@link useInfiniteProductSearch}
 * then continues paging the rest in the background (bounded, see
 * `MAX_AUTO_PAGES`) so the list fills in without the user needing to scroll
 * first — `handleListboxScroll` below is a scroll-triggered fallback for
 * paging past that cap. Narrows server-side as the user types. The selected
 * values ARE the names,
 * which flow straight into `filters.productNames` (ServiceNow matches
 * `product.name`, so every version of a product is included) — unlike
 * Project/Assignee there's no id to resolve to a display name, so an
 * already-selected name is simply unioned into the option pool directly.
 */
export default function ProductNameMultiSelect({
  id = "cases-filter-product",
  label = "Product",
  values,
  onChange,
}: ProductNameMultiSelectProps): JSX.Element {
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const debounced = useDebouncedValue(input, 300);
  const query = debounced.trim();

  // Enabled while the dropdown is open, so it loads the first page of product
  // names on open (no typing needed) and re-pages as the user types.
  const {
    productNames,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    isError,
    fetchNextPage,
  } = useInfiniteProductSearch(query, open);

  // Lazy-load the next page when the listbox is scrolled near its end. Also
  // guard on `!isFetching`, not just `!isFetchingNextPage`: while a new
  // search query's first page is loading, `isFetchingNextPage` is already
  // false and `hasNextPage` can still reflect the previous query's state
  // (kept visible via `placeholderData`) — without this, a scroll during
  // that window could fetch a "next page" for a query that's about to be
  // replaced entirely.
  const handleListboxScroll = (event: React.UIEvent<HTMLElement>): void => {
    const el = event.currentTarget;
    if (
      hasNextPage &&
      !isFetching &&
      !isFetchingNextPage &&
      el.scrollHeight - el.scrollTop - el.clientHeight < 80
    ) {
      fetchNextPage();
    }
  };

  // Union of the fetched names and any current selection, so selected chips
  // stay valid even if a name isn't in the currently-loaded pages yet.
  const options: string[] = useMemo(() => {
    const merged = new Set<string>(productNames);
    values.forEach((v) => merged.add(v));
    return [...merged].sort((a, b) => a.localeCompare(b));
  }, [productNames, values]);

  return (
    <Autocomplete<string, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={values}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      // Spinner only while the first page loads; later pages append on scroll.
      loading={isFetching && productNames.length === 0}
      disableCloseOnSelect
      sx={{
        "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap", minHeight: 40 },
      }}
      // The backend already filtered by the typed term; don't re-filter locally.
      filterOptions={(opts) => opts}
      getOptionLabel={(opt) => opt}
      isOptionEqualToValue={(opt, val) => opt === val}
      slotProps={{ listbox: { onScroll: handleListboxScroll } }}
      onChange={(_event, next) => onChange(next)}
      inputValue={input}
      onInputChange={(_event, value, reason) => {
        // Keep the typed term after a selection (reason "reset") so the user can
        // pick several from one search; clear only on explicit input/clear.
        if (reason === "input" || reason === "clear") setInput(value);
      }}
      noOptionsText={
        isError
          ? "Couldn't load products. Try again."
          : isFetching
            ? "Loading products…"
            : "No products found"
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
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText
              primary={option}
              slotProps={{ primary: { style: { fontSize: 13 } } }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Type a product…"}
        />
      )}
    />
  );
}
