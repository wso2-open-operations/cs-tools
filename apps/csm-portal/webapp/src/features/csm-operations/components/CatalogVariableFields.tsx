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
  AdapterDateFns,
  Box,
  DatePickers,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";
import type { BeCatalogItemVariable } from "@api/backend/types";
import {
  isChoiceField,
  isDateTimeField,
  isDescriptionField,
  isFileCopyPathField,
  isMultiLineField,
  variableLabel,
} from "@features/csm-operations/utils/catalogVariables";
import { formatDateTimeLocal, parseDateTimeLocal } from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface CatalogVariableFieldsProps {
  /** Already filtered to user-editable variables (no context/hidden fields). */
  variables: BeCatalogItemVariable[];
  values: Record<string, string>;
  onChange: (id: string, value: string) => void;
}

/**
 * Renders a catalog item's user-editable variables, one input per variable,
 * picking the control from the ServiceNow variable `type`/label: Yes/No select,
 * multi-line text, datetime picker, rich-text Description, or single-line text.
 * File Copy Path fields are optional; attachment fields are handled by the
 * page's shared attachments section and are not rendered here.
 */
export default function CatalogVariableFields({
  variables,
  values,
  onChange,
}: CatalogVariableFieldsProps): JSX.Element {
  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <Grid container spacing={2.5}>
        {variables.map((v) => {
          const value = values[v.id] ?? "";
          const label = variableLabel(v);

          if (isChoiceField(v)) {
            const labelId = `sr-var-${v.id}-label`;
            return (
              <Grid key={v.id} size={{ xs: 12, sm: 6 }}>
                <FormControl fullWidth size="small" required>
                  <InputLabel id={labelId}>{label}</InputLabel>
                  <Select
                    labelId={labelId}
                    label={label}
                    value={value}
                    onChange={(e) => onChange(v.id, String(e.target.value))}
                  >
                    <MenuItem value="Yes">Yes</MenuItem>
                    <MenuItem value="No">No</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
            );
          }

          if (isDescriptionField(v.questionText ?? "")) {
            return (
              <Grid key={v.id} size={{ xs: 12 }}>
                <Typography
                  component="label"
                  variant="caption"
                  color="text.secondary"
                  sx={{ display: "block", mb: 0.5 }}
                >
                  {label} *
                </Typography>
                <Box role="group" aria-label={label}>
                  <Editor
                    value={value}
                    onChange={(html) => onChange(v.id, html)}
                    placeholder=""
                    minHeight={150}
                    maxHeight="300px"
                    toolbarVariant="full"
                  />
                </Box>
              </Grid>
            );
          }

          if (isMultiLineField(v)) {
            return (
              <Grid key={v.id} size={{ xs: 12 }}>
                <TextField
                  label={label}
                  size="small"
                  fullWidth
                  required
                  multiline
                  minRows={4}
                  value={value}
                  onChange={(e) => onChange(v.id, e.target.value)}
                />
              </Grid>
            );
          }

          if (isDateTimeField(v)) {
            return (
              <Grid key={v.id} size={{ xs: 12, sm: 6 }}>
                <DateTimePicker
                  label={label}
                  value={parseDateTimeLocal(value)}
                  onChange={(next) =>
                    onChange(
                      v.id,
                      next instanceof Date && !Number.isNaN(next.getTime())
                        ? formatDateTimeLocal(next)
                        : "",
                    )
                  }
                  slotProps={{
                    textField: { size: "small", fullWidth: true, required: true },
                    field: { clearable: true },
                  }}
                />
              </Grid>
            );
          }

          // File Copy Path is an optional plain text input.
          const optional = isFileCopyPathField(v);
          return (
            <Grid key={v.id} size={{ xs: 12 }}>
              <TextField
                label={label}
                size="small"
                fullWidth
                required={!optional}
                value={value}
                onChange={(e) => onChange(v.id, e.target.value)}
              />
            </Grid>
          );
        })}
      </Grid>
    </LocalizationProvider>
  );
}
