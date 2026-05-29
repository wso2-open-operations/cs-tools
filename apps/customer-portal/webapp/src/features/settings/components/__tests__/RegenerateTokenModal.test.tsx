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
import RegenerateTokenModal from "@features/settings/components/RegenerateTokenModal";

const mutateMock = vi.fn();

vi.mock("@features/settings/api/useRegenerateRegistryToken", () => ({
  useRegenerateRegistryToken: () => ({
    mutate: mutateMock,
    reset: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
}));

describe("RegenerateTokenModal", () => {
  it("triggers regenerate action", () => {
    render(
      <RegenerateTokenModal
        open
        onClose={vi.fn()}
        projectId="p-1"
        token={{ id: 4, name: "tok" } as never}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /regenerate secret/i }));
    expect(mutateMock).toHaveBeenCalledWith(4, expect.any(Object));
  });
});

