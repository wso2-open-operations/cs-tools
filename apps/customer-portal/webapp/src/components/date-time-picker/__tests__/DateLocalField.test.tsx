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
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import DateLocalField from "@components/date-time-picker/DateLocalField";

const input = (): HTMLInputElement =>
  screen.getAllByLabelText(/Applied On/).find((el) => el.tagName === "INPUT") as HTMLInputElement;
const sections = (): HTMLElement[] => screen.getAllByRole("spinbutton");

function Harness({
  initial,
  onChange,
  disableFuture,
}: {
  initial: string;
  onChange: (v: string) => void;
  disableFuture?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <DateLocalField
      label="Applied On"
      value={value}
      disableFuture={disableFuture}
      onChange={(next) => {
        onChange(next);
        setValue(next);
      }}
    />
  );
}

describe("DateLocalField", () => {
  it("shows a YYYY-MM-DD value in the picker's format", () => {
    render(<Harness initial="2026-03-04" onChange={vi.fn()} />);
    expect(input().value).toBe("03/04/2026");
  });

  it("reports a finished entry as YYYY-MM-DD", () => {
    const onChange = vi.fn();
    render(<Harness initial="" onChange={onChange} />);

    fireEvent.change(input(), { target: { value: "07/01/2027" } });

    expect(onChange).toHaveBeenCalledWith("2027-07-01");
  });

  it("reports an empty value once focus leaves an unfinished field", () => {
    const onChange = vi.fn();
    render(<Harness initial="2026-03-04" onChange={onChange} />);

    fireEvent.change(input(), { target: { value: "" } });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.blur(sections()[0]);

    expect(onChange).toHaveBeenCalledWith("");
  });

  it("refuses a typed future day when disableFuture is set", () => {
    const onChange = vi.fn();
    render(<Harness initial="" onChange={onChange} disableFuture />);

    fireEvent.change(input(), { target: { value: "01/01/2999" } });

    expect(onChange).not.toHaveBeenCalled();
  });

  it("accepts today and past days when disableFuture is set", () => {
    const onChange = vi.fn();
    render(<Harness initial="" onChange={onChange} disableFuture />);

    fireEvent.change(input(), { target: { value: "01/02/2020" } });

    expect(onChange).toHaveBeenCalledWith("2020-01-02");
  });

  it("accepts a future day without disableFuture", () => {
    const onChange = vi.fn();
    render(<Harness initial="" onChange={onChange} />);

    fireEvent.change(input(), { target: { value: "01/01/2999" } });

    expect(onChange).toHaveBeenCalledWith("2999-01-01");
  });
});
