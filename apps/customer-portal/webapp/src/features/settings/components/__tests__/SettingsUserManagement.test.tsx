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

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SettingsUserManagement from "@features/settings/components/SettingsUserManagement";

vi.mock("@features/settings/api/useGetProjectContacts", () => ({
  default: () => ({
    data: [{ id: "1", email: "user@test.dev", membershipStatus: "Active" }],
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
vi.mock("@features/settings/api/usePatchProjectContact", () => ({
  usePatchProjectContact: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
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
});
