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

import { Autocomplete, Box, TextField, Tooltip } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface TagsMultiSelectProps {
  id?: string;
  label?: string;
  /** Selected free-text tag labels (the filter values themselves). */
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * Tag filter for the cases list. Unlike {@link ProductNameMultiSelect} (a
 * bounded catalogue), tags are genuinely free-text on the backing data source
 * (SN's generic label mechanism, e.g. `micro-gw`, `ws-policy`) — there is no
 * "list all tags" endpoint to seed suggestions from, so this is a plain
 * `freeSolo` multi-value input: type a label and press Enter/comma to add it,
 * no fixed option list.
 */
export default function TagsMultiSelect({
  id = "cases-filter-tags",
  label = "Tags",
  values,
  onChange,
}: TagsMultiSelectProps): JSX.Element {
  return (
    <Autocomplete<string, true, false, true>
      multiple
      freeSolo
      size="small"
      id={id}
      options={[]}
      value={values}
      sx={{ "& .MuiAutocomplete-inputRoot": { flexWrap: "nowrap" } }}
      onChange={(_event, next) =>
        onChange(next.map((v) => v.trim()).filter((v) => v.length > 0))
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
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : "Type a tag and press Enter…"}
        />
      )}
    />
  );
}
