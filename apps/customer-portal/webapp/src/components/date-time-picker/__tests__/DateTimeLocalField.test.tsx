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

import { fireEvent, render, screen } from "@testing-library/react";
import { createRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import DateTimeLocalField, { type DateTimeLocalFieldHandle } from "@components/date-time-picker/DateTimeLocalField";

const input = (): HTMLInputElement =>
  screen.getAllByLabelText(/Start/).find((el) => el.tagName === "INPUT") as HTMLInputElement;

function Harness({ initial, onChange }: { initial: string; onChange: (v: string) => void }) {
  const [value, setValue] = useState(initial);
  return (
    <DateTimeLocalField
      label="Start"
      value={value}
      onChange={(next) => {
        onChange(next);
        setValue(next);
      }}
    />
  );
}

const sections = (): HTMLElement[] => screen.getAllByRole("spinbutton");

describe("DateTimeLocalField", () => {
  it("shows a YYYY-MM-DDTHH:mm value in the picker's format", () => {
    render(<Harness initial="2026-06-10T04:30" onChange={vi.fn()} />);
    expect(input().value).toBe("06/10/2026 04:30 AM");
  });

  it("shows an empty field for an empty value", () => {
    render(<Harness initial="" onChange={vi.fn()} />);
    expect(input().value).not.toMatch(/2026/);
  });

  it("reports a finished entry as YYYY-MM-DDTHH:mm wall-clock text", () => {
    const onChange = vi.fn();
    render(<Harness initial="" onChange={onChange} />);

    fireEvent.change(input(), { target: { value: "07/01/2027 09:15 PM" } });

    expect(onChange).toHaveBeenCalledWith("2027-07-01T21:15");
    expect(input().value).toBe("07/01/2027 09:15 PM");
  });

  it("does not report an unfinished entry until focus leaves the field", () => {
    const onChange = vi.fn();
    render(<Harness initial="2026-06-10T04:30" onChange={onChange} />);

    fireEvent.change(input(), { target: { value: "" } });
    expect(onChange).not.toHaveBeenCalled();

    fireEvent.blur(sections()[0]);
    expect(onChange).toHaveBeenCalledWith("");
  });

  it("does not treat moving between sections as leaving the field", () => {
    const onChange = vi.fn();
    render(<Harness initial="2026-06-10T04:30" onChange={onChange} />);
    fireEvent.change(input(), { target: { value: "" } });

    fireEvent.blur(sections()[0], { relatedTarget: sections()[1] });

    expect(onChange).not.toHaveBeenCalled();
  });

  it("keeps a finished value when focus leaves", () => {
    const onChange = vi.fn();
    render(<Harness initial="2026-06-10T04:30" onChange={onChange} />);

    fireEvent.blur(sections()[0]);

    expect(onChange).not.toHaveBeenCalled();
  });

  it("adopts a value the parent changes", () => {
    const { rerender } = render(
      <DateTimeLocalField label="Start" value="2026-06-10T04:30" onChange={vi.fn()} />,
    );
    rerender(<DateTimeLocalField label="Start" value="2026-12-25T18:00" onChange={vi.fn()} />);
    expect(input().value).toBe("12/25/2026 06:00 PM");
  });

  it("shows the helper text and the error state", () => {
    render(
      <DateTimeLocalField label="Start" value="" onChange={vi.fn()} error helperText="Start is required." />,
    );
    expect(screen.getByText("Start is required.")).toBeInTheDocument();
    expect(screen.getByRole("group")).toHaveAttribute("aria-invalid", "true");
  });

  it("moves focus to the first section through the focus handle", () => {
    const focusRef = createRef<DateTimeLocalFieldHandle>();
    render(<DateTimeLocalField label="Start" value="2026-06-10T04:30" onChange={vi.fn()} focusRef={focusRef} />);

    focusRef.current?.focus();

    expect(sections()[0]).toHaveFocus();
  });

  it("takes its accessible name from ariaLabel when there is no visible label", () => {
    render(<DateTimeLocalField ariaLabel="Preferred slot" value="2026-06-10T04:30" onChange={vi.fn()} />);
    expect(screen.getByRole("group", { name: "Preferred slot" })).toBeInTheDocument();
  });

  it("is not editable when disabled", () => {
    render(<DateTimeLocalField label="Start" value="2026-06-10T04:30" onChange={vi.fn()} disabled />);
    expect(input()).toBeDisabled();
  });
});
