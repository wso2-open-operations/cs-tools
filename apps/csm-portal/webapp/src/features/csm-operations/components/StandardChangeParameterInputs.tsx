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

import { Box, TextField, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  STANDARD_CHANGE_TEXT_VALUE_MAX,
  STANDARD_CHANGE_TEXTAREA_VALUE_MAX,
  standardChangeValueError,
} from "@features/csm-operations/utils/standardChanges";
import type { BeStandardChangeParameter } from "@api/backend/types";

interface StandardChangeParameterInputsProps {
  parameters: readonly BeStandardChangeParameter[];
  values: Readonly<Record<string, string>>;
  onChange: (key: string, value: string) => void;
  /** Makes each input's id unique on the page. */
  idPrefix: string;
  disabled?: boolean;
  /** Show each value's problem (a required one missing, a malformed number or date). */
  showErrors?: boolean;
}

/**
 * One input per parameter of a standard change template, by its type: a line
 * of text, several lines, a number, a date (YYYY-MM-DD, as the server takes it)
 * or a choice. The template's "change specifics" on the create form; also the
 * sample values of a proposal's live preview.
 */
export default function StandardChangeParameterInputs({
  parameters,
  values,
  onChange,
  idPrefix,
  disabled = false,
  showErrors = false,
}: StandardChangeParameterInputsProps): JSX.Element {
  if (parameters.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        This template has no parameters: everything is specified by the template.
      </Typography>
    );
  }
  return (
    <Box
      sx={{
        display: "grid",
        gap: 2,
        gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" },
        alignItems: "start",
      }}
    >
      {parameters.map((p) => {
        const id = `${idPrefix}-${p.key}`;
        const value = values[p.key] ?? "";
        const error = showErrors ? standardChangeValueError(p, value) : null;
        const helperText = error ?? p.helpText ?? undefined;
        const common = {
          id,
          label: p.label,
          value,
          required: p.required,
          disabled,
          fullWidth: true,
          size: "small" as const,
          error: !!error,
          helperText,
        };
        switch (p.type) {
          case "textarea":
            return (
              <TextField
                key={p.key}
                {...common}
                multiline
                minRows={3}
                onChange={(e) => onChange(p.key, e.target.value.slice(0, STANDARD_CHANGE_TEXTAREA_VALUE_MAX))}
                sx={{ gridColumn: "1 / -1" }}
              />
            );
          case "number":
            return (
              <TextField
                key={p.key}
                {...common}
                onChange={(e) => onChange(p.key, e.target.value)}
                slotProps={{ htmlInput: { inputMode: "decimal" } }}
              />
            );
          case "date":
            return (
              <TextField
                key={p.key}
                {...common}
                type="date"
                onChange={(e) => onChange(p.key, e.target.value)}
                slotProps={{ inputLabel: { shrink: true } }}
              />
            );
          case "select":
            return (
              <TextField
                key={p.key}
                {...common}
                select
                onChange={(e) => onChange(p.key, e.target.value)}
                slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
              >
                <option value="">-- Select --</option>
                {(p.options ?? []).map((o) => (
                  <option key={o} value={o}>
                    {o}
                  </option>
                ))}
              </TextField>
            );
          default:
            return (
              <TextField
                key={p.key}
                {...common}
                onChange={(e) =>
                  // One line: a pasted line break would be refused by the server.
                  onChange(p.key, e.target.value.replace(/[\r\n]+/g, " ").slice(0, STANDARD_CHANGE_TEXT_VALUE_MAX))
                }
              />
            );
        }
      })}
    </Box>
  );
}
