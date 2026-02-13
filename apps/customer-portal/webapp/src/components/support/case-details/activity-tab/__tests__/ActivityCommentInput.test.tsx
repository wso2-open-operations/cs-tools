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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ActivityCommentInput from "@case-details-activity/ActivityCommentInput";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";

const mockMutate = vi.fn();

vi.mock("@api/usePostComment", () => ({
  usePostComment: vi.fn(() => ({
    mutate: mockMutate,
    isPending: false,
  })),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: vi.fn(() => ({
    isSignedIn: true,
    isLoading: false,
  })),
}));

function renderInput(caseId = "case-001") {
  return render(
    <ThemeProvider theme={createTheme()}>
      <ErrorBannerProvider>
        <ActivityCommentInput caseId={caseId} />
      </ErrorBannerProvider>
    </ThemeProvider>,
  );
}

describe("ActivityCommentInput", () => {
  it("should render placeholder and send button", () => {
    renderInput();
    expect(screen.getByPlaceholderText("Add a comment...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /send comment/i })).toBeInTheDocument();
  });

  it("should disable send when input is empty", () => {
    renderInput();
    const btn = screen.getByRole("button", { name: /send comment/i });
    expect(btn).toBeDisabled();
  });

  it("should enable send when input has content and call mutate on click", () => {
    renderInput();
    const input = screen.getByPlaceholderText("Add a comment...");
    fireEvent.change(input, { target: { value: "Hello" } });
    const btn = screen.getByRole("button", { name: /send comment/i });
    expect(btn).not.toBeDisabled();
    fireEvent.click(btn);
    expect(mockMutate).toHaveBeenCalledWith(
      { caseId: "case-001", body: { content: "Hello" } },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );
  });
});
