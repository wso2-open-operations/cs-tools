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
import { ScheduleCallDialog } from "@features/csm-cases/components/ScheduleCallDialog";
import type { BeCallRequestView } from "@api/backend/types";

function makeCallRequest(preferredTimes: string[]): BeCallRequestView {
  return {
    id: "11111111-1111-1111-1111-111111111111",
    preferredTimes,
    durationMin: 30,
  };
}

// The dialog selects the first preferred time only when the target *changes*
// (null -> record), matching how the widget opens it. Mimic that with rerender.
function renderWithTarget(cr: BeCallRequestView) {
  const onSubmit = vi.fn();
  const onClose = vi.fn();
  const shared = {
    isReschedule: false,
    submitting: false,
    error: null,
    onClose,
    onSubmit,
  };
  const { rerender } = render(
    <ScheduleCallDialog callRequest={null} {...shared} />,
  );
  rerender(<ScheduleCallDialog callRequest={cr} {...shared} />);
  return { onSubmit, onClose };
}

const scheduleButton = (): HTMLElement =>
  screen.getByRole("button", { name: /^schedule$/i });

describe("ScheduleCallDialog", () => {
  it("submits a selected preferred time as an RFC3339 UTC timestamp", () => {
    // Preferred times arrive in the backend wall-clock format (e.g. ServiceNow's
    // MM/DD/YYYY), but the API requires RFC3339 UTC. This is the regression:
    // sending the raw preferred-time string produced a 400. Far-future slot so
    // the past-guard passes.
    const { onSubmit } = renderWithTarget(
      makeCallRequest(["12/31/2027 08:45:00"]),
    );

    fireEvent.click(scheduleButton());

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        meetingDate: "2027-12-31T08:45:00Z",
        durationInMinutes: 30,
      }),
    );
  });

  it("also normalizes the ISO-8601-with-space backend format", () => {
    const { onSubmit } = renderWithTarget(
      makeCallRequest(["2027-12-31 08:45:00"]),
    );
    fireEvent.click(scheduleButton());
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ meetingDate: "2027-12-31T08:45:00Z" }),
    );
  });

  it("blocks a preferred time in the past and does not submit", () => {
    const { onSubmit } = renderWithTarget(
      makeCallRequest(["01/01/2020 08:45:00"]),
    );
    fireEvent.click(scheduleButton());
    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByText(/must be in the future/i)).toBeInTheDocument();
  });
});
