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


import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ChangeRequestDetailsPage from "@features/operations/pages/ChangeRequestDetailsPage";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";
import {
  CHANGE_REQUEST_ACTION_FAILED_MESSAGE,
  CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
  CHANGE_REQUEST_NOT_FOUND_MESSAGE,
  CHANGE_REQUEST_ON_HOLD_MESSAGE,
  CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE,
  ChangeRequestErrorCode,
} from "@features/operations/utils/changeRequests";
import { ApiError } from "@utils/ApiError";

// Where focus goes after every way an answer attempt or an answer dialog can end,
// with the real error and success banners (the page's live regions) so that what
// is announced and where focus lands are asserted together. Only the data hooks
// are replaced.
//
// A refused answer closes the dialog and the answer buttons go (the page reads the
// change request again): without care, a keyboard or screen reader user lands on
// the document body. After every exit the focus is on something real, and the
// outcome is in an alert.

const mocks = vi.hoisted(() => ({
  changeRequest: { value: null as Record<string, unknown> | null },
  // What the detail query reports as its error (null = none). A re-read that fails keeps the data it had.
  error: { value: null as unknown },
  mutateAsync: vi.fn(),
  isPending: { value: false },
}));

vi.mock("@features/operations/api/useGetChangeRequestDetails", () => ({
  default: () => ({
    data: mocks.changeRequest.value,
    error: mocks.error.value,
    isLoading: false,
    isFetching: false,
    isError: false,
  }),
}));

vi.mock("@features/operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutateAsync: mocks.mutateAsync,
    isPending: mocks.isPending.value,
  }),
}));

const APPROVAL = { id: "5", label: "Customer Approval" };
const REVIEW = { id: "1", label: "Customer Review" };
const SCHEDULED = { id: "-2", label: "Scheduled" };

function makeChangeRequest(overrides: Record<string, unknown> = {}) {
  return {
    id: "cr-1",
    number: "CHG001",
    title: "Deploy patch",
    state: APPROVAL,
    customerCanAnswer: true,
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

function tree() {
  return (
    <ErrorBannerProvider>
      <SuccessBannerProvider>
        <MemoryRouter initialEntries={["/projects/p1/operations/change-requests/cr-1"]}>
          <Routes>
            <Route
              path="/projects/:projectId/operations/change-requests/:changeRequestId"
              element={<ChangeRequestDetailsPage />}
            />
          </Routes>
        </MemoryRouter>
      </SuccessBannerProvider>
    </ErrorBannerProvider>
  );
}

const heading = () => screen.getByRole("heading", { name: "Deploy patch" });
const button = (name: string) => screen.getByRole("button", { name });

/** Presses a button the way a person does: it takes focus, then the click. */
function press(name: string) {
  const target = button(name);
  target.focus();
  fireEvent.click(target);
}

/** The change request as the refetch brings it back once the answer is over. */
const answerIsOver = () =>
  (mocks.changeRequest.value = makeChangeRequest({ state: SCHEDULED, customerCanAnswer: false, hasCustomerApproved: true }));

/** The alert the outcome is announced in. */
const alertText = async (text: string) =>
  expect(await screen.findByRole("alert")).toHaveTextContent(text);

const refusals: Array<[string, ApiError, string]> = [
  [
    "the answer was already given",
    new ApiError(409, "Conflict", "any wording", undefined, ChangeRequestErrorCode.APPROVAL_NOT_PENDING),
    CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  ],
  [
    "the planned window changed after the page was opened",
    new ApiError(409, "Conflict", "any wording", undefined, ChangeRequestErrorCode.SCHEDULE_CHANGED),
    CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE,
  ],
  [
    "the caller was not asked",
    new ApiError(403, "Forbidden", "any wording", undefined, ChangeRequestErrorCode.NOT_ASKED),
    CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
  ],
  [
    "the caller may not answer",
    new ApiError(403, "Forbidden", "any wording", undefined, ChangeRequestErrorCode.FORBIDDEN),
    CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
  ],
  [
    "an older backend refuses with a 409 that names nothing",
    new ApiError(409, "Conflict", "any wording"),
    CHANGE_REQUEST_ACTION_FAILED_MESSAGE,
  ],
];

describe("ChangeRequestDetailsPage: where focus goes after an answer", () => {
  beforeEach(() => {
    mocks.changeRequest.value = makeChangeRequest();
    mocks.error.value = null;
    mocks.mutateAsync.mockReset();
    mocks.mutateAsync.mockResolvedValue({ id: "cr-1" });
    mocks.isPending.value = false;
  });

  describe("Approve (one click, no dialog)", () => {
    it("moves focus to the heading and announces it once the answer is given", async () => {
      mocks.mutateAsync.mockImplementationOnce(async () => answerIsOver());
      const view = render(tree());
      press("Approve");
      await alertText("Change request approved. It is now scheduled.");
      await waitFor(() => expect(document.activeElement).toBe(heading()));
      view.unmount();
    });

    it.each(refusals)("moves focus to the heading and announces why when %s", async (_name, error, message) => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        // The refusal makes the page read the change request again, and the
        // answer buttons go with what it brings back.
        answerIsOver();
        throw error;
      });
      render(tree());
      press("Approve");
      await alertText(message);
      await waitFor(() => expect(document.activeElement).toBe(heading()));
      expect(document.activeElement).not.toBe(document.body);
    });

    it("moves focus to the heading even before the refetch has taken the buttons away", async () => {
      // The refusal arrives; the page has not yet re-read the change request, so the
      // buttons are still there when focus is placed: it must not stay on one that
      // is about to vanish.
      mocks.mutateAsync.mockRejectedValueOnce(
        new ApiError(409, "Conflict", "any wording", undefined, ChangeRequestErrorCode.APPROVAL_NOT_PENDING),
      );
      render(tree());
      press("Approve");
      await alertText(CHANGE_REQUEST_ANSWER_STALE_MESSAGE);
      await waitFor(() => expect(document.activeElement).toBe(heading()));
      expect(button("Approve")).toBeInTheDocument();
    });

    it("puts focus back on the button that was used when the failure leaves the answer open", async () => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        (document.activeElement as HTMLElement | null)?.blur();
        throw new ApiError(500, "Internal Server Error", "Failed to update change request.");
      });
      render(tree());
      press("Approve");
      await alertText("Failed to update change request.");
      await waitFor(() => expect(document.activeElement).toBe(button("Approve")));
    });
  });

  describe("Reject (a confirmation dialog)", () => {
    const openReject = () => {
      press("Reject");
      return screen.getByRole("dialog", { name: "Reject this change request?" });
    };

    it("moves focus to the heading and announces it once the rejection is given", async () => {
      mocks.mutateAsync.mockImplementationOnce(async () => answerIsOver());
      render(tree());
      const dialog = openReject();
      fireEvent.click(within(dialog).getByRole("button", { name: "Reject change request" }));
      await alertText("Change request rejected. It has been canceled.");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(heading()));
    });

    it.each(refusals)("closes the dialog, moves focus to the heading and announces why when %s", async (_name, error, message) => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        answerIsOver();
        throw error;
      });
      render(tree());
      const dialog = openReject();
      fireEvent.click(within(dialog).getByRole("button", { name: "Reject change request" }));
      await alertText(message);
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(heading()));
      expect(document.activeElement).not.toBe(document.body);
    });

    it("returns focus to the Reject button when the failure leaves the answer open", async () => {
      mocks.mutateAsync.mockRejectedValueOnce(new ApiError(500, "Internal Server Error", "Failed to update change request."));
      render(tree());
      const dialog = openReject();
      fireEvent.click(within(dialog).getByRole("button", { name: "Reject change request" }));
      await alertText("Failed to update change request.");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(button("Reject")));
    });

    it("returns focus to the Reject button when the customer goes back, or presses Escape", async () => {
      render(tree());
      let dialog = openReject();
      fireEvent.click(within(dialog).getByRole("button", { name: "Go back" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(button("Reject")));

      dialog = openReject();
      fireEvent.keyDown(dialog, { key: "Escape" });
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(button("Reject")));
      expect(mocks.mutateAsync).not.toHaveBeenCalled();
    });
  });

  describe("the review's Unsuccessful (a confirmation dialog)", () => {
    it("moves focus to the heading when the answer is refused", async () => {
      mocks.changeRequest.value = makeChangeRequest({ state: REVIEW });
      mocks.mutateAsync.mockImplementationOnce(async () => {
        answerIsOver();
        throw new ApiError(409, "Conflict", "any wording", undefined, ChangeRequestErrorCode.APPROVAL_NOT_PENDING);
      });
      render(tree());
      press("Unsuccessful");
      fireEvent.click(screen.getByRole("button", { name: "Mark unsuccessful" }));
      await alertText(CHANGE_REQUEST_ANSWER_STALE_MESSAGE);
      await waitFor(() => expect(document.activeElement).toBe(heading()));
    });
  });

  describe("Propose New Time (a dialog with a form)", () => {
    const openPropose = () => {
      press("Propose New Time");
      return screen.getByRole("dialog", { name: "Propose New Implementation Time" });
    };
    const sendProposal = (dialog: HTMLElement) => {
      fireEvent.change(within(dialog).getByLabelText(/Proposed start/), { target: { value: "2099-06-11T15:30" } });
      fireEvent.click(within(dialog).getByRole("button", { name: "Submit Proposal" }));
    };

    it("returns focus to Propose New Time once the proposal is made: the change stays in Customer Approval with every answer on offer", async () => {
      // What the refetch brings back: the proposal waits for WSO2, and nothing else moved.
      mocks.mutateAsync.mockImplementationOnce(async () => {
        mocks.changeRequest.value = makeChangeRequest({
          customerProposal: { startDate: "2099-06-11T15:30:00Z", answer: "pending", proposedByViewer: true },
        });
      });
      render(tree());
      sendProposal(openPropose());
      await alertText("New time proposed. WSO2 will accept it or suggest a different time, and the answer will appear on this page.");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      // The buttons did not go (the customer's request stays live), so the heading is not the
      // place to land: the button that opened the dialog is.
      expect(button("Approve")).toBeEnabled();
      await waitFor(() => expect(document.activeElement).toBe(button("Propose New Time")));
    });

    it("falls back to the heading when a proposal ends the customer's question (the change moved on behind the page)", async () => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        answerIsOver();
      });
      render(tree());
      sendProposal(openPropose());
      await alertText("New time proposed. WSO2 will accept it or suggest a different time, and the answer will appear on this page.");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(heading()));
    });

    it.each(refusals)("closes the dialog, moves focus to the heading and announces why when %s", async (_name, error, message) => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        answerIsOver();
        throw error;
      });
      render(tree());
      sendProposal(openPropose());
      await alertText(message);
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(heading()));
      expect(document.activeElement).not.toBe(document.body);
    });

    it("keeps the dialog open on the hold, says so in its alert, and keeps focus on its Submit button", async () => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        (document.activeElement as HTMLElement | null)?.blur();
        throw new ApiError(409, "Conflict", "any wording", undefined, ChangeRequestErrorCode.ON_HOLD);
      });
      render(tree());
      const dialog = openPropose();
      within(dialog).getByRole("button", { name: "Submit Proposal" }).focus();
      sendProposal(dialog);
      expect(await within(dialog).findByText(CHANGE_REQUEST_ON_HOLD_MESSAGE)).toBeInTheDocument();
      expect(screen.getByRole("dialog", { name: "Propose New Implementation Time" })).toBeInTheDocument();
      await waitFor(() =>
        expect(document.activeElement).toBe(within(dialog).getByRole("button", { name: "Submit Proposal" })),
      );
    });

    it("returns focus to a usable Propose New Time button when the dialog is cancelled", async () => {
      render(tree());
      const dialog = openPropose();
      fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(button("Propose New Time")));
    });

    it("falls back to the heading when the hold has switched the button off by the time the dialog closes", async () => {
      mocks.mutateAsync.mockRejectedValueOnce(
        new ApiError(409, "Conflict", "any wording", undefined, ChangeRequestErrorCode.ON_HOLD),
      );
      const view = render(tree());
      const dialog = openPropose();
      sendProposal(dialog);
      await within(dialog).findByText(CHANGE_REQUEST_ON_HOLD_MESSAGE);

      // What the refetch brings back: the change request is on hold now, so the
      // button that opened the dialog is switched off.
      mocks.changeRequest.value = makeChangeRequest({ isOnHold: true });
      view.rerender(tree());
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Cancel" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(button("Propose New Time")).toBeDisabled();
      await waitFor(() => expect(document.activeElement).toBe(heading()));
    });
  });
  // A refused answer re-reads the change request. When that read fails too (the contact was
  // deregistered meanwhile: the change request is a 404 to them now) the page is replaced by the
  // error state, and the heading focus was moved to would be gone with it.
  describe("when the re-read after a refusal fails and the error state replaces the page", () => {
    const notFound = () => new ApiError(404, "Not Found", "The change request was not found.");
    const errorGroup = () => screen.getByRole("group", { name: CHANGE_REQUEST_NOT_FOUND_MESSAGE });

    it.each(refusals)("moves focus to the error state, not the document body, when %s", async (_name, error, message) => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        // The refusal invalidates the detail query; the refetch fails.
        mocks.error.value = notFound();
        throw error;
      });
      render(tree());
      press("Approve");
      await alertText(message);
      // The page, its heading and its buttons are gone: this is the error state.
      await waitFor(() => expect(screen.queryByRole("heading", { name: "Deploy patch" })).not.toBeInTheDocument());
      expect(screen.queryByRole("button", { name: "Approve" })).not.toBeInTheDocument();
      await waitFor(() => expect(document.activeElement).toBe(errorGroup()));
      expect(document.activeElement).not.toBe(document.body);
    });

    it("does the same after a refused rejection, whose dialog had focus", async () => {
      mocks.mutateAsync.mockImplementationOnce(async () => {
        mocks.error.value = notFound();
        throw new ApiError(403, "Forbidden", "any wording", undefined, ChangeRequestErrorCode.NOT_ASKED);
      });
      render(tree());
      press("Reject");
      const dialog = screen.getByRole("dialog", { name: "Reject this change request?" });
      fireEvent.click(within(dialog).getByRole("button", { name: "Reject change request" }));
      await alertText(CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE);
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(document.activeElement).toBe(errorGroup()));
    });

    it("names the error state after what happened, and keeps the error page inside it", async () => {
      mocks.error.value = notFound();
      render(tree());
      expect(errorGroup()).toHaveAttribute("tabindex", "-1");
      expect(errorGroup()).toHaveTextContent(CHANGE_REQUEST_NOT_FOUND_MESSAGE);
      // Any other failure of the read is named for what it is.
      cleanup();
      mocks.error.value = new ApiError(500, "Internal Server Error", "boom");
      render(tree());
      expect(screen.getByRole("group", { name: "Could not load change request details." })).toHaveTextContent(
        "Could not load change request details.",
      );
    });

    it("never takes focus from a control the customer has moved to", async () => {
      const elsewhere = document.createElement("button");
      elsewhere.textContent = "Elsewhere";
      document.body.appendChild(elsewhere);
      try {
        elsewhere.focus();
        mocks.error.value = notFound();
        render(tree());
        expect(errorGroup()).toBeInTheDocument();
        expect(document.activeElement).toBe(elsewhere);
      } finally {
        elsewhere.remove();
      }
    });
  });
});
