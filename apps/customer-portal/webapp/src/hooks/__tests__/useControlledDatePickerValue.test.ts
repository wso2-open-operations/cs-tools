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

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  useControlledDatePickerValue,
  isPastOrPresentDate,
  type UseControlledDatePickerValueOptions,
} from "@hooks/useControlledDatePickerValue";

function parseDateOnly(value: string): Date | null {
  if (!value) return null;
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

// Mimics MUI's own mid-typing report: a month/day entered but the year not
// yet finished is a `Date` whose getTime() is NaN.
const INCOMPLETE_DATE = new Date(NaN);

function setup(overrides: Partial<UseControlledDatePickerValueOptions> = {}) {
  const onChange = vi.fn();
  const { result, rerender } = renderHook(
    (props: { value: string }) =>
      useControlledDatePickerValue({
        value: props.value,
        onChange,
        parse: parseDateOnly,
        format: formatDateOnly,
        ...overrides,
      }),
    { initialProps: { value: overrides.value ?? "" } },
  );
  return { result, rerender, onChange };
}

function advance(ms: number): void {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

describe("useControlledDatePickerValue", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("keeps a half-typed date instead of resetting it", () => {
    const { result, onChange } = setup();

    act(() => {
      result.current.handleChange(INCOMPLETE_DATE);
    });

    expect(result.current.localDate).toBe(INCOMPLETE_DATE);
    advance(1000);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("commits a complete date only after the debounce settles, not immediately", () => {
    const { result, onChange } = setup();
    const complete = new Date(2026, 0, 15);

    act(() => {
      result.current.handleChange(complete);
    });
    expect(onChange).not.toHaveBeenCalled();

    advance(299);
    expect(onChange).not.toHaveBeenCalled();

    advance(1);
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("2026-01-15");
  });

  it("commits an explicit clear immediately, without waiting for the debounce", () => {
    const { result, onChange } = setup({ value: "2026-01-15" });

    act(() => {
      result.current.handleClear();
    });

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("");
    expect(result.current.localDate).toBeNull();
  });

  it("never commits a stale valid date once the user has moved on to an incomplete edit", () => {
    const { result, onChange } = setup();

    act(() => {
      result.current.handleChange(new Date(2026, 0, 15));
    });
    advance(100);

    act(() => {
      result.current.handleChange(INCOMPLETE_DATE);
    });

    advance(1000);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("supersedes a still-pending commit with whatever the user edits to next", () => {
    const { result, onChange } = setup();

    act(() => {
      result.current.handleChange(new Date(2026, 0, 15));
    });
    advance(100);

    act(() => {
      result.current.handleChange(new Date(2026, 1, 20));
    });
    advance(1000);

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("2026-02-20");
  });

  it("ignores a cleared section (null from onChange) -- only handleClear commits a clear", () => {
    const { result, onChange } = setup({ value: "2026-01-15" });

    act(() => {
      result.current.handleChange(null);
    });

    advance(1000);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("stays in sync when the value changes externally", () => {
    const { result, rerender, onChange } = setup();

    act(() => {
      result.current.handleChange(new Date(2026, 0, 15));
    });
    // Still pending -- an external reset arrives before it settles.
    rerender({ value: "2026-02-01" });

    expect(result.current.localDate).toEqual(new Date(2026, 1, 1));
    advance(1000);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does not re-sync against its own echoed commit", () => {
    const { result, rerender, onChange } = setup();

    act(() => {
      result.current.handleChange(new Date(2026, 0, 15));
    });
    advance(300);
    expect(onChange).toHaveBeenCalledWith("2026-01-15");

    // The parent round-trips the committed value back down, as it would in
    // a real controlled component.
    rerender({ value: "2026-01-15" });

    expect(result.current.localDate).toEqual(new Date(2026, 0, 15));
  });

  it("honours a caller-supplied isComplete predicate", () => {
    const isComplete = (date: unknown): date is Date =>
      date instanceof Date && !Number.isNaN(date.getTime()) && date.getFullYear() >= 1000;
    const { result, onChange } = setup({ isComplete });

    // A short, technically-valid year (e.g. the user typed "2" and tabbed
    // away) must not be treated as complete. `new Date(2, 0, 15)` would
    // actually construct 1902 (the Date constructor's own 2-digit-year
    // special case), so the literal year is set afterwards instead.
    const shortYearDate = new Date(2026, 0, 15);
    shortYearDate.setFullYear(2);
    act(() => {
      result.current.handleChange(shortYearDate);
    });
    advance(1000);
    expect(onChange).not.toHaveBeenCalled();

    act(() => {
      result.current.handleChange(new Date(2026, 0, 15));
    });
    advance(300);
    expect(onChange).toHaveBeenCalledWith("2026-01-15");
  });
});

describe("isPastOrPresentDate", () => {
  it("accepts today and any earlier date", () => {
    expect(isPastOrPresentDate(new Date())).toBe(true);
    expect(isPastOrPresentDate(new Date(2020, 0, 1))).toBe(true);
  });

  it("rejects a date later than today", () => {
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    expect(isPastOrPresentDate(tomorrow)).toBe(false);
  });

  it("rejects an incomplete/invalid date the same as the default check", () => {
    expect(isPastOrPresentDate(new Date(NaN))).toBe(false);
    expect(isPastOrPresentDate(null)).toBe(false);
  });
});

describe("useControlledDatePickerValue with isComplete: isPastOrPresentDate", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("never commits a hand-typed future date, even though it's a complete, valid Date", () => {
    const { result, onChange } = setup({ isComplete: isPastOrPresentDate });
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);

    act(() => {
      result.current.handleChange(tomorrow);
    });
    advance(1000);

    // This is the gap a `disableFuture`-flagged MUI DatePicker alone does
    // not close: it marks the typed value invalid in the UI, but still
    // reports it through onChange -- the caller has to reject it itself.
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe("useControlledDatePickerValue with a short-year guard composed with isPastOrPresentDate", () => {
  // Mirrors `DateRangeFilter`'s own `isCompleteCalendarDate`: its short-year
  // guard, plus a future-date rejection now that every real caller of that
  // component filters on a "Created Date"/"Updated Date" that can't be in
  // the future.
  function isCompleteCalendarDate(date: unknown): date is Date {
    return (
      date instanceof Date &&
      !Number.isNaN(date.getTime()) &&
      date.getFullYear() >= 1000 &&
      isPastOrPresentDate(date)
    );
  }

  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("never commits a hand-typed future date, even with a full year typed", () => {
    const { result, onChange } = setup({ isComplete: isCompleteCalendarDate });
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);

    act(() => {
      result.current.handleChange(tomorrow);
    });
    advance(1000);

    expect(onChange).not.toHaveBeenCalled();
  });

  it("still rejects a short, technically-valid year even when it's in the past", () => {
    const { result, onChange } = setup({ isComplete: isCompleteCalendarDate });
    const shortYearDate = new Date(2026, 0, 15);
    shortYearDate.setFullYear(2);

    act(() => {
      result.current.handleChange(shortYearDate);
    });
    advance(1000);

    expect(onChange).not.toHaveBeenCalled();
  });

  it("commits a complete date that is both a full year and not in the future", () => {
    const { result, onChange } = setup({ isComplete: isCompleteCalendarDate });

    act(() => {
      result.current.handleChange(new Date(2020, 0, 15));
    });
    advance(300);

    expect(onChange).toHaveBeenCalledWith("2020-01-15");
  });
});
