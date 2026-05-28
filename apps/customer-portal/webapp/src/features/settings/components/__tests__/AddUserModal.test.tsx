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
import AddUserModal from "@features/settings/components/AddUserModal";

const mutateMock = vi.fn();

vi.mock("@features/settings/api/useValidateProjectContact", () => ({
  useValidateProjectContact: () => ({
    mutate: mutateMock,
    reset: vi.fn(),
    isPending: false,
  }),
}));

describe("AddUserModal", () => {
  it("runs email validation on next click", () => {
    render(
      <AddUserModal
        open
        projectId="p-1"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );
    fireEvent.change(screen.getByLabelText("Email Address *"), {
      target: { value: "user@test.dev" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(mutateMock).toHaveBeenCalledWith(
      { contactEmail: "user@test.dev" },
      expect.any(Object),
    );
  });
});

