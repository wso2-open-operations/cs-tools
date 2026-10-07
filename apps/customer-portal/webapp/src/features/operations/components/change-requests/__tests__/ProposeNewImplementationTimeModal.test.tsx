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
  CHANGE_REQUEST_ON_HOLD_MESSAGE,
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

  it("renders a labelled dialog that says it proposes a time, reviewed internally first", () => {
    renderModal();
    const dialog = screen.getByRole("dialog", { name: "Propose New Implementation Time" });
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText(/proposing a new time, not approving one/)).toBeInTheDocument();
    expect(within(dialog).getByText(/WSO2 will review it internally first/)).toBeInTheDocument();
    expect(within(dialog).getByText(/you will then be asked to approve the new time/)).toBeInTheDocument();
  });

  it("prefills the start and the end from the current window, in the viewer's time zone", () => {
    renderModal();
    expect(startInput().value).toBe("2026-06-10T10:00");
    expect(endInput().value).toBe("2026-06-10T12:00");
    expect(startInput()).toBeRequired();
    expect(endInput()).toBeRequired();
    expect(screen.getByText(/Asia\/Colombo/)).toBeInTheDocument();
  });

  it("leaves both fields empty when the change request has no window yet", () => {
    renderModal({ startDate: "", endDate: "" });
    expect(startInput().value).toBe("");
    expect(endInput().value).toBe("");
  });

  it("moves the end with the start, keeping the length, until the end is edited by hand", () => {
    renderModal();
    setValue(startInput(), "2026-06-12T09:00");
    expect(endInput().value).toBe("2026-06-12T11:00");

    setValue(endInput(), "2026-06-12T15:00");
    setValue(startInput(), "2026-06-13T09:00");
    expect(endInput().value).toBe("2026-06-12T15:00");
  });

  it("does not invent an end when the change request has no length to keep", () => {
    renderModal({ startDate: "", endDate: "" });
    setValue(startInput(), "2026-06-12T09:00");
    expect(endInput().value).toBe("");
  });

  it("shows no errors before the first submit", () => {
    renderModal({ startDate: "", endDate: "" });
    expect(screen.queryByText(/Enter the proposed/)).not.toBeInTheDocument();
  });

  it("requires both fields, shows the errors inline and sends nothing", () => {
    renderModal({ startDate: "", endDate: "" });
    submit();
    expect(screen.getByText("Enter the proposed start date and time.")).toBeInTheDocument();
    expect(screen.getByText("Enter the proposed end date and time.")).toBeInTheDocument();
    expect(startInput()).toHaveAttribute("aria-invalid", "true");
    expect(document.activeElement).toBe(startInput());
    expect(mocks.mutateAsync).not.toHaveBeenCalled();
  });

  it("refuses a start in the past", () => {
    renderModal();
    setValue(startInput(), "2026-04-30T10:00");
    setValue(endInput(), "2026-04-30T12:00");
    submit();
    expect(screen.getByText("The proposed start must be in the future.")).toBeInTheDocument();
    expect(mocks.mutateAsync).not.toHaveBeenCalled();
  });

  it("refuses an end that is not after the start, and clears the error once fixed", () => {
    renderModal();
    setValue(endInput(), "2026-06-10T09:00");
    submit();
    expect(screen.getByText("The proposed end must be after the proposed start.")).toBeInTheDocument();
    expect(document.activeElement).toBe(endInput());
    expect(mocks.mutateAsync).not.toHaveBeenCalled();

    setValue(endInput(), "2026-06-10T13:00");
    expect(screen.queryByText("The proposed end must be after the proposed start.")).not.toBeInTheDocument();
  });

  it("refuses a window that is the same as the current one", () => {
    renderModal();
    submit();
    expect(screen.getByRole("alert")).toHaveTextContent(/same as the current schedule/);
    expect(mocks.mutateAsync).not.toHaveBeenCalled();
  });

  it("sends both ends as UTC for a viewer outside UTC, then says what happens next and closes", async () => {
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
      "New time proposed. We'll ask for your approval again once it's confirmed internally.",
    );
    expect(mocks.showError).not.toHaveBeenCalled();
  });

  it("sends the start too when only the end was changed", async () => {
    const { onClose } = renderModal();
    setValue(endInput(), "2026-06-10T14:00");
    submit();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mocks.mutateAsync).toHaveBeenCalledWith({
      plannedStartOn: "2026-06-10 04:30:00",
      plannedEndOn: "2026-06-10 08:30:00",
    });
  });

  it("tells a Standard change's customer they will be asked to approve the updated schedule", async () => {
    const { onClose } = renderModal({ type: { id: "standard", label: "Standard" } });
    expect(screen.queryByText(/review it internally/)).not.toBeInTheDocument();
    setValue(startInput(), "2026-06-11T15:30");
    submit();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mocks.showSuccess).toHaveBeenCalledWith(
      "New time proposed. Review the updated schedule and approve it when you are ready.",
    );
  });

  it("tells the page it was proposed (so focus can move on), only when it was", async () => {
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

  it("submits when Enter is pressed in a field: the fields and the button are one form", async () => {
    const { onClose } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    const form = endInput().closest("form");
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
    expect(screen.getByRole("heading", { level: 3, name: "Proposed implementation window" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 6 })).not.toBeInTheDocument();
  });

  it("keeps the dialog open with the backend's message when the window is refused", async () => {
    mocks.mutateAsync.mockRejectedValueOnce(
      new ApiError(400, "Bad Request", "the planned start must not be after the planned end"),
    );
    const { onClose } = renderModal();
    setValue(startInput(), "2026-06-11T15:30");
    submit();

    expect(await screen.findByText("The proposed end must be after the proposed start.")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.showSuccess).not.toHaveBeenCalled();
    // The customer can fix it and send again.
    mocks.mutateAsync.mockResolvedValueOnce({ id: "cr-1" });
    setValue(endInput(), "2026-06-11T19:00");
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
