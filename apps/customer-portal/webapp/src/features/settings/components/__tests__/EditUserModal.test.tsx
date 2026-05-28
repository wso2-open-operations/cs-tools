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
});

