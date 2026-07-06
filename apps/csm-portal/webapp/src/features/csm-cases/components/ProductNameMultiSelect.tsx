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
import { useMemo, type JSX } from "react";
import type * as React from "react";
import { useProductNameOptions } from "@features/csm-cases/api/useProductNameOptions";

interface ProductNameMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected product family names (the filter values themselves). */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * Product filter for the cases list. Products are a bounded catalogue, so the
 * distinct family names are fetched once ({@link useProductNameOptions}) and
 * filtered locally as the user types. The selected values ARE the names, which
 * flow straight into `filters.productNames` (ServiceNow matches `product.name`,
 * so every version of a product is included). Already-selected names are kept
 * in the option pool so their chips render even before the fetch resolves.
 */
export default function ProductNameMultiSelect({
  id = "cases-filter-product",
  label = "Product",
  values,
  onChange,
}: ProductNameMultiSelectProps): JSX.Element {
  const { data, isFetching, isError } = useProductNameOptions();

  // Union of the fetched names and any current selection, so selected chips
  // stay valid even if a name isn't in the (cached) fetch yet.
  const options: string[] = useMemo(() => {
    const merged = new Set<string>(data ?? []);
    values.forEach((v) => merged.add(v));
    return [...merged].sort((a, b) => a.localeCompare(b));
  }, [data, values]);

  return (
    <Autocomplete<string, true>
      multiple
      size="small"
      id={id}
      options={options}
      value={values}
      loading={isFetching && !data}
      disableCloseOnSelect
      getOptionLabel={(opt) => opt}
      isOptionEqualToValue={(opt, val) => opt === val}
      onChange={(_event, next) => onChange(next)}
      noOptionsText={
        isError
          ? "Couldn't load products. Try again."
          : isFetching
            ? "Loading products…"
            : "No products found"
      }
      renderTags={(value, getTagProps) =>
        value.map((option, index) => {
          const { key, ...tagProps } = getTagProps({ index });
          return <Chip key={key} size="small" label={option} {...tagProps} />;
        })
      }
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
