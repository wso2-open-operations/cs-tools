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
  Chip,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import type { ProjectContact } from "@features/settings/types/users";

type ContactOption = {
  label: string;
  value: string;
};

type WatchListSectionProps = {
  contacts: ProjectContact[];
  selectedEmails: string[];
  onChange: (emails: string[]) => void;
  isLoading?: boolean;
};

/**
 * Renders the Watch List section for case creation.
 * Only contacts with Admin or Portal User roles are shown as options.
 *
 * @returns {JSX.Element} The Watch List section.
 */
export function WatchListSection({
  contacts,
  selectedEmails,
  onChange,
  isLoading = false,
}: WatchListSectionProps): JSX.Element {
  const options: ContactOption[] = useMemo(
    () =>
      contacts
        .map((c) => ({
          label: `${c.firstName} ${c.lastName}`.trim() || c.email,
          value: c.email,
        }))
        .sort((a, b) => a.label.localeCompare(b.label)),
    [contacts],
  );

  const selectedOptions = selectedEmails
    .map((email) => options.find((o) => o.value === email))
    .filter((o): o is ContactOption => o !== undefined);

  return (
    <Paper sx={{ p: 3 }}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 3,
        }}
      >
        <Typography variant="h6">Watch List</Typography>
      </Box>

      <Box>
        <Box sx={{ mb: 1 }}>
          <Typography variant="caption">
            Watchers{" "}
            <Typography variant="caption" color="text.secondary">
              (users who will receive case notifications)
            </Typography>
          </Typography>
        </Box>
        <Autocomplete
          multiple
          loading={isLoading}
          loadingText="Loading..."
          options={options}
          getOptionLabel={(option) => option.label}
          value={selectedOptions}
          onChange={(_event, newValue) => {
            onChange(newValue.map((o) => o.value));
          }}
          disableCloseOnSelect
          isOptionEqualToValue={(option, val) => option.value === val.value}
          renderOption={(props, option, { selected }) => {
            const { key, ...rest } = props;
            return (
              <li key={key} {...rest}>
                <Checkbox
                  size="small"
                  checked={selected}
                  sx={{ mr: 1, p: 0.5 }}
                />
                {option.label}
              </li>
            );
          }}
          renderTags={(tagValue, getTagProps) =>
            tagValue.map((option, index) => {
              const { key, ...tagProps } = getTagProps({ index });
              return (
                <Chip key={key} label={option.label} size="small" {...tagProps} />
              );
            })
          }
          renderInput={(params) => (
            <TextField
              {...params}
              size="small"
              placeholder={selectedOptions.length === 0 ? "Add watchers..." : ""}
            />
          )}
        />
      </Box>
    </Paper>
  );
}
