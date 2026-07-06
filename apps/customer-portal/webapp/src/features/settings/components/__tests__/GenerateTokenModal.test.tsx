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
import GenerateTokenModal from "@features/settings/components/GenerateTokenModal";
import { RegistryTokenType } from "@features/settings/types/registryTokens";

const mutateMock = vi.fn();

vi.mock("@features/settings/api/useCreateRegistryToken", () => ({
  useCreateRegistryToken: () => ({
    mutate: mutateMock,
    reset: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
}));

vi.mock("@features/settings/api/useGetIntegrationUsers", () => ({
  useGetIntegrationUsers: () => ({
    data: [{ id: "i-1", email: "bot@test.dev" }],
    isLoading: false,
  }),
}));

describe("GenerateTokenModal", () => {
  it("validates then submits token generation for user token", () => {
    render(
      <GenerateTokenModal
        open
        onClose={vi.fn()}
        projectId="p-1"
        tokenType={RegistryTokenType.USER}
        isAdmin
      />,
    );
    fireEvent.change(screen.getByPlaceholderText("e.g. my-ci-token"), {
      target: { value: "my-token" },
    });
    fireEvent.click(screen.getByRole("button", { name: /generate token/i }));
    expect(mutateMock).toHaveBeenCalledWith(
      expect.objectContaining({ robotName: "my-token" }),
      expect.any(Object),
    );
  });
});

