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

import type { ComponentProps } from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestTransitionReasonDialog from "@features/csm-operations/components/ChangeRequestTransitionReasonDialog";

function renderDialog(
  props: Partial<
    ComponentProps<typeof ChangeRequestTransitionReasonDialog>
  > = {},
): {
  onConfirm: ReturnType<typeof vi.fn>;
  onClose: ReturnType<typeof vi.fn>;
} {
  const onConfirm = vi.fn();
  const onClose = vi.fn();
  render(
    <ChangeRequestTransitionReasonDialog
      target="canceled"
      isSubmitting={false}
      onClose={onClose}
      onConfirm={onConfirm}
      {...props}
    />,
  );
  return { onConfirm, onClose };
}

const reasonField = (): HTMLElement => screen.getByLabelText(/reason/i);

describe("ChangeRequestTransitionReasonDialog — reason is required", () => {
  it("disables the confirm action while the reason is empty", () => {
    renderDialog();
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeDisabled();
  });

  it("keeps the confirm action disabled for a whitespace-only reason", () => {
    renderDialog();
    fireEvent.change(reasonField(), { target: { value: "   \n  " } });
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeDisabled();
  });

  it("enables the confirm action once the reason has content", () => {
    renderDialog();
    fireEvent.change(reasonField(), { target: { value: "Superseded by CHG0009999." } });
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeEnabled();
  });

  it("passes the trimmed reason to onConfirm", () => {
    const { onConfirm } = renderDialog();
    fireEvent.change(reasonField(), {
      target: { value: "  Superseded by another change.  " },
    });
    fireEvent.click(screen.getByRole("button", { name: /cancel change/i }));
    expect(onConfirm).toHaveBeenCalledWith("Superseded by another change.");
  });
});

describe("ChangeRequestTransitionReasonDialog — per-target copy", () => {
  it("uses rollback wording and confirm label for the rollback target", () => {
    renderDialog({ target: "rollback" });
    expect(screen.getByRole("heading", { name: /roll back this change/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^roll back$/i })).toBeInTheDocument();
  });

  it("uses cancellation wording and confirm label for the canceled target", () => {
    renderDialog({ target: "canceled" });
    expect(
      screen.getByRole("heading", { name: /cancel this change request/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("requires the reason for a rollback, like a cancellation: confirm stays disabled until it has content", () => {
    const { onConfirm } = renderDialog({ target: "rollback" });
    const confirm = screen.getByRole("button", { name: /^roll back$/i });
    expect(reasonField()).toBeRequired();
    expect(confirm).toBeDisabled();
    fireEvent.change(reasonField(), { target: { value: "   " } });
    expect(confirm).toBeDisabled();
    fireEvent.change(reasonField(), { target: { value: "  The customer's review failed on 6 Oct.  " } });
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);
    expect(onConfirm).toHaveBeenCalledWith("The customer's review failed on 6 Oct.");
  });

  it("confirms a rollback and a cancellation in the error colour", () => {
    renderDialog({ target: "rollback" });
    expect(screen.getByRole("button", { name: /^roll back$/i }).className).toContain("MuiButton-colorError");
    cleanup();
    renderDialog({ target: "canceled" });
    expect(screen.getByRole("button", { name: /cancel change/i }).className).toContain("MuiButton-colorError");
  });

  it("keeps the same retry handling for a rollback: the recorded reason locks, only the state is retried", () => {
    renderDialog({ target: "rollback", reasonRecorded: true });
    expect(reasonField()).toBeDisabled();
    expect(screen.getByText(/already recorded as an internal note/i)).toBeInTheDocument();
  });

  it("has no copy for answering on the customer's behalf: scheduled and closed fall back to the generic wording", () => {
    // Staff never record a customer's approval or review, so the dialog is never opened for
    // `scheduled` or `closed`; if it ever were, it must not read as a recorded customer answer.
    for (const target of ["scheduled", "closed"]) {
      cleanup();
      renderDialog({ target });
      expect(screen.queryByText(/bypass|on their behalf|the customer's (approval|review)/i)).not.toBeInTheDocument();
      expect(screen.getByText("This change to the record can't be undone from here.")).toBeInTheDocument();
    }
  });

  it("still renders for a target it has no curated copy for", () => {
    renderDialog({ target: "awaiting_vendor" });
    expect(screen.getByRole("heading", { name: /awaiting vendor/i })).toBeInTheDocument();
    expect(reasonField()).toBeInTheDocument();
  });
});

describe("ChangeRequestTransitionReasonDialog — in-flight and error states", () => {
  it("disables both actions and the field while submitting", () => {
    renderDialog({ isSubmitting: true });
    expect(screen.getByRole("button", { name: "Go back" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeDisabled();
    expect(reasonField()).toBeDisabled();
  });

  it("renders no alert by default", () => {
    renderDialog();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("surfaces the caller's error message inline", () => {
    renderDialog({
      error: "Your reason was recorded as an internal note, but the state did not change.",
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      /recorded as an internal note, but the state did not change/i,
    );
  });

  it("locks the reason field once it has already been recorded, so a retry can't post it twice", () => {
    renderDialog({ reasonRecorded: true });
    expect(reasonField()).toBeDisabled();
    expect(
      screen.getByText(/already recorded as an internal note/i),
    ).toBeInTheDocument();
  });
});

describe("ChangeRequestTransitionReasonDialog — wording and accessibility", () => {
  it("says the reason is an internal note the customer does not see", () => {
    renderDialog({ target: "rollback" });
    expect(
      screen.getByText("Recorded as an internal note (not visible to the customer) before the state changes."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/as a comment/i)).not.toBeInTheDocument();
  });

  it("names the way out 'Go back', so it is never read as the action that closes the change", () => {
    renderDialog({ target: "canceled" });
    expect(screen.getByRole("button", { name: "Go back" })).toBeInTheDocument();
    // The one 'Close...' left is not a button of this dialog.
    expect(screen.queryByRole("button", { name: /^close$/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel change" })).toBeInTheDocument();
  });

  it("describes the dialog by its body, which says what the action does", () => {
    renderDialog({ target: "rollback" });
    const dialog = screen.getByRole("dialog");
    const describedBy = dialog.getAttribute("aria-describedby");
    expect(describedBy).toBeTruthy();
    expect(document.getElementById(describedBy!)).toHaveTextContent(
      "This moves the change request into Rollback, recording that the implemented change is being reversed. Rollback is final and can't be undone from here.",
    );
    expect(dialog).toHaveAccessibleName("Roll back this change?");
  });

  it("says what Roll back does and never why: no review is claimed to have failed (it opens from Review and from Customer Review, asked or not)", () => {
    renderDialog({ target: "rollback" });
    const body = document.getElementById(screen.getByRole("dialog").getAttribute("aria-describedby")!)!;
    expect(body).not.toHaveTextContent(/failed|fail\b|review/i);
    expect(body).toHaveTextContent(/implemented change is being reversed/);
    expect(body).toHaveTextContent(/final/);
  });

  it("says what Cancel change does without claiming the given approvals are lost: the waiting ones are withdrawn, the given ones stay on the record", () => {
    renderDialog({ target: "canceled" });
    const body = document.getElementById(screen.getByRole("dialog").getAttribute("aria-describedby")!)!;
    expect(body).toHaveTextContent(
      "This ends the change request as canceled. It can't be reopened from here. Approvals still waiting are withdrawn; the ones already given stay on its record.",
    );
    expect(body).not.toHaveTextContent(/lost/i);
  });

  it("puts focus in the Reason field once the dialog has opened, so typing right away lands in it", async () => {
    renderDialog({ target: "canceled" });
    await waitFor(() => expect(reasonField()).toHaveFocus());
  });

  it("takes focus back from whatever grabbed it while the dialog opened (the menu it was opened from)", async () => {
    const trigger = document.createElement("button");
    document.body.appendChild(trigger);
    renderDialog({ target: "rollback" });
    trigger.focus();
    await waitFor(() => expect(reasonField()).toHaveFocus());
    trigger.remove();
  });
});

describe("ChangeRequestTransitionReasonDialog — dismissal", () => {
  it("closes on Escape", () => {
    const { onClose } = renderDialog();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape", code: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("does not close on Escape while submitting", () => {
    const { onClose } = renderDialog({ isSubmitting: true });
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape", code: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});
