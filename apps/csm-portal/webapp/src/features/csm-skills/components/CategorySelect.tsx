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

import { Autocomplete, Box, TextField } from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import { buildCategoryTree, flattenCategoryTree, type FlatCategory } from "@features/csm-skills/utils/categoryTree";
import type { SkillCategory } from "@features/csm-skills/types";

interface BaseProps {
  id: string;
  label: string;
  categories: readonly SkillCategory[] | undefined;
  loading?: boolean;
  disabled?: boolean;
  helperText?: string;
}

function useFlat(categories: readonly SkillCategory[] | undefined): FlatCategory[] {
  return useMemo(() => flattenCategoryTree(buildCategoryTree(categories ?? [])), [categories]);
}

function renderTreeOption(
  props: React.HTMLAttributes<HTMLLIElement> & { key?: string },
  option: FlatCategory,
): JSX.Element {
  const { key, ...rest } = props;
  return (
    <Box component="li" key={key} {...rest} sx={{ pl: `${16 + option.depth * 16}px !important` }}>
      {option.category.name}
    </Box>
  );
}

const sameCategory = (a: FlatCategory, b: FlatCategory): boolean => a.category.id === b.category.id;
const labelOf = (o: FlatCategory): string => o.category.name;

/**
 * Single category picker over the category tree: options appear in tree order,
 * indented by depth, and typing filters them by name. A parent category covers
 * its descendants when used as a search filter (the backend does that).
 */
export function CategorySelect({
  value,
  onChange,
  ...base
}: BaseProps & { value: string; onChange: (id: string) => void }): JSX.Element {
  const flat = useFlat(base.categories);
  const selected = flat.find((o) => o.category.id === value) ?? null;
  return (
    <Autocomplete<FlatCategory>
      fullWidth
      size="small"
      id={base.id}
      options={flat}
      value={selected}
      loading={base.loading}
      disabled={base.disabled}
      getOptionLabel={labelOf}
      isOptionEqualToValue={sameCategory}
      renderOption={renderTreeOption}
      onChange={(_e, next) => onChange(next ? next.category.id : "")}
      renderInput={(params) => (
        <TextField {...params} label={base.label} helperText={base.helperText} />
      )}
    />
  );
}

/** Multi category picker over the category tree, used where a skill is filed under several categories. */
export function CategoryMultiSelect({
  values,
  onChange,
  ...base
}: BaseProps & { values: string[]; onChange: (ids: string[]) => void }): JSX.Element {
  const flat = useFlat(base.categories);
  const selected = flat.filter((o) => values.includes(o.category.id));
  return (
    <Autocomplete<FlatCategory, true>
      multiple
      fullWidth
      size="small"
      id={base.id}
      options={flat}
      value={selected}
      loading={base.loading}
      disabled={base.disabled}
      disableCloseOnSelect
      getOptionLabel={labelOf}
      isOptionEqualToValue={sameCategory}
      renderOption={renderTreeOption}
      onChange={(_e, next) => onChange(next.map((o) => o.category.id))}
      renderInput={(params) => (
        <TextField {...params} label={base.label} helperText={base.helperText} />
      )}
    />
  );
}
