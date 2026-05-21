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
import type { ChangeEvent, ReactNode } from "react";

import {
  FormControl,
  FormHelperText,
  MenuItem,
  TextField as MuiTextField,
  Select,
  type SelectChangeEvent,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useField } from "formik";

interface SelectFieldProps {
  name: string;
  options: { value: number | string; label: string | ReactNode }[];
  label?: string | ReactNode;
  placeholder?: string;
  helperText?: string;
  required?: boolean;
  disabled?: boolean;
  slots?: {
    label?: { startAdornment?: ReactNode; endAdornment?: ReactNode };
    input?: { startAdornment?: ReactNode };
  };

  onChange?: (event: SelectChangeEvent) => void;
}
export function SelectField({
  name,
  label,
  placeholder,
  helperText,
  options,
  required = false,
  disabled = false,
  slots,
  onChange,
}: SelectFieldProps) {
  const [field, meta] = useField(name);

  return (
    <FormControl component={Stack} gap={1} fullWidth>
      <FieldLabel
        label={label}
        required={required}
        startAdornment={slots?.label?.startAdornment}
        endAdornment={slots?.label?.endAdornment}
      />

      <Select
        {...field}
        error={meta.touched && Boolean(meta.error)}
        displayEmpty={placeholder !== undefined}
        sx={{ bgcolor: "background.paper" }}
        startAdornment={slots?.input?.startAdornment}
        disabled={disabled}
        onChange={(event) => {
          field.onChange(event);
          onChange?.(event);
        }}
        renderValue={(selected) => {
          if (typeof selected === "string" && selected.length === 0) {
            return (
              <Typography variant="body2" color="text.secondary">
                {placeholder}
              </Typography>
            );
          }

          return options.find((option) => option.value === selected)?.label ?? selected;
        }}
      >
        {options.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </Select>

      {meta.touched && meta.error ? (
        <FormHelperText error sx={{ m: 0, mt: -0.5 }}>
          {meta.error}
        </FormHelperText>
      ) : helperText ? (
        <FormHelperText sx={{ m: 0, mt: -0.5 }}>{helperText}</FormHelperText>
      ) : null}
    </FormControl>
  );
}

interface TextFieldProps {
  name: string;
  value?: string;
  label?: string;
  placeholder?: string;
  helperText?: string;
  multiline?: boolean;
  rows?: number;
  required?: boolean;
  disabled?: boolean;
  slots?: {
    label?: { startAdornment?: ReactNode; endAdornment?: ReactNode };
    input?: { startAdornment?: ReactNode };
  };

  onChange?: (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
}

export function TextField({
  name,
  value,
  label,
  placeholder,
  helperText,
  multiline = false,
  rows = 10,
  required = false,
  disabled = false,
  slots,
  onChange,
}: TextFieldProps) {
  const [field, meta] = useField(name);

  return (
    <FormControl component={Stack} gap={1} fullWidth>
      <FieldLabel
        label={label}
        required={required}
        startAdornment={slots?.label?.startAdornment}
        endAdornment={slots?.label?.endAdornment}
      />

      <MuiTextField
        {...field}
        value={value}
        error={meta.touched && Boolean(meta.error)}
        placeholder={placeholder}
        multiline={multiline}
        rows={rows}
        disabled={disabled}
        sx={{
          lineHeight: multiline ? 1.65 : undefined,
          "& .MuiOutlinedInput-root": { bgcolor: "background.paper" },
        }}
        slotProps={{ input: { startAdornment: slots?.input?.startAdornment } }}
        onChange={(event) => {
          field.onChange(event);
          onChange?.(event);
        }}
      />

      {meta.touched && meta.error ? (
        <FormHelperText error sx={{ m: 0, mt: -0.5 }}>
          {meta.error}
        </FormHelperText>
      ) : helperText ? (
        <FormHelperText sx={{ m: 0, mt: -0.5 }}>{helperText}</FormHelperText>
      ) : null}
    </FormControl>
  );
}

interface FieldLabelProps {
  label: string | ReactNode;
  required?: boolean;
  startAdornment?: ReactNode;
  endAdornment?: ReactNode;
}

function FieldLabel({ label, required, startAdornment, endAdornment }: FieldLabelProps) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="end" gap={1}>
      {/* Optional element displayed at the beginning of the label row */}
      {startAdornment}

      <Stack direction="row" alignItems="center" gap={0.5}>
        <Typography variant="subtitle2">{label}</Typography>
        {required && (
          <Typography variant="h5" component="span" color="error">
            *
          </Typography>
        )}
      </Stack>

      {/* Optional element displayed at the end of the label row */}
      {endAdornment}
    </Stack>
  );
}
