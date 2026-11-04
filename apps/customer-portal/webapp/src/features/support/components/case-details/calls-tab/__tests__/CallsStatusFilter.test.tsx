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
import CallsStatusFilter from "@case-details-calls/CallsStatusFilter";
import { CALL_REQUEST_STATUS_FILTER_OPTIONS } from "@features/support/constants/supportConstants";

describe("CallsStatusFilter", () => {
  it("offers Pending, Completed and Cancelled, plus a way back to every status", () => {
    render(
      <CallsStatusFilter
        value=""
        options={CALL_REQUEST_STATUS_FILTER_OPTIONS}
        onChange={vi.fn()}
      />,
    );

    fireEvent.mouseDown(screen.getByRole("combobox"));

    expect(
      screen.getAllByRole("option").map((o) => o.textContent),
    ).toEqual(["All statuses", "Pending", "Completed", "Cancelled"]);
  });

  it("reports the chosen status", () => {
    const onChange = vi.fn();
    render(
      <CallsStatusFilter
        value=""
        options={CALL_REQUEST_STATUS_FILTER_OPTIONS}
        onChange={onChange}
      />,
    );

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Cancelled" }));

    expect(onChange).toHaveBeenCalledWith("cancelled");
  });

  it("reports an empty value for all statuses", () => {
    const onChange = vi.fn();
    render(
      <CallsStatusFilter
        value="completed"
        options={CALL_REQUEST_STATUS_FILTER_OPTIONS}
        onChange={onChange}
      />,
    );

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "All statuses" }));

    expect(onChange).toHaveBeenCalledWith("");
  });
});
