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
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import SettingsUserManagementComponent from "@features/settings/components/SettingsUserManagement";
import { resetPendingInvitesForTests } from "@features/settings/hooks/usePendingInvites";

// The page reads through the query client, so every render needs one.
function SettingsUserManagement(props: { projectId: string }) {
  return (
    <QueryClientProvider client={new QueryClient()}>
      <SettingsUserManagementComponent {...props} />
    </QueryClientProvider>
  );
}

const DEFAULT_CONTACTS = [{ id: "1", email: "user@test.dev", membershipStatus: "Active" }];
// The contact list each test sees; tests replace it before rendering.
const contactsState = vi.hoisted(() => ({ data: [] as unknown[] }));
const resendMutate = vi.hoisted(() => vi.fn());
const { showSuccess, showError } = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }));

vi.mock("@features/settings/api/useGetProjectContacts", () => ({
  default: () => ({
    data: contactsState.data,
    isLoading: false,
    error: null,
    refetch: vi.fn().mockResolvedValue({ data: [] }),
  }),
}));

const mutateAsync = vi.fn();
vi.mock("@features/settings/api/usePostProjectContact", () => ({
  usePostProjectContact: () => ({ mutate: vi.fn(), mutateAsync, isPending: false }),
}));
vi.mock("@features/settings/api/useDeleteProjectContact", () => ({
  useDeleteProjectContact: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/settings/api/useResendProjectContactInvitation", () => ({
  useResendProjectContactInvitation: () => ({ mutateAsync: resendMutate, isPending: false }),
}));
vi.mock("@features/settings/api/usePatchProjectContact", () => ({
  usePatchProjectContact: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess }),
}));
vi.mock("@features/settings/components/AddUserModal", () => ({
  default: ({ open, onSubmit }: { open: boolean; onSubmit: (r: unknown) => void }) =>
    open ? (
      <div>
        add-user-modal
        <button
          type="button"
          onClick={() =>
            onSubmit({
              contactEmail: "new@acme.com",
              contactFirstName: "New",
              contactLastName: "Person",
              isCsAdmin: false,
              isCsIntegrationUser: false,
              isLead: false,
              isPortalUser: true,
              isSecurityContact: false,
            })
          }
        >
          submit-invite
        </button>
        <button
          type="button"
          onClick={() =>
            onSubmit({
              contactEmail: "jane.doe@example.com",
              contactFirstName: "Jane",
              contactLastName: "Doe",
              isCsAdmin: false,
              isCsIntegrationUser: true,
              isLead: false,
              isPortalUser: false,
              isSecurityContact: false,
            })
          }
        >
          submit-system-user
        </button>
      </div>
    ) : null,
}));
vi.mock("@features/settings/components/EditUserModal", () => ({
  default: () => null,
}));
vi.mock("@features/settings/components/RemoveUserModal", () => ({
  default: () => null,
}));

describe("SettingsUserManagement", () => {
  beforeEach(() => {
    resendMutate.mockReset();
    showSuccess.mockClear();
    showError.mockClear();
    resetPendingInvitesForTests();
    contactsState.data = DEFAULT_CONTACTS;
  });

  it("renders contacts and opens add-user modal", () => {
    render(<SettingsUserManagement projectId="p-1" />);
    expect(screen.getByText("user@test.dev")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /add user/i }));
    expect(screen.getByText("add-user-modal")).toBeInTheDocument();
  });

  it("closes the dialog at once and shows the invitation as a pending row", async () => {
    mutateAsync.mockReturnValue(new Promise(() => {}));
    render(<SettingsUserManagement projectId="p-1" />);

    fireEvent.click(screen.getByRole("button", { name: /add user/i }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "submit-invite" }));
    });

    expect(screen.queryByText("add-user-modal")).not.toBeInTheDocument();
    const row = screen.getByTestId("pending-invite-new@acme.com");
    expect(within(row).getByText("New Person")).toBeInTheDocument();
    expect(within(row).getByText("Inviting…")).toBeInTheDocument();
    expect(screen.getByText("user@test.dev")).toBeInTheDocument();
  });

  it("uses invitation wording for portal users", async () => {
    mutateAsync.mockResolvedValueOnce("created");
    render(<SettingsUserManagement projectId="p-1" />);

    fireEvent.click(screen.getByRole("button", { name: /add user/i }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "submit-invite" }));
    });

    expect(showSuccess).toHaveBeenCalledWith("Inviting new@acme.com. This takes a few seconds.");
    expect(showSuccess).toHaveBeenCalledWith("Invitation sent successfully: new@acme.com");
  });

  it("uses add wording, not invitation wording, for system users", async () => {
    mutateAsync.mockResolvedValueOnce("created");
    render(<SettingsUserManagement projectId="p-1" />);

    fireEvent.click(screen.getByRole("button", { name: /add user/i }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "submit-system-user" }));
    });

    expect(showSuccess).toHaveBeenCalledWith("Adding jane.doe@example.com. This takes a few seconds.");
    expect(showSuccess).toHaveBeenCalledWith("User added successfully: jane.doe@example.com");
    for (const [msg] of showSuccess.mock.calls) {
      expect(String(msg)).not.toMatch(/invit/i);
    }
  });

  it("hides resend for an integration user even when the backend flags it resendable", () => {
    contactsState.data = [
      {
        id: "c-1",
        email: "jane.doe@example.com",
        membershipStatus: "INVITED",
        isCsIntegrationUser: true,
        canResendInvitation: true,
      },
    ];
    render(<SettingsUserManagement projectId="p-1" />);
    expect(screen.getByText("jane.doe@example.com")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Resend invitation" })).not.toBeInTheDocument();
  });

  it("shows the failure reason on the row with a retry action", async () => {
    mutateAsync.mockRejectedValueOnce(new Error("This address is already a contact on the project"));
    render(<SettingsUserManagement projectId="p-1" />);

    fireEvent.click(screen.getByRole("button", { name: /add user/i }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "submit-invite" }));
    });

    const row = screen.getByTestId("pending-invite-new@acme.com");
    expect(within(row).getByText("Failed")).toBeInTheDocument();
    expect(
      within(row).getByText("This address is already a contact on the project"),
    ).toBeInTheDocument();

    mutateAsync.mockReturnValueOnce(new Promise(() => {}));
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: "Retry invitation" }));
    });
    expect(within(screen.getByTestId("pending-invite-new@acme.com")).getByText("Inviting…")).toBeInTheDocument();
  });

  it("offers a resend only on rows the backend marks as resendable, behind a confirmation", async () => {
    contactsState.data = [
      { id: "c-1", email: "invited@acme.com", firstName: "Ina", lastName: "Vite", membershipStatus: "INVITED", canResendInvitation: true },
      { id: "c-2", email: "registered@acme.com", membershipStatus: "REGISTERED" },
      // A pre-cutover row never carries the flag, even while INVITED.
      { id: "c-3", email: "legacy@acme.com", membershipStatus: "INVITED" },
    ];
    resendMutate.mockResolvedValueOnce(undefined);
    render(<SettingsUserManagement projectId="p-1" />);

    const buttons = screen.getAllByRole("button", { name: "Resend invitation" });
    expect(buttons).toHaveLength(1);

    // The row button only asks; nothing is sent yet.
    fireEvent.click(buttons[0]);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Resend Invitation")).toBeInTheDocument();
    expect(within(dialog).getByText("Ina Vite")).toBeInTheDocument();
    expect(dialog).toHaveTextContent("(invited@acme.com)");
    expect(resendMutate).not.toHaveBeenCalled();

    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: "Yes, Resend" }));
    });
    expect(resendMutate).toHaveBeenCalledWith("invited@acme.com");
    expect(showSuccess).toHaveBeenCalledWith("Invitation resent to invited@acme.com");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("sends nothing when the resend confirmation is cancelled", async () => {
    contactsState.data = [
      { id: "c-1", email: "invited@acme.com", membershipStatus: "INVITED", canResendInvitation: true },
    ];
    render(<SettingsUserManagement projectId="p-1" />);

    fireEvent.click(screen.getByRole("button", { name: "Resend invitation" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(resendMutate).not.toHaveBeenCalled();
  });

  it("keeps the confirmation busy until the resend finishes, then reports its outcome", async () => {
    contactsState.data = [
      { id: "c-1", email: "a@acme.com", membershipStatus: "INVITED", canResendInvitation: true },
    ];
    let failA: (e: Error) => void = () => {};
    resendMutate.mockReturnValueOnce(new Promise<void>((_, reject) => { failA = reject; }));
    render(<SettingsUserManagement projectId="p-1" />);

    fireEvent.click(screen.getByRole("button", { name: "Resend invitation" }));
    const dialog = screen.getByRole("dialog");
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: "Yes, Resend" }));
    });
    expect(within(dialog).getByRole("button", { name: /Resending/ })).toBeDisabled();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
    // The row's own button stays disabled while its resend runs (the page
    // behind the modal is aria-hidden, hence hidden: true).
    expect(screen.getByRole("button", { name: "Resend invitation", hidden: true })).toBeDisabled();

    await act(async () => {
      failA(new Error("An invitation was sent a few minutes ago. Please try again later."));
    });
    expect(showError).toHaveBeenCalledWith("An invitation was sent a few minutes ago. Please try again later.");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Resend invitation" })).not.toBeDisabled();
  });

  describe("pagination", () => {
    const manyContacts = (n: number) =>
      Array.from({ length: n }, (_, i) => ({
        id: `c-${i + 1}`,
        email: `person${String(i + 1).padStart(2, "0")}@acme.com`,
        membershipStatus: "REGISTERED",
      }));

    it("shows ten contacts per page and the rest on the next page", () => {
      contactsState.data = manyContacts(13);
      render(<SettingsUserManagement projectId="p-1" />);

      expect(screen.getByText("person01@acme.com")).toBeInTheDocument();
      expect(screen.getByText("person10@acme.com")).toBeInTheDocument();
      expect(screen.queryByText("person11@acme.com")).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: /next page/i }));

      expect(screen.getByText("person11@acme.com")).toBeInTheDocument();
      expect(screen.getByText("person13@acme.com")).toBeInTheDocument();
      expect(screen.queryByText("person01@acme.com")).not.toBeInTheDocument();
    });

    it("keeps the rows-per-page selector visible (but the next-page button disabled) when every contact fits on one page", () => {
      contactsState.data = manyContacts(10);
      render(<SettingsUserManagement projectId="p-1" />);

      // ListPagination used to return null entirely once everything fit on
      // one page, which also hid its rows-per-page selector with no way to
      // change it back -- the same bug reported for the Announcements page.
      // The control must stay visible; only the next-page button should be
      // disabled rather than removed.
      expect(screen.getByLabelText(/rows per page/i)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /next page/i })).toBeDisabled();
    });

    it("searches the whole list and goes back to the first page", () => {
      contactsState.data = manyContacts(13);
      render(<SettingsUserManagement projectId="p-1" />);
      fireEvent.click(screen.getByRole("button", { name: /next page/i }));

      fireEvent.change(screen.getByPlaceholderText(/search/i), { target: { value: "person03" } });

      // person03 lives on page 1; it is found from page 2, and shown.
      expect(screen.getByText("person03@acme.com")).toBeInTheDocument();
      expect(screen.queryByText("person11@acme.com")).not.toBeInTheDocument();
    });

    it("moves back to the last page that still exists when the list shrinks", () => {
      contactsState.data = manyContacts(13);
      const view = render(<SettingsUserManagement projectId="p-1" />);
      fireEvent.click(screen.getByRole("button", { name: /next page/i }));
      expect(screen.getByText("person11@acme.com")).toBeInTheDocument();

      contactsState.data = manyContacts(8);
      view.rerender(<SettingsUserManagement projectId="p-1" />);

      expect(screen.getByText("person01@acme.com")).toBeInTheDocument();
      expect(screen.getByText("person08@acme.com")).toBeInTheDocument();
    });

    it("keeps pending invitations above the contacts on every page, outside the page size", async () => {
      contactsState.data = manyContacts(13);
      mutateAsync.mockReturnValue(new Promise(() => {}));
      render(<SettingsUserManagement projectId="p-1" />);

      fireEvent.click(screen.getByRole("button", { name: /add user/i }));
      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "submit-invite" }));
      });

      const rows = screen.getAllByRole("row");
      expect(within(rows[1]).getByText("new@acme.com")).toBeInTheDocument();
      expect(screen.getByText("person10@acme.com")).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: /next page/i }));
      expect(screen.getByTestId("pending-invite-new@acme.com")).toBeInTheDocument();
      expect(screen.getByText("person13@acme.com")).toBeInTheDocument();
    });
  });
});
