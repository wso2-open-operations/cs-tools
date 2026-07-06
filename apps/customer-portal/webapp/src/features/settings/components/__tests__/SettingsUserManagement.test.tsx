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
import SettingsUserManagement from "@features/settings/components/SettingsUserManagement";

vi.mock("@features/settings/api/useGetProjectContacts", () => ({
  default: () => ({
    data: [{ id: "1", email: "user@test.dev", membershipStatus: "Active" }],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@features/settings/api/usePostProjectContact", () => ({
  usePostProjectContact: () => ({ mutate: vi.fn(), isPending: false }),
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
  default: ({ open }: { open: boolean }) => (open ? <div>add-user-modal</div> : null),
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
});

