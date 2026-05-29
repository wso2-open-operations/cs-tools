// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import DeleteTokenModal from "@features/settings/components/DeleteTokenModal";

const mutateMock = vi.fn();
const resetMock = vi.fn();

vi.mock("@features/settings/api/useDeleteRegistryToken", () => ({
  useDeleteRegistryToken: () => ({
    mutate: mutateMock,
    reset: resetMock,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

describe("DeleteTokenModal", () => {
  it("renders token name and triggers delete mutation", () => {
    render(
      <DeleteTokenModal
        open
        onClose={vi.fn()}
        projectId="p-1"
        token={{ id: 9, name: "token-1" } as never}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(mutateMock).toHaveBeenCalledWith(9, expect.any(Object));
  });
});

