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

import { render, screen, fireEvent } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CaseDetailsActionRow from "@case-details/CaseDetailsActionRow";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: {
      caseStates: [
        { id: "1", label: "Open" },
        { id: "3", label: "Closed" },
        { id: "1003", label: "Waiting On WSO2" },
      ],
    },
  }),
}));

const mutateMock = vi.fn();
const postCommentMutateMock = vi.fn();

vi.mock("@features/support/api/usePatchCase", () => ({
  usePatchCase: () => ({ mutate: mutateMock, isPending: false }),
}));

vi.mock("@features/support/api/usePostComment", () => ({
  usePostComment: () => ({ mutate: postCommentMutateMock, isPending: false }),
}));

const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: showSuccessMock }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));

// Defaults to an external (customer) caller, the normal customer portal user;
// individual tests override this where internal behaviour matters.
const useCustomerPermissionsMock = vi.fn(
  (): {
    isExternalUser: boolean;
    isInternalUser?: boolean;
    canCreateCase: boolean;
    canUpdateCase: boolean;
  } => ({
    isExternalUser: true,
    canCreateCase: true,
    canUpdateCase: true,
  }),
);

// The row's hooks log through useLogger, which needs a LoggerProvider this
// test does not set up.
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock("@hooks/useCustomerPermissions", () => ({
  useCustomerPermissions: () => useCustomerPermissionsMock(),
}));

function renderActionRow(
  props: Partial<Parameters<typeof CaseDetailsActionRow>[0]> = {},
) {
  // The row's child dialogs read the query cache and the router.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ThemeProvider theme={createTheme()}>
          <CaseDetailsActionRow
            assignedEngineer="Jane Doe"
            engineerInitials="JD"
            statusLabel="Open"
            projectId="proj-1"
            caseId="case-1"
            {...props}
          />
        </ThemeProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CaseDetailsActionRow", () => {
  beforeEach(() => {
    mutateMock.mockReset();
    postCommentMutateMock.mockReset();
    showSuccessMock.mockReset();
    showErrorMock.mockReset();
    useCustomerPermissionsMock.mockReset();
    useCustomerPermissionsMock.mockReturnValue({
      isExternalUser: true,
      canCreateCase: true,
      canUpdateCase: true,
    });
  });

  it("should render Close action for open status when case can be patched", () => {
    renderActionRow({ statusLabel: "Open" });
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("closes with a plain confirm and sends no resolution fields for an external user", () => {
    useCustomerPermissionsMock.mockReturnValue({
      isExternalUser: true,
      canCreateCase: true,
      canUpdateCase: true,
    });
    renderActionRow({ statusLabel: "Open" });

    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    expect(screen.queryByLabelText(/Resolution Code/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/^Cause/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Close notes")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0]![0]).toEqual({ stateKey: 3 });
  });

  it("accepts a solution with a plain confirm for an external user", () => {
    renderActionRow({ statusLabel: "Solution Proposed" });

    fireEvent.click(screen.getByRole("button", { name: "Accept Solution" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0]![0]).toEqual({ stateKey: 3 });
  });

  describe("internal users", () => {
    beforeEach(() => {
      useCustomerPermissionsMock.mockReturnValue({
        isExternalUser: false,
        isInternalUser: true,
        canCreateCase: true,
        canUpdateCase: true,
      });
    });

    it("offers Wait on WSO2 on an Awaiting Info case", () => {
      renderActionRow({ statusLabel: "Awaiting Info" });

      expect(
        screen.getByRole("button", { name: "Wait on WSO2" }),
      ).toBeInTheDocument();
    });

    it("disables Close and cannot open the confirm dialog", () => {
      renderActionRow({ statusLabel: "Open" });

      const close = screen.getByRole("button", { name: "Close" });
      expect(close).toBeDisabled();
      fireEvent.click(close);

      expect(screen.queryByText("Confirm State Change")).not.toBeInTheDocument();
      expect(mutateMock).not.toHaveBeenCalled();
    });

    it("disables Accept Solution but leaves Reject Solution usable", () => {
      renderActionRow({ statusLabel: "Solution Proposed" });

      const accept = screen.getByRole("button", { name: "Accept Solution" });
      expect(accept).toBeDisabled();
      fireEvent.click(accept);

      expect(screen.queryByText("Confirm State Change")).not.toBeInTheDocument();
      expect(mutateMock).not.toHaveBeenCalled();
      expect(
        screen.getByRole("button", { name: "Reject Solution" }),
      ).toBeEnabled();
    });

    it("explains why Close is unavailable via a tooltip", async () => {
      renderActionRow({ statusLabel: "Open" });

      fireEvent.mouseOver(screen.getByRole("button", { name: "Close" }).parentElement!);

      expect(
        await screen.findByText(
          "Cases can't be closed from the customer portal for internal users. Use the CSM portal.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("never offers Wait on WSO2 to an external user on an Awaiting Info case", () => {
    renderActionRow({ statusLabel: "Awaiting Info" });

    expect(
      screen.queryByRole("button", { name: "Wait on WSO2" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("hides Wait on WSO2 until the user is known to be internal", () => {
    useCustomerPermissionsMock.mockReturnValue({
      isExternalUser: false,
      isInternalUser: false,
      canCreateCase: true,
      canUpdateCase: true,
    });
    renderActionRow({ statusLabel: "Awaiting Info" });

    expect(
      screen.queryByRole("button", { name: "Wait on WSO2" }),
    ).not.toBeInTheDocument();
  });

  it("should render solution actions for solution proposed status", () => {
    renderActionRow({ statusLabel: "Solution Proposed" });
    expect(
      screen.getByRole("button", { name: "Accept Solution" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Reject Solution" }),
    ).toBeInTheDocument();
  });

  it("should render Open Related Case for recently closed cases", () => {
    renderActionRow({
      statusLabel: "Closed",
      closedOn: "2026-05-15 10:00:00",
      onOpenRelatedCase: vi.fn(),
    });
    expect(
      screen.getByRole("button", { name: "Open Related Case" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  });

  it("should hide Open Related Case when closed more than 2 months ago", () => {
    renderActionRow({
      statusLabel: "Closed",
      closedOn: "2020-01-01 10:00:00",
      onOpenRelatedCase: vi.fn(),
    });
    expect(
      screen.queryByRole("button", { name: "Open Related Case" }),
    ).not.toBeInTheDocument();
  });

  it("should call onOpenRelatedCase when Open Related Case is clicked", () => {
    const onOpenRelatedCase = vi.fn();
    renderActionRow({
      statusLabel: "Closed",
      closedOn: "2026-05-15 10:00:00",
      onOpenRelatedCase,
    });
    fireEvent.click(screen.getByRole("button", { name: "Open Related Case" }));
    expect(onOpenRelatedCase).toHaveBeenCalledTimes(1);
  });

  describe("Reject Solution", () => {
    it("opens the reject dialog instead of the bare confirm dialog", () => {
      renderActionRow({ statusLabel: "Solution Proposed" });
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));

      expect(
        screen.getByLabelText("Reason for rejecting the solution"),
      ).toBeInTheDocument();
      expect(postCommentMutateMock).not.toHaveBeenCalled();
      expect(mutateMock).not.toHaveBeenCalled();
    });

    it("aborts with no comment and no state change when Cancel is clicked", () => {
      renderActionRow({ statusLabel: "Solution Proposed" });
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

      expect(postCommentMutateMock).not.toHaveBeenCalled();
      expect(mutateMock).not.toHaveBeenCalled();
    });

    it("posts the comment before flipping state, in that order, when a reason is given", () => {
      postCommentMutateMock.mockImplementation((_vars, options) => {
        options?.onSuccess?.();
      });
      mutateMock.mockImplementation((_body, options) => {
        options?.onSuccess?.();
        options?.onSettled?.();
      });

      renderActionRow({ statusLabel: "Solution Proposed" });
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));
      fireEvent.change(
        screen.getByLabelText("Reason for rejecting the solution"),
        { target: { value: "Still failing" } },
      );
      // The row's own action buttons are aria-hidden while the modal is open,
      // so only the dialog's confirm button matches here.
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));

      expect(postCommentMutateMock).toHaveBeenCalledTimes(1);
      expect(postCommentMutateMock.mock.calls[0]![0]).toEqual({
        caseId: "case-1",
        body: {
          content:
            "Proposed solution was rejected with following feedback:<br>Still failing",
          type: "comments",
        },
      });
      expect(mutateMock).toHaveBeenCalledTimes(1);
      expect(mutateMock.mock.calls[0]![0]).toEqual({ stateKey: 1003 });
    });

    it("escapes HTML in the reason before it is spliced into the comment", () => {
      postCommentMutateMock.mockImplementation((_vars, options) => {
        options?.onSuccess?.();
      });
      mutateMock.mockImplementation((_body, options) => {
        options?.onSuccess?.();
        options?.onSettled?.();
      });

      renderActionRow({ statusLabel: "Solution Proposed" });
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));
      fireEvent.change(
        screen.getByLabelText("Reason for rejecting the solution"),
        { target: { value: "<b>still</b> broken & unusable" } },
      );
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));

      expect(postCommentMutateMock.mock.calls[0]![0].body.content).toBe(
        "Proposed solution was rejected with following feedback:<br>" +
          "&lt;b&gt;still&lt;/b&gt; broken &amp; unusable",
      );
    });

    it("still posts a comment (no content omission) when the reason is left blank", () => {
      postCommentMutateMock.mockImplementation((_vars, options) => {
        options?.onSuccess?.();
      });
      mutateMock.mockImplementation((_body, options) => {
        options?.onSuccess?.();
        options?.onSettled?.();
      });

      renderActionRow({ statusLabel: "Solution Proposed" });
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));

      expect(postCommentMutateMock.mock.calls[0]![0]).toEqual({
        caseId: "case-1",
        body: {
          content:
            "Proposed solution was rejected. No additional feedback was provided.",
          type: "comments",
        },
      });
      expect(mutateMock.mock.calls[0]![0]).toEqual({ stateKey: 1003 });
    });

    it("never attempts the state PATCH when posting the comment fails", () => {
      postCommentMutateMock.mockImplementation((_vars, options) => {
        options?.onError?.(new Error("comment failed"));
      });

      renderActionRow({ statusLabel: "Solution Proposed" });
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));
      fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));

      expect(postCommentMutateMock).toHaveBeenCalledTimes(1);
      expect(mutateMock).not.toHaveBeenCalled();
      expect(showErrorMock).toHaveBeenCalledWith("comment failed");
      // Dialog stays open so the customer doesn't lose their typed reason.
      expect(
        screen.getByLabelText("Reason for rejecting the solution"),
      ).toBeInTheDocument();
    });
  });

  describe("read-only (isReadOnly INTERNAL caller)", () => {
    beforeEach(() => {
      useCustomerPermissionsMock.mockReturnValue({
        isExternalUser: false,
        canCreateCase: false,
        canUpdateCase: false,
      });
    });

    it("renders no state-change actions", () => {
      renderActionRow({ statusLabel: "Open" });
      expect(
        screen.queryByRole("button", { name: "Close" }),
      ).not.toBeInTheDocument();
    });

    it("never shows the Escalate button even when otherwise eligible", () => {
      renderActionRow({
        statusLabel: "Open",
        escalationLevelId: "0",
        isCurrentUserLead: true,
      });
      expect(
        screen.queryByRole("button", { name: "Escalate Case" }),
      ).not.toBeInTheDocument();
    });

    it("never shows the De-escalate button even when escalated", () => {
      renderActionRow({ statusLabel: "Open", isEscalated: true, canDeescalate: true });
      expect(
        screen.queryByRole("button", { name: "De-escalate Case" }),
      ).not.toBeInTheDocument();
    });
  });
});
