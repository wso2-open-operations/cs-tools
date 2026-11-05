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
import { useImperativeHandle, useRef, type JSX, type ReactNode, type Ref } from "react";
import { usePickerFieldValue } from "@hooks/usePickerFieldValue";
import { formatDateTimeLocal, parseDateTimeLocal } from "@utils/dateTimeLocal";

const { LocalizationProvider, DateTimePicker } = DatePickers;

export type DateTimeLocalFieldProps = {
  id?: string;
  label?: string;
  /** Accessible name for a field whose visible label is rendered elsewhere. */
  ariaLabel?: string;
  /** `YYYY-MM-DDTHH:mm` wall-clock text, or "" for none. */
  value: string;
  /** Called with a finished `YYYY-MM-DDTHH:mm`, or "" once the field is left unfinished. */
  onChange: (value: string) => void;
  /** Earliest selectable `YYYY-MM-DDTHH:mm`. */
  minDateTime?: string;
  /** Minute step of the time clock. */
  minutesStep?: number;
  required?: boolean;
  disabled?: boolean;
  /** Shown but not editable and without the popup, unlike `disabled`. */
  readOnly?: boolean;
  error?: boolean;
  helperText?: ReactNode;
  /** Receives a handle whose `focus()` moves focus to the field's first section. */
  focusRef?: Ref<DateTimeLocalFieldHandle>;
};

export type DateTimeLocalFieldHandle = { focus: () => void };

/**
 * Date-time field built on the Oxygen UI date-time picker that keeps the value
 * as a `YYYY-MM-DDTHH:mm` string, the shape a native `datetime-local` input
 * had, so the zone conversion done by each form stays where it was. The text
 * is wall-clock time: it is shown and parsed as typed, never shifted.
 *
 * @param {DateTimeLocalFieldProps} props - Field value, change handler and options.
 * @returns {JSX.Element} The rendered picker.
 */
export default function DateTimeLocalField({
  id,
  label,
  ariaLabel,
  value,
  onChange,
  minDateTime,
  minutesStep = 5,
  required,
  disabled,
  readOnly,
  error,
  helperText,
  focusRef,
}: DateTimeLocalFieldProps): JSX.Element {
  const { draft, handleChange, handleBlur } = usePickerFieldValue({
    value,
    onChange,
    parse: parseDateTimeLocal,
    format: formatDateTimeLocal,
  });

  // The picker's own focus handle differs between MUI X versions, and Oxygen UI
  // bundles its own, so focus the first editable section of the field directly.
  const rootRef = useRef<HTMLDivElement>(null);
  useImperativeHandle(
    focusRef,
    () => ({
      focus: () => rootRef.current?.querySelector<HTMLElement>('[role="spinbutton"]')?.focus(),
    }),
    [],
  );

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <DateTimePicker
        ref={rootRef}
        label={label}
        value={draft}
        onChange={handleChange}
        minDateTime={parseDateTimeLocal(minDateTime) ?? undefined}
        timeSteps={{ minutes: minutesStep }}
        disabled={disabled}
        readOnly={readOnly}
        slotProps={{
          textField: {
            id,
            size: "small",
            fullWidth: true,
            required,
            error,
            helperText,
            "aria-label": ariaLabel,
            onBlur: handleBlur,
          },
        }}
      />
    </LocalizationProvider>
  );
}
