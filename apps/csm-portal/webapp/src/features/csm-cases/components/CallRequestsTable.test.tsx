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
import { CallRequestsTable } from "@features/csm-cases/components/CallRequestsTable";
import type { BeCallRequestView } from "@api/backend/types";

function callRequest(overrides: Partial<BeCallRequestView> = {}): BeCallRequestView {
  return {
    id: "cr-1",
    number: "CSTASK0000001",
    reason: "Discuss upgrade",
    state: { id: "pending_on_wso2", label: "Pending on WSO2" },
    createdOn: "2026-08-01 10:00:00",
    updatedOn: "2026-08-01 10:00:00",
    ...overrides,
  };
}

describe("CallRequestsTable — closed-case gating", () => {
  it("keeps Reject enabled on a `pending_on_wso2` request even when the case is closed", () => {
    render(
      <CallRequestsTable
        requests={[callRequest()]}
        onAction={vi.fn()}
        scheduleBlockReason="Calls can only be requested while the case is work in progress, awaiting info, waiting on WSO2, solution proposed, or reopened."
      />,
    );

    expect(screen.getByRole("button", { name: "Reject" })).toBeEnabled();
  });

  it("disables Schedule when scheduleBlockReason is set", () => {
    render(
      <CallRequestsTable
        requests={[callRequest()]}
        onAction={vi.fn()}
        scheduleBlockReason="Calls can only be requested while the case is work in progress."
      />,
    );

    expect(screen.getByRole("button", { name: "Schedule" })).toBeDisabled();
  });

  it("keeps Cancel enabled on a `scheduled` request when scheduleBlockReason is set", () => {
    render(
      <CallRequestsTable
        requests={[callRequest({ state: { id: "scheduled", label: "Scheduled" } })]}
        onAction={vi.fn()}
        scheduleBlockReason="blocked"
      />,
    );

    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Reschedule" })).toBeDisabled();
  });

  it("keeps Send call notes enabled on a `notes_pending` request when scheduleBlockReason is set", () => {
    render(
      <CallRequestsTable
        requests={[callRequest({ state: { id: "notes_pending", label: "Notes Pending" } })]}
        onAction={vi.fn()}
        scheduleBlockReason="blocked"
      />,
    );

    expect(screen.getByRole("button", { name: "Send call notes" })).toBeEnabled();
  });

  it("enables every action (including Schedule) when there is no block reason", () => {
    render(
      <CallRequestsTable requests={[callRequest()]} onAction={vi.fn()} />,
    );

    expect(screen.getByRole("button", { name: "Schedule" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Reject" })).toBeEnabled();
  });

  it("calls onAction with the clicked action and the row's call request", () => {
    const onAction = vi.fn();
    const cr = callRequest();
    render(<CallRequestsTable requests={[cr]} onAction={onAction} />);

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    expect(onAction).toHaveBeenCalledWith("reject", cr);
  });
});
