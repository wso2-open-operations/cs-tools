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

import { useState, type FocusEvent } from "react";
import { isCompleteDate } from "@utils/dateTimeLocal";

export interface UsePickerFieldValueOptions {
  /** The committed, external value (e.g. "2026-06-10T04:30"), or "" for none. */
  value: string;
  /** Called with the new committed value, or "" when the field is left unfinished. */
  onChange: (next: string) => void;
  /** Parses the committed string into the Date the picker displays. */
  parse: (value: string) => Date | null;
  /** Formats a finished Date back into the committed string. */
  format: (date: Date) => string;
  /** Overrides what counts as a finished date, e.g. to also reject a future day. */
  isComplete?: (date: unknown) => date is Date;
}

export interface UsePickerFieldValueResult {
  /** Wire to the picker's `value`. */
  draft: Date | null;
  /** Wire to the picker's `onChange`. */
  handleChange: (date: unknown) => void;
  /** Wire to the field's `onBlur`. */
  handleBlur: (event: FocusEvent<HTMLElement>) => void;
}

/**
 * Backs a form's date or date-time picker whose value is an externally owned
 * string, the shape a native `datetime-local` / `date` input had.
 *
 * A masked picker reports an unfinished entry (some sections missing) on every
 * keystroke, so the displayed date is kept as local `draft` state and only a
 * finished, valid date is pushed up. Reading the picker's value straight from
 * the string and treating "unfinished" as "empty" would reset the field in the
 * middle of typing.
 *
 * Unlike a filter, a form field must not keep a stale value once the user has
 * emptied it: with a native input, blanking it gave "". When focus leaves the
 * field with the entry unfinished, "" is committed, so a required-field check
 * sees what the user sees. Focus moving between the sections of the same field
 * is not leaving it.
 *
 * Stays in sync with an external change to `value` (a form reset, a value the
 * parent clamped, a derived field) by adopting it and dropping the draft.
 */
export function usePickerFieldValue({
  value,
  onChange,
  parse,
  format,
  isComplete = isCompleteDate,
}: UsePickerFieldValueOptions): UsePickerFieldValueResult {
  const [draft, setDraft] = useState<Date | null>(() => parse(value));
  // The last string this field either received or committed. A `value` that
  // differs from it can only have come from outside.
  const [syncedValue, setSyncedValue] = useState(value);

  if (value !== syncedValue) {
    setSyncedValue(value);
    setDraft(parse(value));
  }

  const commit = (next: string): void => {
    setSyncedValue(next);
    if (next !== value) onChange(next);
  };

  const handleChange = (date: unknown): void => {
    setDraft(date as Date | null);
    if (isComplete(date)) commit(format(date));
    // An unfinished entry (null or an invalid Date) is left to the blur check.
  };

  const handleBlur = (event: FocusEvent<HTMLElement>): void => {
    if (event.currentTarget.contains(event.relatedTarget as Node | null)) return;
    if (!isComplete(draft)) commit("");
  };

  return { draft, handleChange, handleBlur };
}
