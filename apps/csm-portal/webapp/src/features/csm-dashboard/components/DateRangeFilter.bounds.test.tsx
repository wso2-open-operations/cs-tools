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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// Replace the sectioned pickers with buttons that expose the bounds the
// component passes and let a test fire an arbitrary onChange value, since a
// typed out-of-range date bypasses the picker's own maxDate.
vi.mock("@wso2/oxygen-ui", async (importActual) => {
  const actual = await importActual<typeof import("@wso2/oxygen-ui")>();
  return {
    ...actual,
    DatePickers: {
      ...actual.DatePickers,
      DatePicker: (props: {
        label: string;
        maxDate?: Date;
        onChange: (d: Date | null) => void;
      }) => (
        <>
          <button
            type="button"
            data-testid={`${props.label}-future`}
            onClick={() => props.onChange(new Date(2999, 0, 1))}
          />
          <button
            type="button"
            data-testid={`${props.label}-past`}
            onClick={() => props.onChange(new Date(2020, 0, 5))}
          />
          <span data-testid={`${props.label}-max`}>
            {props.maxDate ? props.maxDate.toDateString() : "none"}
          </span>
        </>
      ),
    },
  };
});

import DateRangeFilter from "@features/csm-dashboard/components/DateRangeFilter";

describe("DateRangeFilter — caps at today", () => {
  it("rejects a typed future date on either field", () => {
    const onChange = vi.fn();
    render(<DateRangeFilter value={{}} onChange={onChange} />);
    fireEvent.click(screen.getByTestId("From-future"));
    fireEvent.click(screen.getByTestId("To-future"));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("accepts a past date and passes today as both fields' maxDate", () => {
    const onChange = vi.fn();
    render(<DateRangeFilter value={{}} onChange={onChange} />);
    fireEvent.click(screen.getByTestId("From-past"));
    expect(onChange).toHaveBeenCalledWith({ from: "2020-01-05" });
    expect(screen.getByTestId("To-max")).toHaveTextContent(new Date().toDateString());
    expect(screen.getByTestId("From-max")).toHaveTextContent(new Date().toDateString());
  });

  it("caps From's maxDate at an earlier To value", () => {
    render(<DateRangeFilter value={{ to: "2020-03-01" }} onChange={vi.fn()} />);
    expect(screen.getByTestId("From-max")).toHaveTextContent(new Date(2020, 2, 1).toDateString());
  });
});
