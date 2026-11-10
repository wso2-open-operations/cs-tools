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

import { act, renderHook } from "@testing-library/react";
import { useState, type FocusEvent } from "react";
import { describe, expect, it, vi } from "vitest";
import { usePickerFieldValue } from "@hooks/usePickerFieldValue";
import { formatDateTimeLocal, parseDateTimeLocal } from "@utils/dateTimeLocal";

const setup = (initial: string) => {
  const onChange = vi.fn();
  const hook = renderHook(
    ({ value }) =>
      usePickerFieldValue({ value, onChange, parse: parseDateTimeLocal, format: formatDateTimeLocal }),
    { initialProps: { value: initial } },
  );
  return { onChange, ...hook };
};

const blurTo = (relatedTarget: Element | null) =>
  ({ currentTarget: document.createElement("div"), relatedTarget }) as unknown as FocusEvent<HTMLElement>;

describe("usePickerFieldValue", () => {
  it("shows the committed value as a Date", () => {
    const { result } = setup("2026-06-10T04:30");
    expect(result.current.draft?.getHours()).toBe(4);
  });

  it("commits a finished date straight away, formatted", () => {
    const { result, onChange } = setup("");
    act(() => result.current.handleChange(new Date(2026, 5, 10, 9, 5)));
    expect(onChange).toHaveBeenCalledWith("2026-06-10T09:05");
  });

  it("does not commit while the entry is unfinished, and keeps what was typed", () => {
    const { result, onChange } = setup("2026-06-10T04:30");
    const partial = new Date(Number.NaN);
    act(() => result.current.handleChange(partial));
    act(() => result.current.handleChange(null));
    expect(onChange).not.toHaveBeenCalled();
    expect(result.current.draft).toBeNull();
  });

  it("does not commit a date whose year is still short", () => {
    const { result, onChange } = setup("");
    const shortYear = new Date(2026, 5, 10);
    shortYear.setFullYear(2);
    act(() => result.current.handleChange(shortYear));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("commits an empty value when focus leaves an unfinished field", () => {
    const { result, onChange } = setup("2026-06-10T04:30");
    act(() => result.current.handleChange(null));
    act(() => result.current.handleBlur(blurTo(null)));
    expect(onChange).toHaveBeenCalledWith("");
  });

  it("keeps a finished field on blur", () => {
    const { result, onChange } = setup("2026-06-10T04:30");
    act(() => result.current.handleBlur(blurTo(null)));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does nothing on blur when the value was already empty", () => {
    const { result, onChange } = setup("");
    act(() => result.current.handleBlur(blurTo(null)));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("ignores focus moving between sections of the same field", () => {
    const { result, onChange } = setup("2026-06-10T04:30");
    act(() => result.current.handleChange(null));
    const inside = document.createElement("span");
    const container = document.createElement("div");
    container.appendChild(inside);
    act(() =>
      result.current.handleBlur({ currentTarget: container, relatedTarget: inside } as unknown as FocusEvent<HTMLElement>),
    );
    expect(onChange).not.toHaveBeenCalled();
  });

  it("adopts a value that changed outside, dropping the draft", () => {
    const { result, rerender } = setup("2026-06-10T04:30");
    act(() => result.current.handleChange(null));
    rerender({ value: "2026-07-01T10:00" });
    expect(result.current.draft?.getMonth()).toBe(6);
  });

  it("keeps the draft when the parent stores what was just committed", () => {
    const { result } = renderHook(() => {
      const [value, setValue] = useState("");
      return {
        value,
        ...usePickerFieldValue({ value, onChange: setValue, parse: parseDateTimeLocal, format: formatDateTimeLocal }),
      };
    });
    const picked = new Date(2026, 5, 10, 9, 5);
    act(() => result.current.handleChange(picked));
    expect(result.current.value).toBe("2026-06-10T09:05");
    expect(result.current.draft).toBe(picked);
  });

  it("honours a custom isComplete", () => {
    const onChange = vi.fn();
    const { result } = renderHook(() =>
      usePickerFieldValue({
        value: "",
        onChange,
        parse: parseDateTimeLocal,
        format: formatDateTimeLocal,
        isComplete: (d): d is Date => d instanceof Date && d.getFullYear() === 2030,
      }),
    );
    act(() => result.current.handleChange(new Date(2026, 5, 10, 9, 5)));
    expect(onChange).not.toHaveBeenCalled();
    act(() => result.current.handleChange(new Date(2030, 5, 10, 9, 5)));
    expect(onChange).toHaveBeenCalledWith("2030-06-10T09:05");
  });
});
