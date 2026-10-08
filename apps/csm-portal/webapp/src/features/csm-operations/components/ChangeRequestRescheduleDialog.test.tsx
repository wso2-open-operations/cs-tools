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
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestRescheduleDialog from "@features/csm-operations/components/ChangeRequestRescheduleDialog";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";
import type { BeChangeRequestDetail } from "@api/backend/types";

const CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "customer_approval",
  type: "normal",
  plannedStartOn: "2030-03-01 09:00:00",
  plannedEndOn: "2030-03-01 11:00:00",
};

function renderDialog(
  props: Partial<Omit<ComponentProps<typeof ChangeRequestRescheduleDialog>, "cr">> & { cr?: Partial<BeChangeRequestDetail> } = {},
): { onSubmit: ReturnType<typeof vi.fn>; onClose: ReturnType<typeof vi.fn> } {
  const onSubmit = vi.fn();
  const onClose = vi.fn();
  const { cr, ...rest } = props;
  render(
    <ChangeRequestRescheduleDialog
      cr={{ ...CR, ...cr }}
      isSubmitting={false}
      onClose={onClose}
      onSubmit={onSubmit}
      {...rest}
    />,
  );
  return { onSubmit, onClose };
}

// The picker's hidden <input> carries the whole value as text; changing it is
// how a complete date is entered without driving every section.
function pickerInput(label: string): HTMLInputElement {
  const group = screen.getAllByText(label)[0].closest(".MuiFormControl-root") as HTMLElement;
  return group.querySelector("input") as HTMLInputElement;
}

const submitButton = (): HTMLElement => screen.getByRole("button", { name: "Re-schedule" });
const dialogButton = (name: string): HTMLElement => within(screen.getByRole("dialog")).getByRole("button", { name });

describe("ChangeRequestRescheduleDialog", () => {
  beforeEach(() => setUserPreferredTimeZone("UTC"));
  afterEach(() => clearUserPreferredTimeZone());

  it("is prefilled with the current planned window and blocks submitting until one end changes", () => {
    renderDialog();
    expect(pickerInput("Planned start").value).toBe("03/01/2030 09:00 AM");
    expect(pickerInput("Planned end").value).toBe("03/01/2030 11:00 AM");
    expect(submitButton()).toBeDisabled();
    expect(screen.getByText(/change the planned start or end to re-schedule/i)).toBeInTheDocument();
  });

  it("re-entering the current value is not a change", () => {
    renderDialog();
    fireEvent.change(pickerInput("Planned start"), { target: { value: "03/01/2030 09:00 AM" } });
    expect(submitButton()).toBeDisabled();
  });

  it("sends only the changed end as UTC, with state authorize", () => {
    const { onSubmit } = renderDialog();
    fireEvent.change(pickerInput("Planned end"), { target: { value: "03/01/2030 01:00 PM" } });
    expect(submitButton()).toBeEnabled();
    fireEvent.click(submitButton());
    expect(onSubmit).toHaveBeenCalledWith({ state: "authorize", plannedEndOn: "2030-03-01 13:00:00" }, "");
  });

  it("sends both ends and the trimmed optional reason", () => {
    const { onSubmit } = renderDialog();
    fireEvent.change(pickerInput("Planned start"), { target: { value: "03/08/2030 09:00 AM" } });
    fireEvent.change(pickerInput("Planned end"), { target: { value: "03/08/2030 11:00 AM" } });
    fireEvent.change(screen.getByLabelText(/reason \(optional\)/i), { target: { value: "  Customer freeze.  " } });
    fireEvent.click(submitButton());
    expect(onSubmit).toHaveBeenCalledWith(
      { state: "authorize", plannedStartOn: "2030-03-08 09:00:00", plannedEndOn: "2030-03-08 11:00:00" },
      "Customer freeze.",
    );
  });

  it("blocks an end that is not after the start", () => {
    renderDialog();
    fireEvent.change(pickerInput("Planned start"), { target: { value: "03/01/2030 12:00 PM" } });
    expect(screen.getByText(/planned end must be after planned start/i)).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });

  it("shows the backend's refusal verbatim", () => {
    renderDialog({ error: "re-scheduling requires a changed planned start or end" });
    expect(screen.getByRole("alert")).toHaveTextContent("re-scheduling requires a changed planned start or end");
  });

  it("explains the new time goes to the customer and no CAB approval is involved, for every type of change", () => {
    for (const type of ["normal", "standard", "emergency"]) {
      cleanup();
      renderDialog({ cr: { type } });
      expect(screen.getByText(/The customer is asked to approve it\. No further internal approval is needed: the change itself has not changed\./), type).toBeInTheDocument();
      // The CAB loop is gone: nothing says the change goes back to Authorize or to the CAB.
      expect(screen.queryByText(/Authorize|CAB|goes back|stays in Customer Approval/), type).not.toBeInTheDocument();
    }
  });

  it("closes via Close", () => {
    const { onClose } = renderDialog();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("disables the inputs and both actions while submitting", () => {
    renderDialog({ isSubmitting: true });
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" })).toBeDisabled();
    expect(submitButton()).toBeDisabled();
    expect(screen.getByLabelText(/reason \(optional\)/i)).toBeDisabled();
  });

  it("keeps the reason editable, and says it is recorded once the change has been updated (never before the PATCH)", () => {
    renderDialog();
    expect(screen.getByLabelText(/reason \(optional\)/i)).toBeEnabled();
    expect(screen.getByText("Recorded as an internal work note once the change has been updated.")).toBeInTheDocument();
    // There is no "already recorded" lock: nothing is recorded by an attempt that was refused.
    expect(screen.queryByText(/Already recorded/)).not.toBeInTheDocument();
  });

  describe("stale: the page moved on behind a refused attempt", () => {
    it("holds submit back and says to close the dialog, for a plain Re-schedule", () => {
      renderDialog({ stale: true, error: "the planned implementation time of this change request changed after you opened it" });
      fireEvent.change(pickerInput("Planned end"), { target: { value: "03/01/2030 01:00 PM" } });
      expect(submitButton()).toBeDisabled();
      expect(screen.getByRole("alert")).toHaveTextContent("changed after you opened it");
      expect(screen.getByRole("status")).toHaveTextContent(/Close\s+this dialog to see the current state/);
      // Closing is still possible, and reading the reason is unchanged.
      expect(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" })).toBeEnabled();
    });

    it("is not shown, and submit follows the window alone, when nothing moved", () => {
      renderDialog();
      fireEvent.change(pickerInput("Planned end"), { target: { value: "03/01/2030 01:00 PM" } });
      expect(submitButton()).toBeEnabled();
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });
  });

  describe("the customer proposed a time (counter mode)", () => {
    // A proposer on record: the dialog then says the customer proposed it. `UNATTRIBUTED` is the same time with nobody on record.
    const PROPOSAL = {
      startOn: "2030-03-08T09:00:00Z",
      endOn: "2030-03-08T11:00:00Z",
      answer: "pending",
      proposerRecorded: true,
      proposedByName: "Mia Member",
      proposedByEmail: "mia.member@example.com",
      proposedOn: "2030-02-01T10:00:00Z",
    };
    const UNATTRIBUTED = { startOn: PROPOSAL.startOn, endOn: PROPOSAL.endOn, answer: "pending" };
    // The window the page showed, as received: the answer's precondition.
    const SHOWN = { expectedPlannedStartOn: "2030-03-01 09:00:00", expectedPlannedEndOn: "2030-03-01 11:00:00" };

    it("is 'Propose a different time', says CAB is not involved and that keeping the current time declines", () => {
      renderDialog({ proposal: PROPOSAL });
      expect(screen.getByRole("heading", { name: "Propose a different time" })).toBeInTheDocument();
      expect(screen.getByText(/The customer proposed Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM\./)).toBeInTheDocument();
      expect(screen.getByText(/Set the time WSO2 proposes instead and the customer is asked to approve it\./)).toBeInTheDocument();
      expect(screen.getByText(/Keep the current time to decline the proposal\. No CAB approval is needed\./)).toBeInTheDocument();
      // Nothing of the plain Re-schedule hint, and nothing of the old CAB loop.
      expect(screen.queryByText(/change the planned start or end to re-schedule/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/Authorize|goes back/)).not.toBeInTheDocument();
    });

    it("is prefilled with the PLANNED window, not the customer's, and offers to decline as it stands", () => {
      renderDialog({ proposal: PROPOSAL });
      expect(pickerInput("Planned start").value).toBe("03/01/2030 09:00 AM");
      expect(pickerInput("Planned end").value).toBe("03/01/2030 11:00 AM");
      expect(dialogButton("Decline proposed time")).toBeEnabled();
      expect(screen.getByText(/The current time stays, so the proposal is declined\./)).toBeInTheDocument();
    });

    it("a decline sends no window, only the version of the proposal and of the window it was shown", () => {
      const { onSubmit } = renderDialog({ proposal: PROPOSAL });
      fireEvent.click(dialogButton("Decline proposed time"));
      expect(onSubmit).toHaveBeenCalledWith(
        { state: "authorize", expectedCustomerUpdatedOn: "2030-03-08T09:00:00Z", ...SHOWN },
        "",
      );
    });

    it("a different time sends only what changed, with the same preconditions, and the button says what it does", () => {
      const { onSubmit } = renderDialog({ proposal: PROPOSAL });
      fireEvent.change(pickerInput("Planned start"), { target: { value: "03/15/2030 09:00 AM" } });
      fireEvent.change(pickerInput("Planned end"), { target: { value: "03/15/2030 12:00 PM" } });
      expect(screen.queryByRole("button", { name: "Decline proposed time" })).not.toBeInTheDocument();
      expect(screen.queryByText(/so the proposal is declined/)).not.toBeInTheDocument();
      fireEvent.click(dialogButton("Propose this time"));
      expect(onSubmit).toHaveBeenCalledWith(
        {
          state: "authorize",
          plannedStartOn: "2030-03-15 09:00:00",
          plannedEndOn: "2030-03-15 12:00:00",
          expectedCustomerUpdatedOn: "2030-03-08T09:00:00Z",
          ...SHOWN,
        },
        "",
      );
    });

    it("refuses to send the very time the customer proposed: that is Accept proposed time, and says so", () => {
      const { onSubmit } = renderDialog({ proposal: PROPOSAL });
      fireEvent.change(pickerInput("Planned start"), { target: { value: "03/08/2030 09:00 AM" } });
      fireEvent.change(pickerInput("Planned end"), { target: { value: "03/08/2030 11:00 AM" } });
      expect(dialogButton("Propose this time")).toBeDisabled();
      expect(screen.getByRole("status")).toHaveTextContent("That is the time the customer proposed. Close this and use Accept proposed time instead.");
      fireEvent.click(dialogButton("Propose this time"));
      expect(onSubmit).not.toHaveBeenCalled();
    });

    describe("nobody is recorded as having proposed the stored time (a date WSO2 users write too, or one left over from an earlier round)", () => {
      const STORED = /A time is stored \(Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM\) but nobody is recorded as having proposed it\./;

      it("says a time is stored and nobody proposed it, and that there is nothing to decline", () => {
        for (const proposal of [UNATTRIBUTED, { ...UNATTRIBUTED, proposerRecorded: false, proposedByName: "Mia Member" }]) {
          cleanup();
          renderDialog({ proposal });
          expect(screen.getByRole("heading", { name: "Propose a different time" })).toBeInTheDocument();
          expect(screen.getByText(STORED)).toBeInTheDocument();
          expect(screen.getByText(/There is no proposal to decline\./)).toBeInTheDocument();
          expect(screen.getByText(/Set the time WSO2 proposes and the customer is asked to approve it\. No CAB approval is needed\./)).toBeInTheDocument();
          expect(screen.queryByText(/The customer proposed|Keep the current time|decline the proposal/)).not.toBeInTheDocument();
        }
      });

      it("is a plain Re-schedule: no decline, the window must change, and the hint says so", () => {
        const { onSubmit } = renderDialog({ proposal: UNATTRIBUTED });
        // Left as it is there is nothing to send: no "Decline proposed time" in this mode.
        expect(screen.queryByRole("button", { name: "Decline proposed time" })).not.toBeInTheDocument();
        expect(dialogButton("Propose this time")).toBeDisabled();
        expect(screen.getByText("Change the planned start or end to propose a time.")).toBeInTheDocument();
        fireEvent.click(dialogButton("Propose this time"));
        expect(onSubmit).not.toHaveBeenCalled();
        fireEvent.change(pickerInput("Planned end"), { target: { value: "03/01/2030 01:00 PM" } });
        expect(dialogButton("Propose this time")).toBeEnabled();
        expect(screen.queryByText("Change the planned start or end to propose a time.")).not.toBeInTheDocument();
      });

      it("the stored time itself may be named as the new window: nobody proposed it, so nothing says to use Accept", () => {
        const { onSubmit } = renderDialog({ proposal: UNATTRIBUTED });
        fireEvent.change(pickerInput("Planned start"), { target: { value: "03/08/2030 09:00 AM" } });
        fireEvent.change(pickerInput("Planned end"), { target: { value: "03/08/2030 11:00 AM" } });
        expect(screen.queryByRole("status")).not.toBeInTheDocument();
        expect(dialogButton("Propose this time")).toBeEnabled();
        fireEvent.click(dialogButton("Propose this time"));
        expect(onSubmit).toHaveBeenCalledWith(
          {
            state: "authorize",
            plannedStartOn: "2030-03-08 09:00:00",
            plannedEndOn: "2030-03-08 11:00:00",
            expectedCustomerUpdatedOn: "2030-03-08T09:00:00Z",
            ...SHOWN,
          },
          "",
        );
      });

      it("names the stored time and the planned window it was shown, so one that moved is refused in words", () => {
        const { onSubmit } = renderDialog({ proposal: UNATTRIBUTED });
        fireEvent.change(pickerInput("Planned end"), { target: { value: "03/01/2030 01:00 PM" } });
        fireEvent.click(dialogButton("Propose this time"));
        expect(onSubmit).toHaveBeenCalledWith(
          { state: "authorize", plannedEndOn: "2030-03-01 13:00:00", expectedCustomerUpdatedOn: "2030-03-08T09:00:00Z", ...SHOWN },
          "",
        );
      });

      it("an end that is not after the start is still blocked", () => {
        renderDialog({ proposal: UNATTRIBUTED });
        fireEvent.change(pickerInput("Planned start"), { target: { value: "03/01/2030 12:00 PM" } });
        expect(screen.getByText(/planned end must be after planned start/i)).toBeInTheDocument();
        expect(dialogButton("Propose this time")).toBeDisabled();
      });
    });

    it("the same start with another end is a different window, so it can be proposed", () => {
      renderDialog({ proposal: PROPOSAL });
      fireEvent.change(pickerInput("Planned start"), { target: { value: "03/08/2030 09:00 AM" } });
      fireEvent.change(pickerInput("Planned end"), { target: { value: "03/08/2030 01:00 PM" } });
      expect(dialogButton("Propose this time")).toBeEnabled();
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });

    it("still blocks an end that is not after the start", () => {
      renderDialog({ proposal: PROPOSAL });
      fireEvent.change(pickerInput("Planned start"), { target: { value: "03/01/2030 12:00 PM" } });
      expect(screen.getByText(/planned end must be after planned start/i)).toBeInTheDocument();
      expect(dialogButton("Propose this time")).toBeDisabled();
    });

    it("hands the trimmed reason to the caller, which records it after the change has been updated", () => {
      const { onSubmit } = renderDialog({ proposal: PROPOSAL });
      fireEvent.change(screen.getByLabelText(/reason \(optional\)/i), { target: { value: "  Freeze that week.  " } });
      fireEvent.click(dialogButton("Decline proposed time"));
      expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ state: "authorize" }), "Freeze that week.");
    });

    it("holds the counter back when stale, the decline included", () => {
      renderDialog({ proposal: PROPOSAL, stale: true });
      expect(dialogButton("Decline proposed time")).toBeDisabled();
      fireEvent.change(pickerInput("Planned start"), { target: { value: "03/15/2030 09:00 AM" } });
      fireEvent.change(pickerInput("Planned end"), { target: { value: "03/15/2030 11:00 AM" } });
      expect(dialogButton("Propose this time")).toBeDisabled();
    });

    it("shows the backend's refusal verbatim", () => {
      renderDialog({ proposal: PROPOSAL, error: "the customer's proposed time is no longer waiting for a response; read the change request again" });
      expect(screen.getByRole("alert")).toHaveTextContent("the customer's proposed time is no longer waiting for a response; read the change request again");
    });

    it("sends nothing of the proposal when no proposal waits (a plain Re-schedule carries no version)", () => {
      const { onSubmit } = renderDialog();
      fireEvent.change(pickerInput("Planned end"), { target: { value: "03/01/2030 01:00 PM" } });
      fireEvent.click(submitButton());
      const [patch] = onSubmit.mock.calls[0] as [Record<string, unknown>];
      expect(Object.keys(patch).sort()).toEqual(["plannedEndOn", "state"]);
    });
  });
});
