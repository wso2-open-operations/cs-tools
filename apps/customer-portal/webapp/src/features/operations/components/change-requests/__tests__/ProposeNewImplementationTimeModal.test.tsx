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

import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import ProposeNewImplementationTimeModal from "@features/operations/components/change-requests/ProposeNewImplementationTimeModal";
import {
  CHANGE_REQUEST_ACTION_FAILED_MESSAGE,
  CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
  CHANGE_REQUEST_NO_WINDOW_MESSAGE,
  CHANGE_REQUEST_ON_HOLD_MESSAGE,
  CHANGE_REQUEST_PROPOSAL_NOT_NOW_MESSAGE,
  CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE,
  ChangeRequestErrorCode,
} from "@features/operations/utils/changeRequests";
import { ApiError } from "@utils/ApiError";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";

const mocks = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  isPending: { value: false },
}));

vi.mock("@features/operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutateAsync: mocks.mutateAsync,
    isPending: mocks.isPending.value,
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: mocks.showError }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: mocks.showSuccess }),
}));

// The API sends UTC; the viewer is in Colombo (UTC+05:30), so the dialog shows
// 10:00 - 12:00 on 10 June.
const changeRequest = {
  id: "cr-1",
  number: "CHG001",
  type: { id: "normal", label: "Normal" },
  startDate: "2026-06-10 04:30:00",
  endDate: "2026-06-10 06:30:00",
} as never;

function renderModal(
  overrides: Record<string, unknown> = {},
  onClose = vi.fn(),
  onProposed = vi.fn(),
  onRefused = vi.fn(),
) {
  render(
    <ProposeNewImplementationTimeModal
      open
      onClose={onClose}
      onProposed={onProposed}
      onRefused={onRefused}
      changeRequest={{ ...(changeRequest as object), ...overrides } as never}
    />,
  );
  return { onClose, onProposed, onRefused };
}

const startInput = () => screen.getByLabelText(/Proposed start/) as HTMLInputElement;
const endInput = () => screen.getByLabelText(/Proposed end/) as HTMLInputElement;
const setValue = (input: HTMLElement, value: string) =>
  fireEvent.change(input, { target: { value } });
const submit = () => fireEvent.click(screen.getByRole("button", { name: "Submit Proposal" }));

describe("ProposeNewImplementationTimeModal", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-01T00:00:00Z"));
    setUserPreferredTimeZone("Asia/Colombo");
    mocks.mutateAsync.mockReset();
    mocks.mutateAsync.mockResolvedValue({ id: "cr-1" });
    mocks.showError.mockReset();
    mocks.showSuccess.mockReset();
    mocks.isPending.value = false;
  });

  afterEach(() => {
    vi.useRealTimers();
    clearUserPreferredTimeZone();
  });

  it("renders a labelled dialog that says it proposes a start, not an approval, and promises no CAB round trip", () => {
    renderModal();
    const dialog = screen.getByRole("dialog", { name: "Propose New Implementation Time" });
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText(/proposing a new start time, not approving one/)).toBeInTheDocument();
    expect(within(dialog).getByText(/WSO2 will either accept it or suggest a different time/)).toBeInTheDocument();
    expect(within(dialog).getByText(/planned length of 2 hours stays the same/)).toBeInTheDocument();
    expect(within(dialog).getByText(/To ask for a different length, contact WSO2/)).toBeInTheDocument();
    const notice = within(dialog).getByRole("note");
    expect(notice).not.toHaveTextContent(/internal/i);
    expect(notice).not.toHaveTextContent(/approval again/i);
    expect(notice).not.toHaveTextContent(/CAB/);
  });

  it("says the same for every change type: no copy depends on Normal, Standard or Emergency", () => {
    for (const label of ["Normal", "Standard", "Emergency"]) {
      const { unmount } = render(
        <ProposeNewImplementationTimeModal
          open
          onClose={() => {}}
          changeRequest={{ ...(changeRequest as object), type: { id: label, label } } as never}
        />,
      );
      expect(screen.getByRole("note"), label).toHaveTextContent(/WSO2 will either accept it or suggest a different time/);
      expect(screen.getByRole("note"), label).not.toHaveTextContent(/internally/);
      unmount();
    }
  });

  it("prefills the start from the current window, and shows the end it implies, in the viewer's time zone", () => {
    renderModal();
    expect(startInput().value).toBe("2026-06-10T10:00");
    expect(endInput().value).toBe("2026-06-10T12:00");
    expect(startInput()).toBeRequired();
    expect(screen.getByText(/Asia\/Colombo/)).toBeInTheDocument();
    expect(screen.getByText("Same length as the planned window (2 hours)")).toBeInTheDocument();
  });

  it("shows the end as read-only: it is not asked for, so it cannot be edited or required", () => {
    renderModal();
    const end = endInput();
    expect(end).toHaveAttribute("readonly");
    expect(end).toHaveAttribute("aria-readonly", "true");
    expect(end).not.toBeRequired();
    // Not disabled: a disabled field loses its contrast and is skipped by assistive technology.
    expect(end).not.toBeDisabled();
    // A change event on a read-only field is not a way in: what shows is always
    // the end the start implies.
    setValue(end, "2026-06-10T18:00");
    expect(end.value).toBe("2026-06-10T12:00");
    setValue(startInput(), "2026-06-12T09:00");
    expect(end.value).toBe("2026-06-12T11:00");
  });

  it("leaves the start empty, and the end with it, when the change request has no window yet, and says it cannot be proposed", () => {
    renderModal({ startDate: "", endDate: "" });
    expect(startInput().value).toBe("");
    expect(endInput().value).toBe("");
    expect(startInput()).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent(CHANGE_REQUEST_NO_WINDOW_MESSAGE);
    expect(screen.getByRole("button", { name: "Submit Proposal" })).toBeDisabled();
    expect(screen.getByRole("note")).not.toHaveTextContent(/planned length of/);
  });

  it("moves the end with the start, keeping the planned length", () => {
    renderModal();
    setValue(startInput(), "2026-06-12T09:00");
    expect(endInput().value).toBe("2026-06-12T11:00");
    setValue(startInput(), "2026-06-13T22:30");
    expect(endInput().value).toBe("2026-06-14T00:30");
  });

  it("empties the end while the start is not a time", () => {
    renderModal();
    setValue(startInput(), "");
    expect(endInput().value).toBe("");
  });

  it("shows no errors before the first submit", () => {
    renderModal();
    setValue(startInput(), "");
    expect(screen.queryByText(/Enter the proposed/)).not.toBeInTheDocument();
  });

  it("requires the start, shows the error inline and sends nothing", () => {
    renderModal();
    setValue(startInput(), "");
    submit();
    expect(screen.getByText("Enter the proposed start date and time.")).toBeInTheDocument();
    expect(startInput()).toHaveAttribute("aria-invalid", "true");
    expect(document.activeElement).toBe(startInput());
    expect(mocks.mutateAsync).not.toHaveBeenCalled();
  });

  it("refuses a start in the past, and clears the error once fixed", () => {
    renderModal();
    setValue(startInput(), "2026-04-30T10:00");
    submit();
    expect(screen.getByText("The proposed start must be in the future.")).toBeInTheDocument();
    expect(mocks.mutateAsync).not.toHaveBeenCalled();

    setValue(startInput(), "2026-06-12T10:00");
    expect(screen.queryByText("The proposed start must be in the future.")).not.toBeInTheDocument();
  });

  it("refuses the start that is the current one", () => {
    renderModal();
    submit();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This is the same as the current schedule. Choose a different start.",
    );
    expect(mocks.mutateAsync).not.toHaveBeenCalled();
  });

  it("sends the start and the end that keeps the planned length, both as UTC for a viewer outside UTC, then says what happens next and closes", async () => {
    const { onClose } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    expect(endInput().value).toBe("2026-06-11T17:30");
    submit();

    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(mocks.mutateAsync).toHaveBeenCalledTimes(1);
    expect(mocks.mutateAsync).toHaveBeenCalledWith({
      plannedStartOn: "2026-06-11 10:00:00",
      plannedEndOn: "2026-06-11 12:00:00",
    });
    expect(mocks.showSuccess).toHaveBeenCalledWith(
      "New time proposed. WSO2 will accept it or suggest a different time, and the answer will appear on this page.",
    );
    expect(mocks.showError).not.toHaveBeenCalled();
  });

  it("says the same on success for a Standard change: the type changes nothing", async () => {
    const { onClose } = renderModal({ type: { id: "standard", label: "Standard" } });
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mocks.showSuccess).toHaveBeenCalledWith(
      "New time proposed. WSO2 will accept it or suggest a different time, and the answer will appear on this page.",
    );
  });

  it("keeps a length that is not a whole number of hours, and a window that crosses midnight", async () => {
    const { onClose } = renderModal({ startDate: "2026-06-10 04:30:00", endDate: "2026-06-10 06:15:00" });
    setValue(startInput(), "2026-06-11T23:00");
    expect(endInput().value).toBe("2026-06-12T00:45");
    submit();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mocks.mutateAsync).toHaveBeenCalledWith({
      plannedStartOn: "2026-06-11 17:30:00",
      plannedEndOn: "2026-06-11 19:15:00",
    });
  });

  it("tells the page it was proposed, only when it was", async () => {
    mocks.mutateAsync.mockRejectedValueOnce(new ApiError(400, "Bad Request", "plannedStartOn is in the past: a proposed implementation time must be one still to come"));
    const { onClose, onProposed } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    expect(await screen.findByText("The proposed time must be in the future.")).toBeInTheDocument();
    expect(onProposed).not.toHaveBeenCalled();

    submit();
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(onProposed).toHaveBeenCalledTimes(1);
  });

  it("submits when Enter is pressed in the field: the field and the button are one form", async () => {
    const { onClose } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    const form = startInput().closest("form");
    expect(form).not.toBeNull();
    expect(screen.getByRole("button", { name: "Submit Proposal" })).toHaveAttribute("type", "submit");
    // What the browser does for Enter in a field of a form with a submit button.
    fireEvent.submit(form as HTMLFormElement);
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(mocks.mutateAsync).toHaveBeenCalledTimes(1);
  });

  it("does not skip a heading level: the two sections sit under the title as h3", () => {
    renderModal();
    expect(screen.getByRole("heading", { level: 2, name: /Propose New Implementation Time/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "Current Schedule" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "Proposed implementation time" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 6 })).not.toBeInTheDocument();
  });

  describe("when a proposed time already waits for WSO2", () => {
    const waiting = (extra: object = {}) => ({
      state: { id: "5", label: "Customer Approval" },
      customerProposal: { startDate: "2026-06-12 04:30:00", endDate: "2026-06-12 06:30:00", answer: "pending", ...extra },
    });

    it("shows the standing proposal beside the current schedule, in the viewer's zone", () => {
      renderModal(waiting({ proposedByViewer: true }));
      expect(screen.getByText("Proposed start waiting for WSO2")).toBeInTheDocument();
      expect(screen.getByText(/Friday, June 12, 2026 at 10:00 AM GMT\+5:30/)).toBeInTheDocument();
      // The field still starts from what is planned now.
      expect(startInput().value).toBe("2026-06-10T10:00");
    });

    it("shows no standing proposal when nothing waits (an accepted or declined one is history)", () => {
      for (const answer of ["agreed", "disagreed", "unanswered"]) {
        const { unmount } = render(
          <ProposeNewImplementationTimeModal
            open
            onClose={() => {}}
            changeRequest={{ ...(changeRequest as object), ...waiting({ answer }) } as never}
          />,
        );
        expect(screen.queryByText("Proposed start waiting for WSO2"), answer).not.toBeInTheDocument();
        unmount();
      }
    });

    it("refuses the time that already waits, a colleague's included, before any request", () => {
      renderModal(waiting());
      setValue(startInput(), "2026-06-12T10:00");
      submit();
      expect(screen.getByRole("alert")).toHaveTextContent(
        "That time is already proposed and is waiting for WSO2's response. Choose a different start.",
      );
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });

    it("takes another start, which replaces the standing proposal", async () => {
      const { onClose } = renderModal(waiting({ proposedByViewer: true }));
      setValue(startInput(), "2026-06-13T10:00");
      submit();
      await waitFor(() => expect(onClose).toHaveBeenCalled());
      expect(mocks.mutateAsync).toHaveBeenCalledWith({
        plannedStartOn: "2026-06-13 04:30:00",
        plannedEndOn: "2026-06-13 06:30:00",
      });
    });
  });

  it("keeps the dialog open with the backend's message when the start is refused, and lets the customer fix it", async () => {
    mocks.mutateAsync.mockRejectedValueOnce(
      new ApiError(400, "Bad Request", "plannedStartOn is the planned start already: propose a different start"),
    );
    const { onClose } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();

    expect(
      await screen.findByText("This is the same as the current schedule. Choose a different start."),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.showSuccess).not.toHaveBeenCalled();
    // The customer can fix it and send again.
    mocks.mutateAsync.mockResolvedValueOnce({ id: "cr-1" });
    setValue(startInput(), "2026-06-11T19:00");
    submit();
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(mocks.mutateAsync).toHaveBeenCalledTimes(2);
  });

  it.each([
    [ChangeRequestErrorCode.APPROVAL_NOT_PENDING, CHANGE_REQUEST_ANSWER_STALE_MESSAGE],
    [ChangeRequestErrorCode.NOT_PROPOSABLE, CHANGE_REQUEST_ANSWER_STALE_MESSAGE],
    [ChangeRequestErrorCode.SCHEDULE_CHANGED, CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE],
  ])("closes and says so on the page when the change request no longer waits on the customer (409 %s)", async (code, message) => {
    mocks.mutateAsync.mockRejectedValueOnce(new ApiError(409, "Conflict", "any wording at all", undefined, code));
    const { onClose, onRefused, onProposed } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(mocks.showError).toHaveBeenCalledWith(message);
    expect(mocks.showSuccess).not.toHaveBeenCalled();
    // The page is told it was refused (not proposed) before the dialog closes, so
    // that it can move focus off the controls that are going away.
    expect(onRefused).toHaveBeenCalledTimes(1);
    expect(onProposed).not.toHaveBeenCalled();
    expect(onRefused.mock.invocationCallOrder[0]).toBeLessThan(onClose.mock.invocationCallOrder[0]);
  });

  it("stays open and says so when WSO2 has the change on hold (409)", async () => {
    mocks.mutateAsync.mockRejectedValueOnce(
      new ApiError(409, "Conflict", "any wording at all", undefined, ChangeRequestErrorCode.ON_HOLD),
    );
    const { onClose, onRefused } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    expect(await screen.findByText(CHANGE_REQUEST_ON_HOLD_MESSAGE)).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(onRefused).not.toHaveBeenCalled();
    expect(mocks.showError).not.toHaveBeenCalled();
  });

  it("stays open and says so when the service finds no window to move (409)", async () => {
    mocks.mutateAsync.mockRejectedValueOnce(
      new ApiError(409, "Conflict", "any wording at all", undefined, ChangeRequestErrorCode.NO_PLANNED_WINDOW),
    );
    const { onClose, onRefused } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    expect(await screen.findByText(CHANGE_REQUEST_NO_WINDOW_MESSAGE)).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(onRefused).not.toHaveBeenCalled();
    expect(mocks.showError).not.toHaveBeenCalled();
  });

  it("stays open and says only that it cannot be proposed right now when another approval is being asked (409)", async () => {
    mocks.mutateAsync.mockRejectedValueOnce(
      new ApiError(409, "Conflict", "any wording at all", undefined, ChangeRequestErrorCode.PROPOSAL_NOT_NOW),
    );
    const { onClose, onRefused } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    expect(await screen.findByText(CHANGE_REQUEST_PROPOSAL_NOT_NOW_MESSAGE)).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(onRefused).not.toHaveBeenCalled();
    expect(mocks.showError).not.toHaveBeenCalled();
  });

  it("keeps focus in the dialog, on the button that was pressed, when the hold keeps it open", async () => {
    // The button is switched off while the request is in flight and focus leaves
    // it; the failure that keeps the dialog open puts it back.
    mocks.mutateAsync.mockImplementationOnce(async () => {
      (document.activeElement as HTMLElement | null)?.blur();
      throw new ApiError(409, "Conflict", "any wording", undefined, ChangeRequestErrorCode.ON_HOLD);
    });
    renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    const button = screen.getByRole("button", { name: "Submit Proposal" });
    button.focus();
    submit();
    await screen.findByText(CHANGE_REQUEST_ON_HOLD_MESSAGE);
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Submit Proposal" })));
  });

  it("closes, as for any 409, and claims nothing it cannot know when the 409 names nothing it knows", async () => {
    for (const code of [undefined, "change_request_from_the_future"]) {
      mocks.showError.mockReset();
      mocks.mutateAsync.mockRejectedValueOnce(new ApiError(409, "Conflict", "this change request is on hold", undefined, code));
      const { onClose, onRefused } = renderModal();
      setValue(startInput(), "2026-06-11T15:30");
      submit();
      await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
      expect(mocks.showError).toHaveBeenCalledWith(CHANGE_REQUEST_ACTION_FAILED_MESSAGE);
      expect(mocks.showError).not.toHaveBeenCalledWith(CHANGE_REQUEST_ANSWER_STALE_MESSAGE);
      expect(onRefused).toHaveBeenCalledTimes(1);
      cleanup();
    }
  });

  it.each([ChangeRequestErrorCode.NOT_ASKED, ChangeRequestErrorCode.FORBIDDEN, undefined])(
    "closes and says so on the page when the customer may not answer it (403 %s)",
    async (code) => {
      mocks.mutateAsync.mockRejectedValueOnce(new ApiError(403, "Forbidden", "nope", undefined, code));
      const { onClose, onRefused } = renderModal();
      setValue(startInput(), "2026-06-11T15:30");
      submit();
      await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
      expect(mocks.showError).toHaveBeenCalledWith(CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE);
      expect(onRefused).toHaveBeenCalledTimes(1);
    },
  );

  it("keeps the dialog open with a plain message when the request itself failed", async () => {
    mocks.mutateAsync.mockRejectedValueOnce(new Error("Failed to fetch"));
    const { onClose } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    expect(await screen.findByText("Could not submit your proposal. Please try again.")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("sends once however often submit is pressed while the request is in flight", async () => {
    let resolve: (v: unknown) => void = () => {};
    mocks.mutateAsync.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
    const { onClose } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    submit();
    submit();
    expect(mocks.mutateAsync).toHaveBeenCalledTimes(1);
    await act(async () => { resolve({ id: "cr-1" }); });
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it("locks the dialog while submitting", () => {
    mocks.isPending.value = true;
    const { onClose } = renderModal();
    expect(screen.getByRole("button", { name: "Submitting..." })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
    expect(startInput()).toBeDisabled();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes on Escape and on Cancel when idle", () => {
    const { onClose } = renderModal();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("renders nothing when closed or without a change request", () => {
    const { rerender } = render(
      <ProposeNewImplementationTimeModal open={false} onClose={() => {}} changeRequest={changeRequest} />,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    rerender(<ProposeNewImplementationTimeModal open onClose={() => {}} changeRequest={null} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
