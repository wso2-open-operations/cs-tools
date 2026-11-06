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


import { Box, MenuItem, TextField } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { VersionFormValues } from "@features/csm-products/utils/catalogChange";
import { SUPPORT_STATUSES, SUPPORT_STATUS_LABEL } from "@features/csm-products/utils/catalogFormat";

type SupportKey =
  | "currentSupportStatus"
  | "releaseDate"
  | "earliestPossibleSupportEolDate"
  | "supportEolAnnouncementDate"
  | "supportEolDate";

interface VersionSupportFieldsProps {
  form: VersionFormValues;
  onChange: (key: SupportKey, value: string) => void;
  errors: Record<string, string>;
  /** Errors show only once the user has tried to save. */
  showErrors: boolean;
  /** Off in the Edit dialog: the release date is set when a version is added. */
  showReleaseDate?: boolean;
}

/**
 * The support status, type and dates of a version, in a two-column grid —
 * shared by the Add product version and Edit version dialogs.
 */
export default function VersionSupportFields({
  form,
  onChange,
  errors,
  showErrors,
  showReleaseDate = true,
}: VersionSupportFieldsProps): JSX.Element {
  const props = (key: SupportKey, label: string) => ({
    label,
    value: form[key],
    onChange: (event: { target: { value: string } }) => onChange(key, event.target.value),
    error: showErrors && Boolean(errors[key]),
    helperText: showErrors ? errors[key] : undefined,
    size: "small" as const,
    fullWidth: true,
  });
  const date = (key: SupportKey, label: string) => ({
    ...props(key, label),
    type: "date",
    slotProps: { inputLabel: { shrink: true } },
  });

  return (
    <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" } }}>
      <TextField select {...props("currentSupportStatus", "Support status")}>
        <MenuItem value="">Not set</MenuItem>
        {SUPPORT_STATUSES.map((s) => (
          <MenuItem key={s} value={s}>
            {SUPPORT_STATUS_LABEL[s]}
          </MenuItem>
        ))}
      </TextField>
      {showReleaseDate && <TextField {...date("releaseDate", "Release date")} />}
      <TextField {...date("earliestPossibleSupportEolDate", "Earliest possible EOL")} />
      <TextField {...date("supportEolAnnouncementDate", "EOL announcement")} />
      <TextField {...date("supportEolDate", "EOL date")} />
    </Box>
  );
}
