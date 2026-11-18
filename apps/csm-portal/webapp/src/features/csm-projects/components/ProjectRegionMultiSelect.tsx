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


import { Autocomplete, Box, Checkbox, Link, ListItemText, TextField, Tooltip } from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import type * as React from "react";
import { useAccountRegions } from "@features/csm-projects/api/useAccountRegions";

interface ProjectRegionMultiSelectProps {
  id: string;
  values: string[];
  onChange: (next: string[]) => void;
}

const sameRegion = (a: string, b: string): boolean => a.trim().toLowerCase() === b.trim().toLowerCase();

/**
 * The projects list's Region value picker. The suggestions are the account
 * regions {@link useAccountRegions} reads, which come from cases' accounts and
 * so can miss a region only project accounts hold; a region can therefore
 * also be typed (Enter adds it). A suggestion and a selected value that
 * differ only in case are one option, keeping the selected spelling, so a
 * region from a link shows as checked. A failed lookup is said under the
 * field, which stays usable: a selected region can still be removed and
 * another typed.
 */
export default function ProjectRegionMultiSelect({
  id,
  values,
  onChange,
}: ProjectRegionMultiSelectProps): JSX.Element {
  const { data: regions, isLoading, isError, refetch } = useAccountRegions(true);
  const options = useMemo(() => {
    const out = [...values];
    for (const r of regions ?? []) {
      if (!out.some((v) => sameRegion(v, r))) out.push(r);
    }
    return out;
  }, [regions, values]);

  const add = (next: string[]): void => {
    const deduped: string[] = [];
    for (const raw of next) {
      const v = raw.trim();
      if (v && !deduped.some((d) => sameRegion(d, v))) deduped.push(v);
    }
    onChange(deduped);
  };

  return (
    <Autocomplete<string, true, false, true>
      multiple
      freeSolo
      size="small"
      id={id}
      options={options}
      value={values}
      loading={isLoading}
      disableCloseOnSelect
      isOptionEqualToValue={(opt, val) => sameRegion(opt, val)}
      onChange={(_event, next) => add(next)}
      noOptionsText={isError ? "Regions couldn't be loaded. Type a region." : "No regions found. Type a region."}
      renderTags={(value) => {
        const displayText = value.join(", ");
        const content = (
          <Box
            component="span"
            sx={{ flex: "1 1 0", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", pl: 1 }}
          >
            {displayText}
          </Box>
        );
        return value.length === 1 ? content : <Tooltip title={displayText} placement="top">{content}</Tooltip>;
      }}
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & { key: string };
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox size="small" checked={selected} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText primary={option} slotProps={{ primary: { style: { fontSize: 13 } } }} />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label="Value(s)"
          placeholder={values.length ? undefined : "Pick or type a region…"}
          error={isError}
          helperText={
            isError ? (
              <>
                Regions couldn&apos;t be loaded; you can still type one.{" "}
                <Link component="button" type="button" onClick={() => void refetch()}>
                  Try again
                </Link>
              </>
            ) : undefined
          }
        />
      )}
    />
  );
}
