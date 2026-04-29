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
  FormControl,
  TextField as MuiTextField,
  MenuItem,
  Select,
  Stack,
  type SelectChangeEvent,
  Chip,
  alpha,
  pxToRem,
  Typography,
  FormHelperText,
} from "@wso2/oxygen-ui";
import { Sparkle } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

interface SelectFieldProps {
  name: string;
  label: string | ReactNode;
  options: { value: number | string; label: string | ReactNode }[];
  value?: number | string;
  required?: boolean;
  aiLabel?: string;
  placeholder?: string;
  startAdornment?: React.ReactNode;
  disabled?: boolean;
  error?: boolean;
  helperText?: string;
  onChange?: (event: SelectChangeEvent<number | string>) => void;
}

export function SelectField({
  name,
  label,
  options,
  value = 0,
  disabled = false,
  required = false,
  error = false,
  helperText,
  aiLabel,
  placeholder,
  startAdornment,
  onChange,
}: SelectFieldProps) {
  const seen = new Set();
  options = options.filter((option) => {
    if (seen.has(option.value)) return false;
    else {
      seen.add(option.value);
      return true;
    }
  });

  return (
    <FormControl component={Stack} gap={1} fullWidth>
      <Stack direction="row" justifyContent="space-between" alignItems="end" gap={1}>
        <Stack direction="row" alignItems="center" gap={0.5}>
          <Typography variant="subtitle2">{label}</Typography>
          {required && (
            <Typography variant="h5" component="span" color="error">
              *
            </Typography>
          )}
        </Stack>
        {aiLabel && <AILabel label={aiLabel} />}
      </Stack>
      <Select
        error={error}
        displayEmpty={placeholder !== undefined}
        name={name}
        value={value}
        sx={{ bgcolor: "background.paper" }}
        startAdornment={startAdornment}
        onChange={onChange}
        disabled={disabled}
        renderValue={(selected) => {
          if (typeof selected === "string" && selected.length === 0)
            return (
              <Typography variant="body2" color="text.secondary">
                {placeholder}
              </Typography>
            );
          return options.find((option) => option.value === selected)?.label ?? selected;
        }}
      >
        {options.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </Select>
      {helperText && (
        <FormHelperText error={error} sx={{ m: 0, mt: -0.5 }}>
          {helperText}
        </FormHelperText>
      )}
    </FormControl>
  );
}

export function TextField({
  name,
  label,
  value,
  multiline = false,
  rows = 10,
  required = false,
  error = false,
  placeholder,
  helperText,
  aiLabel,
  startAdornment,
  disabled = false,
  onChange,
}: {
  name: string;
  label: string;
  value: string;
  placeholder?: string;
  multiline?: boolean;
  rows?: number;
  required?: boolean;
  aiLabel?: string;
  startAdornment?: React.ReactNode;
  error?: boolean;
  helperText?: string;
  disabled?: boolean;

  onChange?: (event: React.ChangeEvent<HTMLInputElement>) => void;
  onBlur?: (event: React.ChangeEvent<HTMLInputElement>) => void;
}) {
  return (
    <FormControl component={Stack} gap={1} fullWidth>
      <Stack direction="row" justifyContent="space-between" alignItems="end" gap={1}>
        <Stack direction="row" alignItems="center" gap={0.5}>
          <Typography variant="subtitle2">{label}</Typography>
          {required && (
            <Typography variant="h5" component="span" color="error">
              *
            </Typography>
          )}
        </Stack>
        {aiLabel && <AILabel label={aiLabel} />}
      </Stack>
      <MuiTextField
        disabled={disabled}
        error={error}
        name={name}
        value={value}
        placeholder={placeholder}
        multiline={multiline}
        rows={rows}
        sx={{
          lineHeight: multiline ? 1.65 : undefined,

          "& .MuiOutlinedInput-root": {
            bgcolor: "background.paper",
          },
        }}
        slotProps={{
          input: {
            startAdornment: startAdornment,
          },
        }}
        onChange={onChange}
      />
      {helperText && (
        <FormHelperText error={error} sx={{ m: 0, mt: -0.5 }}>
          {helperText}
        </FormHelperText>
      )}
    </FormControl>
  );
}

function AILabel({ label }: { label: string }) {
  return (
    <Chip
      size="small"
      label={
        <Stack direction="row" alignItems="center" gap={1}>
          <Sparkle size={pxToRem(12)} />
          {label}
        </Stack>
      }
      sx={(theme) => ({
        bgcolor: alpha(theme.palette.success.light, 0.1),
        color: theme.palette.success.light,
        alignSelf: "end",
      })}
    />
  );
}
