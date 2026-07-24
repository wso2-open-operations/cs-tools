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
import { useQuery } from "@tanstack/react-query";
import { projects } from "@src/services/projects";
import type { Project } from "@src/types";
import { useDebouncedValue } from "@utils/useDebouncedValue";

interface ProjectSelectProps {
  value: Project | null;
  onChange: (project: Project | null) => void;
  disabled?: boolean;
}

// Simplified mobile counterpart of the webapp's AsyncProjectSelect.tsx: type-to-search, debounced
// 300ms, top 20 matches (no infinite-scroll paging — a reasonable mobile scope reduction since
// typing narrows the list quickly). The picked project is kept in `options` even if a later
// search no longer returns it, so the field keeps showing its name.
export function ProjectSelect({ value, onChange, disabled }: ProjectSelectProps) {
  const [inputValue, setInputValue] = useState("");
  const debouncedQuery = useDebouncedValue(inputValue, 300);
  const { data, isFetching } = useQuery(projects.search(debouncedQuery.trim()));

  const options = useMemo(() => {
    const results = data ?? [];
    if (value && !results.some((p) => p.id === value.id)) return [value, ...results];
    return results;
  }, [data, value]);

  return (
    <Autocomplete
      options={options}
      value={value}
      loading={isFetching}
      disabled={disabled}
      getOptionLabel={(option) => option.name}
      isOptionEqualToValue={(option, val) => option.id === val.id}
      onChange={(_, next) => onChange(next)}
      onInputChange={(_, next) => setInputValue(next)}
      renderInput={(params) => <TextField {...params} label="Project" size="small" required />}
    />
  );
}
