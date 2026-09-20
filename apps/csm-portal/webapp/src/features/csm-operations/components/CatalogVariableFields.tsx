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
  isMultiLineField,
  isVariableRequired,
  usableChoices,
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
 * Renders a catalog item's user-editable variables, one input per variable.
 * A variable that carries a usable choice list becomes a real select over
 * those options; otherwise the control comes from the variable `type`/label:
 * Yes/No select, multi-line text, datetime picker, rich-text Description, or
 * single-line text. File Copy Path fields are optional; attachment fields are
 * handled by the page's shared attachments section and are not rendered here.
 *
 * Respects the additive per-variable metadata from
 * `CHANGES-sr-variable-metadata.md` (`BeCatalogItemVariable`): required-ness
 * comes from `isVariableRequired` (the real `mandatory` flag when the
 * backend supplies one, falling back to the old hot fix otherwise);
 * `readOnly` disables the control rather than hiding it (the value is still
 * shown so the engineer can see what will be submitted); `maxLength`
 * constrains free-text inputs; a declared `validation` regex is checked
 * client-side and its `message` shown as the field's error text.
 * `active`/`hidden` are NOT re-checked here — the caller (`variables` prop)
 * is expected to already be filtered through `getUserEditableVariables`,
 * which excludes both.
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
          const required = isVariableRequired(v);
          const readOnly = v.readOnly === true;
          // Checked client-side only when there's something to check and a
          // rule to check it against — matches
          // `getFirstFieldFailingValidation`'s own skip-when-empty rule so
          // the per-field error here and the page-level submit gate agree.
          let validationError: string | undefined;
          if (v.validation?.regex && value.trim()) {
            try {
              if (!new RegExp(v.validation.regex).test(value.trim())) {
                validationError = v.validation.message;
              }
            } catch {
              // Malformed backend regex — don't let it block the field.
            }
          }

          // A supplied choice list wins over every type heuristic below: it
          // is the backing data source's own answer set for this question, so
          // the field must offer exactly those options. `choices` is omitted
          // for non-choice variables, and an all-malformed list yields none —
          // both fall through to the heuristics rather than rendering an
          // empty dropdown.
          const choices = usableChoices(v);
          if (choices.length > 0) {
            const labelId = `sr-var-${v.id}-label`;
            return (
              <Grid key={v.id} size={{ xs: 12, sm: 6 }}>
                <FormControl fullWidth size="small" required={required} disabled={readOnly}>
                  <InputLabel id={labelId} shrink={value !== ""} sx={{ top: "0px !important" }}>
                    {label}
                  </InputLabel>
                  <Select
                    labelId={labelId}
                    label={label}
                    value={value}
                    onChange={(e) => onChange(v.id, String(e.target.value))}
                    notched={value !== ""}
                  >
                    {choices.map((c) => (
                      // The submitted value is the option's `value`; `text`
                      // is display only.
                      <MenuItem key={c.value} value={c.value}>
                        {c.label}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </Grid>
            );
          }

          if (isChoiceField(v)) {
            const labelId = `sr-var-${v.id}-label`;
            return (
              <Grid key={v.id} size={{ xs: 12, sm: 6 }}>
                <FormControl fullWidth size="small" required={required} disabled={readOnly}>
                  <InputLabel id={labelId} shrink={value !== ""} sx={{ top: "0px !important" }}>
                    {label}
                  </InputLabel>
                  <Select
                    labelId={labelId}
                    label={label}
                    value={value}
                    onChange={(e) => onChange(v.id, String(e.target.value))}
                    notched={value !== ""}
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
                  {label} {required ? "*" : ""}
                </Typography>
                <Box role="group" aria-label={label}>
                  <Editor
                    value={value}
                    onChange={(html) => onChange(v.id, html)}
                    placeholder=""
                    minHeight={150}
                    maxHeight="300px"
                    toolbarVariant="full"
                    disabled={readOnly}
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
                  required={required}
                  disabled={readOnly}
                  multiline
                  minRows={4}
                  value={value}
                  onChange={(e) => onChange(v.id, e.target.value)}
                  error={!!validationError}
                  helperText={validationError}
                  slotProps={
                    v.maxLength ? { htmlInput: { maxLength: v.maxLength } } : undefined
                  }
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
                  disabled={readOnly}
                  slotProps={{
                    textField: { size: "small", fullWidth: true, required },
                    field: { clearable: true },
                  }}
                />
              </Grid>
            );
          }

          return (
            <Grid key={v.id} size={{ xs: 12 }}>
              <TextField
                label={label}
                size="small"
                fullWidth
                required={required}
                disabled={readOnly}
                value={value}
                onChange={(e) => onChange(v.id, e.target.value)}
                error={!!validationError}
                helperText={validationError}
                slotProps={v.maxLength ? { htmlInput: { maxLength: v.maxLength } } : undefined}
              />
            </Grid>
          );
        })}
      </Grid>
    </LocalizationProvider>
  );
}
