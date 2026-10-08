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

import { useEffect, useRef, useState } from "react";

const DEFAULT_COMMIT_DEBOUNCE_MS = 300;

/** The default "is this a complete, valid date" check. A caller with its own
 * stricter notion (e.g. rejecting a technically-valid but still-incomplete
 * short year) can supply `isComplete` instead. */
function isValidDate(date: unknown): date is Date {
  return date instanceof Date && !Number.isNaN(date.getTime());
}

/**
 * A complete, valid date that is also not later than today -- the `isComplete`
 * override a `disableFuture`-flagged `DatePicker` needs. MUI's own
 * `disableFuture` prop only disables the calendar popup's future days and
 * marks a typed future date as visually invalid (`textField.error`); it does
 * not stop `onChange` from firing with that date, so without this, a caller
 * still receives and commits a hand-typed future date regardless of
 * `disableFuture` being set. Found live, immediately after the reset-while-
 * typing fix shipped: that bug tended to wipe an in-progress edit before the
 * user finished typing a complete date at all, which incidentally made a
 * complete future date rare to ever reach `onChange` in practice. Fixing the
 * reset made typing a complete date reliable, which is exactly what
 * surfaced this separate, pre-existing gap. Compares at the instant of the
 * call, not a memoized "today": a date-only value (local midnight) is
 * always `<=` "right now" on the same calendar day, so this correctly
 * allows "today" throughout the day and only ever rejects a day strictly
 * after it.
 */
export function isPastOrPresentDate(date: unknown): date is Date {
  return isValidDate(date) && date.getTime() <= Date.now();
}

export interface UseControlledDatePickerValueOptions {
  /** The committed, external value (e.g. "2026-01-15", or "" for none). */
  value: string;
  /** Called with the new committed value, or "" to clear. */
  onChange: (next: string) => void;
  /** Parses the committed string into a `Date` for the picker to display,
   * or `null` if `value` is empty/unparseable. */
  parse: (value: string) => Date | null;
  /** Formats a complete date back into the committed string shape. */
  format: (date: Date) => string;
  /** Overrides the default "complete, valid `Date`" check -- e.g. to also
   * reject a technically-valid but still-incomplete short year (typing "2"
   * is a legal `Date`, just not one anybody meant to commit). */
  isComplete?: (date: unknown) => date is Date;
  /** How long to wait after the last edit before committing a complete date
   * upstream. Default 300ms, matching `useDebouncedValue` elsewhere in this
   * app. */
  debounceMs?: number;
}

export interface UseControlledDatePickerValueResult {
  /** Wire directly to the `DatePicker`'s own `value`. */
  localDate: Date | null;
  /** Wire directly to the `DatePicker`'s own `onChange`. */
  handleChange: (date: unknown) => void;
  /** Wire to `slotProps.field.onClear` -- the one unambiguous "the user
   * clicked the clear button" signal. Never wire this to `onChange`'s own
   * `null` case; see this hook's own doc comment for why. */
  handleClear: () => void;
}

/**
 * Backs a single MUI `DatePicker` field whose value is a controlled,
 * externally-owned string (`value`/`onChange`), fixing a real, reported bug
 * in that shape: typing a date one section at a time (month, then day, then
 * year) used to lose the month/day the moment the year was started.
 *
 * MUI reports an incomplete date as an invalid (`NaN`) `Date` on every
 * keystroke while a masked `MM/DD/YYYY` field is still partially typed, and
 * the naive fix -- reading `value` straight off `parse(value)` and treating
 * "not a complete date" as "clear it" -- commits `""` upstream on every one
 * of those keystrokes, which then round-trips back down through `value` and
 * resets the field to empty. Keeping the displayed date as local state
 * (`localDate`), updated on every keystroke regardless of completeness, and
 * only ever pushing a value upstream for a complete, valid `Date` fixes this
 * without losing anything: MUI never fights a controlled value that hasn't
 * itself changed, so the field keeps whatever's been typed so far during
 * every intermediate, incomplete render.
 *
 * `null` from the picker's own `onChange` is deliberately never treated as
 * "clear it" here -- MUI reports `null` both for the field's clear button
 * AND for removing a single section while editing (e.g. backspacing just
 * the day). Committing a clear for the second case would wipe the filter in
 * the middle of an edit the user never meant to abandon. `handleClear` (wire
 * it to the field's own dedicated `slotProps.field.onClear`) is the one
 * place an actual, deliberate clear is committed.
 *
 * A complete, valid date is debounced before being committed upstream, the
 * same technique `useDebouncedValue` applies elsewhere in this app, but
 * implemented with a cancellable `setTimeout` here rather than that hook
 * directly: a rapid run of valid intermediate dates (e.g. holding the
 * calendar's day/month stepper, or quickly clicking several days in the
 * popup) would otherwise commit a value -- and likely re-run a search --
 * once per intermediate value instead of once the user actually settles.
 * Every picker change cancels whatever commit was previously scheduled,
 * before deciding whether to schedule a new one: an in-progress edit (an
 * incomplete date, or a section the user just removed) must never let an
 * earlier, now-superseded valid date commit out from under it a moment
 * later. An explicit clear is committed immediately, never debounced: a
 * deliberate, discrete action reads as unresponsive if delayed the same way
 * a mid-typing keystroke is.
 *
 * Also stays in sync with an externally-driven change to `value` -- e.g. a
 * "clear filters" action resetting this exact field's value while the
 * component stays mounted. `lastCommittedValueRef` tracks what this hook
 * itself last told the caller; when `value` changes to something else, that
 * can only have come from outside, so the displayed date is re-synced from
 * it and any of this hook's own still-pending commit -- which would
 * otherwise silently overwrite the external change a moment later -- is
 * dropped. The debounced commit itself always calls the latest `onChange`
 * (via `onChangeRef`), not the one captured when the timer was scheduled,
 * in case the caller's own `onChange` identity changes while a commit is
 * still pending.
 */
export function useControlledDatePickerValue({
  value,
  onChange,
  parse,
  format,
  isComplete = isValidDate,
  debounceMs = DEFAULT_COMMIT_DEBOUNCE_MS,
}: UseControlledDatePickerValueOptions): UseControlledDatePickerValueResult {
  const [localDate, setLocalDate] = useState<Date | null>(() => parse(value));
  const commitTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const lastCommittedValueRef = useRef(value);

  const cancelPendingCommit = (): void => {
    if (commitTimeoutRef.current) {
      clearTimeout(commitTimeoutRef.current);
      commitTimeoutRef.current = null;
    }
  };
  // Cancel a still-pending debounced commit if this field unmounts while
  // the timer is in flight.
  useEffect(() => cancelPendingCommit, []);

  // Adopt an externally-driven value change (not our own echo) and drop
  // anything of our own still pending, which would otherwise overwrite it
  // moments later.
  useEffect(() => {
    if (value !== lastCommittedValueRef.current) {
      lastCommittedValueRef.current = value;
      cancelPendingCommit();
      setLocalDate(parse(value));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const commitNow = (next: string): void => {
    cancelPendingCommit();
    lastCommittedValueRef.current = next;
    onChangeRef.current(next);
  };

  const commitDebounced = (next: string): void => {
    cancelPendingCommit();
    commitTimeoutRef.current = setTimeout(() => {
      commitTimeoutRef.current = null;
      lastCommittedValueRef.current = next;
      onChangeRef.current(next);
    }, debounceMs);
  };

  const handleChange = (date: unknown): void => {
    setLocalDate(date as Date | null);
    // Cancel whatever was previously scheduled on *every* change, before
    // deciding whether to schedule a new one -- see this hook's own doc
    // comment.
    cancelPendingCommit();
    if (isComplete(date)) {
      commitDebounced(format(date));
    }
    // `null` and an incomplete date are both left alone otherwise.
  };

  const handleClear = (): void => {
    setLocalDate(null);
    commitNow("");
  };

  return { localDate, handleChange, handleClear };
}
