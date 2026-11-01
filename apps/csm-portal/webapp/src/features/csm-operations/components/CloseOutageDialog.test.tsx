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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";
import CloseOutageDialog from "@features/csm-operations/components/CloseOutageDialog";

// The user's time zone is pinned, and so is "now", so the wall-clock the picker
// shows and the UTC value sent to the backend are both deterministic.
// Asia/Colombo is UTC+05:30 with no daylight saving.
describe("CloseOutageDialog — UTC end time", () => {
  beforeEach(() => {
    setUserPreferredTimeZone("Asia/Colombo");
    vi.useFakeTimers({ toFake: ["Date"] });
  });

  afterEach(() => {
    vi.useRealTimers();
    clearUserPreferredTimeZone();
  });

  it("confirms with the default 'now' converted from the user's time zone to UTC", () => {
    vi.setSystemTime(new Date("2026-03-01T12:00:00Z")); // 17:30 in Colombo
    const onConfirm = vi.fn();
    render(
      <CloseOutageDialog begin="2026-03-01 10:00:00" isSaving={false} onClose={vi.fn()} onConfirm={onConfirm} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "End outage" }));

    expect(onConfirm).toHaveBeenCalledWith("2026-03-01 12:00:00");
  });

  it("compares real instants: a 'now' before the UTC begin blocks the close", () => {
    // 09:00Z is before the 10:00 UTC begin, although the Colombo wall-clock
    // (14:30) is numerically later than the raw begin digits.
    vi.setSystemTime(new Date("2026-03-01T09:00:00Z"));
    render(
      <CloseOutageDialog begin="2026-03-01 10:00:00" isSaving={false} onClose={vi.fn()} onConfirm={vi.fn()} />,
    );

    expect(screen.getByText(/must not be before the outage's begin time/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "End outage" })).toBeDisabled();
  });
});

describe("CloseOutageDialog required End", () => {
  it("marks End required: MUI's asterisk on its label and `required` on the picker's input", () => {
    render(<CloseOutageDialog begin="2026-03-01 10:00:00" isSaving={false} onClose={vi.fn()} onConfirm={vi.fn()} />);

    // The picker's field is a role=group of spinbuttons labelled by its label.
    const field = screen.getByRole("group", { name: "End" });
    const label = document.getElementById(field.getAttribute("aria-labelledby")!) as HTMLLabelElement;
    expect(label.querySelector(".MuiFormLabel-asterisk")).toHaveAttribute("aria-hidden", "true");
    expect(document.getElementById(label.htmlFor)).toBeRequired();
  });
});
