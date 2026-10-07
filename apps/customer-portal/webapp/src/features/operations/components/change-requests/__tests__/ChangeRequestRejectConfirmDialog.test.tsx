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
import ChangeRequestRejectConfirmDialog from "@features/operations/components/change-requests/ChangeRequestRejectConfirmDialog";
import { ChangeRequestDecisionMode } from "@features/operations/types/changeRequests";

function renderDialog(
  props: Partial<React.ComponentProps<typeof ChangeRequestRejectConfirmDialog>> = {},
) {
  const onClose = vi.fn();
  const onConfirm = vi.fn();
  render(
    <ChangeRequestRejectConfirmDialog
      open
      mode={ChangeRequestDecisionMode.CUSTOMER_APPROVAL}
      proposeNewTime="available"
      isPending={false}
      onClose={onClose}
      onConfirm={onConfirm}
      {...props}
    />,
  );
  return { onClose, onConfirm };
}

describe("ChangeRequestRejectConfirmDialog", () => {
  it("says rejecting cancels the change request, and points to Propose New Time while it is on", () => {
    renderDialog();
    expect(screen.getByRole("dialog", { name: "Reject this change request?" })).toBeInTheDocument();
    expect(screen.getByText("Rejecting cancels this change request.")).toBeInTheDocument();
    expect(screen.getByText(/use Propose New Time instead/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject change request" })).toBeInTheDocument();
  });

  it("does not point at Propose New Time while WSO2 has the change on hold: it says why instead", () => {
    renderDialog({ proposeNewTime: "on_hold" });
    expect(screen.getByText("Rejecting cancels this change request.")).toBeInTheDocument();
    expect(screen.queryByText(/use Propose New Time instead/)).not.toBeInTheDocument();
    expect(
      screen.getByText("A new time cannot be proposed right now because WSO2 has this change request on hold."),
    ).toBeInTheDocument();
    // The answer itself is not held back: it can still be confirmed.
    expect(screen.getByRole("button", { name: "Reject change request" })).toBeEnabled();
  });

  it("says nothing about a different time where Propose New Time is not offered", () => {
    renderDialog({ proposeNewTime: "unavailable" });
    expect(screen.getByText("Rejecting cancels this change request.")).toBeInTheDocument();
    expect(screen.queryByText(/Propose New Time/)).not.toBeInTheDocument();
    expect(screen.queryByText(/on hold/)).not.toBeInTheDocument();
  });

  it("describes the dialog by its message and its hint, whichever hint it has", () => {
    renderDialog({ proposeNewTime: "on_hold" });
    const description = document.getElementById("cr-reject-confirm-description");
    expect(description).toHaveTextContent("Rejecting cancels this change request.");
    expect(description).toHaveTextContent("WSO2 has this change request on hold");
    expect(screen.getByRole("dialog")).toHaveAttribute("aria-describedby", "cr-reject-confirm-description");
  });

  it("says marking a review unsuccessful sends the change into rollback", () => {
    renderDialog({ mode: ChangeRequestDecisionMode.CUSTOMER_REVIEW });
    expect(
      screen.getByRole("dialog", { name: "Mark this change as unsuccessful?" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Marking it unsuccessful sends the change into rollback."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Propose New Time/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mark unsuccessful" })).toBeInTheDocument();
  });

  it("confirms with the confirm button and closes with Go back or Escape", () => {
    const { onClose, onConfirm } = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Reject change request" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Go back" }));
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(2);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("cannot be confirmed twice or dismissed while the request is pending", () => {
    const { onClose, onConfirm } = renderDialog({ isPending: true });
    const confirm = screen.getByRole("button", { name: "Submitting..." });
    expect(confirm).toBeDisabled();
    expect(screen.getByRole("button", { name: "Go back" })).toBeDisabled();
    fireEvent.click(confirm);
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onConfirm).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders nothing while closed", () => {
    renderDialog({ open: false });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
