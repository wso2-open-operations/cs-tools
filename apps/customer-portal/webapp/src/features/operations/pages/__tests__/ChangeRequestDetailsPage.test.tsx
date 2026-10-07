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

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ChangeRequestDetailsPage from "@features/operations/pages/ChangeRequestDetailsPage";
import {
  CHANGE_REQUEST_ACTION_FAILED_MESSAGE,
  CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
  CHANGE_REQUEST_NOT_FOUND_MESSAGE,
  CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE,
  ChangeRequestErrorCode,
} from "@features/operations/utils/changeRequests";
import { ApiError } from "@utils/ApiError";

const mocks = vi.hoisted(() => ({
  changeRequest: { value: null as Record<string, unknown> | null },
  error: { value: null as unknown },
  mutateAsync: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  isPending: { value: false },
}));

vi.mock("@features/operations/api/useGetChangeRequestDetails", () => ({
  default: () => ({
    data: mocks.changeRequest.value,
    error: mocks.error.value,
    isLoading: false,
    isFetching: false,
    isError: mocks.error.value != null,
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: mocks.showError }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: mocks.showSuccess }),
}));

vi.mock("@features/operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutateAsync: mocks.mutateAsync,
    isPending: mocks.isPending.value,
  }),
}));

const STATES = {
  approval: { id: "5", label: "Customer Approval" },
  review: { id: "1", label: "Customer Review" },
  scheduled: { id: "-2", label: "Scheduled" },
  authorize: { id: "-3", label: "Authorize" },
};

function makeChangeRequest(overrides: Record<string, unknown> = {}) {
  return {
    id: "cr-1",
    number: "CHG001",
    title: "Deploy patch",
    state: STATES.scheduled,
    type: { id: "normal", label: "Normal" },
    project: { id: "p1", label: "Proj", number: null },
    case: null,
    deployment: null,
    deployedProduct: null,
    product: null,
    assignedEngineer: null,
    assignedTeam: null,
    startDate: "2026-06-10T04:30:00Z",
    endDate: "2026-06-10T06:30:00Z",
    createdOn: "2026-01-01",
    updatedOn: "2026-01-02",
    hasCustomerApproved: false,
    hasCustomerReviewed: false,
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/projects/p1/operations/change-requests/cr-1"]}>
      <Routes>
        <Route
          path="/projects/:projectId/operations/change-requests/:changeRequestId"
          element={<ChangeRequestDetailsPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

const button = (name: string) => screen.queryByRole("button", { name });

/** The window makeChangeRequest shows, as an answer names it (the schedule the customer saw). */
const SHOWN = { expectedPlannedStartOn: "2026-06-10T04:30:00Z", expectedPlannedEndOn: "2026-06-10T06:30:00Z" };

describe("ChangeRequestDetailsPage", () => {
  beforeEach(() => {
    mocks.changeRequest.value = makeChangeRequest();
    mocks.error.value = null;
    mocks.mutateAsync.mockReset();
    mocks.mutateAsync.mockResolvedValue({ id: "cr-1" });
    mocks.showError.mockReset();
    mocks.showSuccess.mockReset();
    mocks.isPending.value = false;
  });

  it("renders change request number when loaded", () => {
    renderPage();
    expect(screen.getByText("CHG001")).toBeInTheDocument();
  });

  describe("a change request in Authorize after the customer proposed a new time", () => {
    beforeEach(() => {
      mocks.changeRequest.value = makeChangeRequest({
        state: STATES.authorize,
        customerCanAnswer: false,
      });
    });

    it("is still there, says where it is, and tells the customer WSO2 is reviewing it", () => {
      renderPage();
      expect(screen.getByText("CHG001")).toBeInTheDocument();
      // The workflow panel marks Authorize as the current step (the same locator the
      // e2e page object uses: the stage name sits two levels above the marker).
      const current = screen.getAllByText("Current");
      expect(current).toHaveLength(1);
      expect(current[0].parentElement?.parentElement?.querySelector("p")?.textContent).toBe("Authorize");
      expect(
        screen.getByText(/WSO2 is reviewing this change request internally/),
      ).toBeInTheDocument();
    });

    it("offers nothing to answer until the customer is asked again", () => {
      renderPage();
      for (const name of ["Propose New Time", "Approve", "Reject", "Successful", "Unsuccessful"]) {
        expect(button(name), name).not.toBeInTheDocument();
      }
    });
  });

  describe("a change request that is not shared with the customer", () => {
    it("is a plain not-found page, whatever the reason, and shows none of the change request", () => {
      mocks.changeRequest.value = null;
      mocks.error.value = new ApiError(404, "Not Found", "change request not found");
      renderPage();
      expect(screen.getByText(CHANGE_REQUEST_NOT_FOUND_MESSAGE)).toBeInTheDocument();
      expect(screen.queryByText("CHG001")).not.toBeInTheDocument();
      for (const name of ["Propose New Time", "Approve", "Reject", "Successful", "Unsuccessful"]) {
        expect(button(name), name).not.toBeInTheDocument();
      }
    });

    it("keeps the generic copy for a failure that is not a 404", () => {
      mocks.changeRequest.value = null;
      mocks.error.value = new ApiError(500, "Internal Server Error", "boom");
      renderPage();
      expect(screen.getByText("Could not load change request details.")).toBeInTheDocument();
      expect(screen.queryByText(CHANGE_REQUEST_NOT_FOUND_MESSAGE)).not.toBeInTheDocument();
    });
  });

  describe("which buttons the customer gets", () => {
    type Case = {
      name: string;
      state: { id: string; label: string };
      customerCanAnswer?: boolean;
      hasCustomerApproved?: boolean;
      buttons: string[];
    };
    const approvalButtons = ["Propose New Time", "Approve", "Reject"];
    const reviewButtons = ["Successful", "Unsuccessful"];
    const cases: Case[] = [
      { name: "Customer Approval, customerCanAnswer true, stamp unset", state: STATES.approval, customerCanAnswer: true, hasCustomerApproved: false, buttons: approvalButtons },
      { name: "Customer Approval, customerCanAnswer false, stamp set", state: STATES.approval, customerCanAnswer: false, hasCustomerApproved: true, buttons: [] },
      { name: "Customer Approval, customerCanAnswer absent, stamp set (legacy gate)", state: STATES.approval, hasCustomerApproved: true, buttons: approvalButtons },
      { name: "Customer Approval, customerCanAnswer absent, stamp unset", state: STATES.approval, hasCustomerApproved: false, buttons: [] },
      { name: "Customer Review, customerCanAnswer true", state: STATES.review, customerCanAnswer: true, buttons: reviewButtons },
      { name: "Customer Review, customerCanAnswer absent", state: STATES.review, buttons: reviewButtons },
      { name: "Customer Review, customerCanAnswer false", state: STATES.review, customerCanAnswer: false, buttons: [] },
      { name: "Scheduled, customerCanAnswer true", state: STATES.scheduled, customerCanAnswer: true, hasCustomerApproved: true, buttons: [] },
      { name: "Authorize (after a proposal), customerCanAnswer true", state: STATES.authorize, customerCanAnswer: true, buttons: [] },
    ];
    const all = [...approvalButtons, ...reviewButtons];

    it.each(cases)("$name", ({ state, customerCanAnswer, hasCustomerApproved, buttons }) => {
      mocks.changeRequest.value = makeChangeRequest({
        state,
        ...(customerCanAnswer === undefined ? {} : { customerCanAnswer }),
        ...(hasCustomerApproved === undefined ? {} : { hasCustomerApproved }),
      });
      renderPage();
      for (const name of all) {
        if (buttons.includes(name)) expect(button(name), name).toBeInTheDocument();
        else expect(button(name), name).not.toBeInTheDocument();
      }
    });
  });

  describe("approving", () => {
    beforeEach(() => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
    });

    it("approves in one click, without a confirmation, and says what happened", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Approve" }));
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerApproved: true, ...SHOWN });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request approved. It is now scheduled.");
      expect(mocks.showError).not.toHaveBeenCalled();
    });

    it("sends only one request however often it is clicked while it is in flight", async () => {
      let resolve: (v: unknown) => void = () => {};
      mocks.mutateAsync.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
      renderPage();
      const approve = screen.getByRole("button", { name: "Approve" });
      fireEvent.click(approve);
      fireEvent.click(approve);
      fireEvent.click(approve);
      expect(mocks.mutateAsync).toHaveBeenCalledTimes(1);
      await act(async () => { resolve({ id: "cr-1" }); });
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
    });

    it("disables every answer button while a request is pending", () => {
      mocks.isPending.value = true;
      renderPage();
      for (const name of ["Propose New Time", "Approve", "Reject"]) {
        expect(screen.getByRole("button", { name })).toBeDisabled();
      }
    });

    it("names only the bounds the change request has", async () => {
      mocks.changeRequest.value = makeChangeRequest({
        state: STATES.approval,
        customerCanAnswer: true,
        startDate: "2026-06-10T04:30:00Z",
        endDate: undefined,
      });
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Approve" }));
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledWith({
        isCustomerApproved: true,
        expectedPlannedStartOn: "2026-06-10T04:30:00Z",
      });
    });

    it("says the schedule changed, not that the request was answered, when it moved under an open page", async () => {
      mocks.mutateAsync.mockRejectedValueOnce(
        new ApiError(
          409,
          "Conflict",
          "the planned implementation time of this change request changed after you opened it (it is now 2026-09-15T10:00:00Z to 2026-09-15T12:00:00Z); read it again before giving your answer",
          undefined,
          ChangeRequestErrorCode.SCHEDULE_CHANGED,
        ),
      );
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Approve" }));
      await waitFor(() => expect(mocks.showError).toHaveBeenCalledWith(CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE));
      expect(mocks.showSuccess).not.toHaveBeenCalled();
    });

    it("moves focus to the page heading once the answer is given", async () => {
      renderPage();
      const approve = screen.getByRole("button", { name: "Approve" });
      approve.focus();
      fireEvent.click(approve);
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      await waitFor(() =>
        expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Deploy patch" })),
      );
    });

    it.each([
      [new ApiError(409, "Conflict", "stale", undefined, ChangeRequestErrorCode.APPROVAL_NOT_PENDING), CHANGE_REQUEST_ANSWER_STALE_MESSAGE],
      // An older backend names nothing: the page does not claim "already answered".
      [new ApiError(409, "Conflict", "stale"), CHANGE_REQUEST_ACTION_FAILED_MESSAGE],
      [new ApiError(403, "Forbidden", "nope", undefined, ChangeRequestErrorCode.FORBIDDEN), CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE],
      [new ApiError(403, "Forbidden", "nope"), CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE],
      [new ApiError(500, "Internal Server Error", "Failed to update change request."), "Failed to update change request."],
      [new Error("Failed to fetch"), "Could not approve the change request. Please try again."],
    ])("shows a readable message when approving fails (%#)", async (error, message) => {
      mocks.mutateAsync.mockRejectedValueOnce(error);
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Approve" }));
      await waitFor(() => expect(mocks.showError).toHaveBeenCalledWith(message));
      expect(mocks.showSuccess).not.toHaveBeenCalled();
    });
  });

  describe("rejecting", () => {
    beforeEach(() => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
    });

    it("asks first, in plain words, and sends nothing until confirmed", () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      const dialog = screen.getByRole("dialog", { name: "Reject this change request?" });
      expect(within(dialog).getByText("Rejecting cancels this change request.")).toBeInTheDocument();
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });

    it("points at Propose New Time as the alternative while it is on", () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      const dialog = screen.getByRole("dialog", { name: "Reject this change request?" });
      expect(within(dialog).getByText(/use Propose New Time instead/)).toBeInTheDocument();
    });

    it("does not point at Propose New Time while WSO2 has the change on hold, and says why instead", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true, isOnHold: true });
      renderPage();
      // The button the dialog would point at is switched off...
      expect(screen.getByRole("button", { name: "Propose New Time" })).toBeDisabled();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      const dialog = screen.getByRole("dialog", { name: "Reject this change request?" });
      // ...so the dialog says what is true, and rejecting is still possible.
      expect(within(dialog).queryByText(/use Propose New Time instead/)).not.toBeInTheDocument();
      expect(
        within(dialog).getByText("A new time cannot be proposed right now because WSO2 has this change request on hold."),
      ).toBeInTheDocument();
      expect(within(dialog).getByRole("button", { name: "Reject change request" })).toBeEnabled();
    });

    it("follows the hold if it arrives while the dialog is open", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
      const view = renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      expect(screen.getByText(/use Propose New Time instead/)).toBeInTheDocument();

      // What the refetch brings back once WSO2 has put the change on hold.
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true, isOnHold: true });
      view.rerender(
        <MemoryRouter initialEntries={["/projects/p1/operations/change-requests/cr-1"]}>
          <Routes>
            <Route
              path="/projects/:projectId/operations/change-requests/:changeRequestId"
              element={<ChangeRequestDetailsPage />}
            />
          </Routes>
        </MemoryRouter>,
      );
      expect(screen.queryByText(/use Propose New Time instead/)).not.toBeInTheDocument();
      expect(screen.getByText(/WSO2 has this change request on hold/, { selector: "p" })).toBeInTheDocument();
    });

    it("never offers the Propose New Time hint to a review", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.review });
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Unsuccessful" }));
      expect(screen.queryByText(/Propose New Time/)).not.toBeInTheDocument();
      expect(screen.queryByText(/on hold/)).not.toBeInTheDocument();
    });

    it("sends nothing when the customer goes back", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.click(screen.getByRole("button", { name: "Go back" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });

    it("sends nothing when the customer presses Escape", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });

    it("rejects once confirmed, closes the dialog and says it was canceled", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.click(screen.getByRole("button", { name: "Reject change request" }));
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledTimes(1);
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerApproved: false, ...SHOWN });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request rejected. It has been canceled.");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    });

    it("closes the dialog and shows why when the rejection is refused", async () => {
      mocks.mutateAsync.mockRejectedValueOnce(
        new ApiError(409, "Conflict", "stale", undefined, ChangeRequestErrorCode.APPROVAL_NOT_PENDING),
      );
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      fireEvent.click(screen.getByRole("button", { name: "Reject change request" }));
      await waitFor(() => expect(mocks.showError).toHaveBeenCalledWith(CHANGE_REQUEST_ANSWER_STALE_MESSAGE));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(mocks.showSuccess).not.toHaveBeenCalled();
    });
  });

  describe("the review", () => {
    beforeEach(() => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.review });
    });

    it("confirms the change in one click", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Successful" }));
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerReviewed: true, ...SHOWN });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request marked as successful. It is now closed.");
    });

    it("asks before marking it unsuccessful, because that sends it into rollback", async () => {
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Unsuccessful" }));
      const dialog = screen.getByRole("dialog", { name: "Mark this change as unsuccessful?" });
      expect(
        within(dialog).getByText("Marking it unsuccessful sends the change into rollback."),
      ).toBeInTheDocument();
      expect(mocks.mutateAsync).not.toHaveBeenCalled();

      fireEvent.click(screen.getByRole("button", { name: "Mark unsuccessful" }));
      await waitFor(() => expect(mocks.showSuccess).toHaveBeenCalledTimes(1));
      expect(mocks.mutateAsync).toHaveBeenCalledWith({ isCustomerReviewed: false, ...SHOWN });
      expect(mocks.showSuccess).toHaveBeenCalledWith("Change request marked as unsuccessful. It is now in rollback.");
    });
  });

  describe("what the page says around the answer", () => {
    it("asks the review question and groups the two answers under it", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.review, customerCanAnswer: true });
      renderPage();
      const group = screen.getByRole("group", { name: "This change has been implemented. Was it successful?" });
      expect(within(group).getByRole("button", { name: "Successful" })).toBeInTheDocument();
      expect(within(group).getByRole("button", { name: "Unsuccessful" })).toBeInTheDocument();
    });

    it("names the group of answers at Customer Approval", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
      renderPage();
      const group = screen.getByRole("group", { name: "Answer this change request" });
      expect(within(group).getAllByRole("button")).toHaveLength(3);
      expect(screen.queryByText(/Was it successful/)).not.toBeInTheDocument();
    });

    it("says, for as long as the page is open, that WSO2 is reviewing a proposed time", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.authorize, customerCanAnswer: false });
      renderPage();
      const note = screen.getByRole("status");
      expect(note).toHaveTextContent("WSO2 is reviewing this change request internally");
      expect(note).toHaveTextContent("You will be asked to approve the schedule once it is confirmed");
    });

    it.each([
      ["Customer Approval", STATES.approval],
      ["Scheduled", STATES.scheduled],
      ["Customer Review", STATES.review],
    ])("does not say it in %s", (_name, state) => {
      mocks.changeRequest.value = makeChangeRequest({ state, customerCanAnswer: true });
      renderPage();
      expect(screen.queryByText(/reviewing this change request internally/)).not.toBeInTheDocument();
    });

    it("calls the window a plan until the change is scheduled", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
      const view = renderPage();
      expect(screen.getByText("Planned Maintenance Window")).toBeInTheDocument();
      view.unmount();
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.scheduled });
      renderPage();
      expect(screen.getByText("Scheduled Maintenance Window")).toBeInTheDocument();
    });
  });

  describe("proposing a new time", () => {
    it("opens the dialog from the header button", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
      renderPage();
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Propose New Time" }));
      expect(screen.getByRole("dialog", { name: "Propose New Implementation Time" })).toBeInTheDocument();
    });

    it("switches Propose New Time off, and says why, while WSO2 has the change on hold; answering stays possible", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true, isOnHold: true });
      renderPage();
      const propose = screen.getByRole("button", { name: "Propose New Time" });
      expect(propose).toBeDisabled();
      const note = screen.getByText(/WSO2 has this change request on hold/);
      expect(propose).toHaveAttribute("aria-describedby", note.id);
      expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled();
      expect(screen.getByRole("button", { name: "Reject" })).toBeEnabled();
      fireEvent.click(propose);
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it.each([[false], [undefined]])("keeps Propose New Time on when the hold flag is %s", (isOnHold) => {
      mocks.changeRequest.value = makeChangeRequest({
        state: STATES.approval,
        customerCanAnswer: true,
        ...(isOnHold === undefined ? {} : { isOnHold }),
      });
      renderPage();
      expect(screen.getByRole("button", { name: "Propose New Time" })).toBeEnabled();
      expect(screen.queryByText(/on hold/)).not.toBeInTheDocument();
    });

    it("closes the dialog by itself once the change request no longer waits on the customer", () => {
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.approval, customerCanAnswer: true });
      const view = renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Propose New Time" }));
      expect(screen.getByRole("dialog")).toBeInTheDocument();

      // What the refetch after a proposal brings back: Authorize.
      mocks.changeRequest.value = makeChangeRequest({ state: STATES.authorize, customerCanAnswer: false });
      view.rerender(
        <MemoryRouter initialEntries={["/projects/p1/operations/change-requests/cr-1"]}>
          <Routes>
            <Route
              path="/projects/:projectId/operations/change-requests/:changeRequestId"
              element={<ChangeRequestDetailsPage />}
            />
          </Routes>
        </MemoryRouter>,
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(button("Propose New Time")).not.toBeInTheDocument();
      expect(button("Approve")).not.toBeInTheDocument();
    });
  });
});
