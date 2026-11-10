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

import { AdapterDateFns, DatePickers } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";
import type { SxProps, Theme } from "@wso2/oxygen-ui";
import { usePickerFieldValue } from "@hooks/usePickerFieldValue";
import {
  formatDateOnly,
  isCompleteDate,
  parseDateOnly,
} from "@utils/dateTimeLocal";

const { LocalizationProvider, DatePicker } = DatePickers;

export type DateLocalFieldProps = {
  id?: string;
  label?: string;
  /** `YYYY-MM-DD` text, or "" for none. */
  value: string;
  /** Called with a finished `YYYY-MM-DD`, or "" once the field is left unfinished. */
  onChange: (value: string) => void;
  /** Greys out days after today and refuses a typed one. */
  disableFuture?: boolean;
  disabled?: boolean;
  required?: boolean;
  error?: boolean;
  helperText?: ReactNode;
  /** Styles for the text field's root. */
  sx?: SxProps<Theme>;
};

/** A finished date that is not after today. */
function isCompletePastOrPresentDate(date: unknown): date is Date {
  return isCompleteDate(date) && date.getTime() <= Date.now();
}

/**
 * Date field built on the Oxygen UI date picker that keeps the value as a
 * `YYYY-MM-DD` string, the shape a native `date` input had.
 *
 * @param {DateLocalFieldProps} props - Field value, change handler and options.
 * @returns {JSX.Element} The rendered picker.
 */
export default function DateLocalField({
  id,
  label,
  value,
  onChange,
  disableFuture,
  disabled,
  required,
  error,
  helperText,
  sx,
}: DateLocalFieldProps): JSX.Element {
  const { draft, handleChange, handleBlur } = usePickerFieldValue({
    value,
    onChange,
    parse: parseDateOnly,
    format: formatDateOnly,
    isComplete: disableFuture ? isCompletePastOrPresentDate : isCompleteDate,
  });

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <DatePicker
        label={label}
        value={draft}
        onChange={handleChange}
        disableFuture={disableFuture}
        disabled={disabled}
        slotProps={{
          textField: {
            id,
            size: "small",
            fullWidth: true,
            required,
            error,
            helperText,
            sx,
            onBlur: handleBlur,
          },
        }}
      />
    </LocalizationProvider>
  );
}
