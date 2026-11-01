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
import EditUserModal from "@features/settings/components/EditUserModal";

describe("EditUserModal", () => {
  it("submits selected role changes", () => {
    const onSubmit = vi.fn();
    render(
      <EditUserModal
        open
        contact={{
          id: "1",
          email: "u@test.dev",
          firstName: "U",
          lastName: "One",
          isCsIntegrationUser: false,
          isCsAdmin: false,
          isPortalUser: true,
          isSecurityContact: false,
        } as never}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.click(screen.getByText("Admin"));
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ isCsAdmin: true }),
    );
  });

  describe("admin role initial state", () => {
    const contact = {
      id: "1",
      email: "jane.doe@example.com",
      firstName: "Jane",
      lastName: "Doe",
      isCsIntegrationUser: false,
      isPortalUser: true,
      isSecurityContact: false,
    };

    it("does not pre-tick admin for an account-level admin and submits isCsAdmin false", () => {
      const onSubmit = vi.fn();
      render(
        <EditUserModal
          open
          contact={{ ...contact, isCsAdmin: false, isAccountAdmin: true } as never}
          onClose={vi.fn()}
          onSubmit={onSubmit}
        />,
      );
      expect(screen.getByRole("checkbox", { name: /admin/i })).not.toBeChecked();
      // Make the form dirty with an unrelated role so Save is enabled.
      fireEvent.click(screen.getByText("Security Contact"));
      fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
      expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ isCsAdmin: false }));
    });

    it("pre-ticks admin for a project-level admin", () => {
      render(
        <EditUserModal
          open
          contact={{ ...contact, isCsAdmin: true } as never}
          onClose={vi.fn()}
          onSubmit={vi.fn()}
        />,
      );
      expect(screen.getByRole("checkbox", { name: /admin/i })).toBeChecked();
    });
  });
});

