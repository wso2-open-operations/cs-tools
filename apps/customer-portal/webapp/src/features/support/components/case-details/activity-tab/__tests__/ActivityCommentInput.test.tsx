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
import LoggerProvider from "@context/logger/LoggerProvider";

const mockMutate = vi.fn();

vi.mock("@features/support/api/usePostComment", () => ({
  usePostComment: vi.fn(() => ({
    mutate: mockMutate,
    isPending: false,
  })),
}));

vi.mock("@features/support/api/usePostAttachments", () => ({
  usePostAttachments: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}));

vi.mock("@case-details-attachments/UploadAttachmentModal", () => ({
  default: () => null,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: vi.fn(() => ({
    isSignedIn: true,
    isLoading: false,
  })),
}));

// Stub the real (Lexical) editor with a plain textarea so tests can drive
// `onChange` directly, e.g. to simulate a comment body over the size guard's
// threshold without needing to type that many characters through Lexical.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    onChange,
    overlayElement,
  }: {
    onChange?: (html: string) => void;
    overlayElement?: React.ReactNode;
  }) => (
    <div>
      <textarea
        data-testid="case-description-editor"
        onChange={(e) => onChange?.(e.target.value)}
      />
      {overlayElement}
    </div>
  ),
}));

function renderInput(caseId = "case-001") {
  return render(
    <LoggerProvider config={{ level: "ERROR", prefix: "Test" }}>
      <ThemeProvider theme={createTheme()}>
        <ErrorBannerProvider>
          <ActivityCommentInput caseId={caseId} />
        </ErrorBannerProvider>
      </ThemeProvider>
    </LoggerProvider>,
  );
}

describe("ActivityCommentInput", () => {
  it("should render editor and send button", () => {
    renderInput();
    expect(screen.getByTestId("case-description-editor")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /send comment/i }),
    ).toBeInTheDocument();
  });

  it("should disable send when input is empty", () => {
    renderInput();
    const btn = screen.getByRole("button", { name: /send comment/i });
    expect(btn).toBeDisabled();
  });

  it("should disable send and swap the tooltip to a size-limit message when the comment body exceeds the 1 MB guard", async () => {
    renderInput();
    const editor = screen.getByTestId("case-description-editor");
    // 1 MB threshold minus the JSON envelope headroom; a single repeated
    // character is enough to cross it without needing real inline images.
    const oversized = "a".repeat(1024 * 1024);
    fireEvent.change(editor, { target: { value: oversized } });

    const btn = screen.getByRole("button", { name: /send comment/i });
    expect(btn).toBeDisabled();

    // MUI Tooltip only mounts its content on hover; opening it here confirms
    // the title was swapped from the static "Send comment" to the size-limit
    // guidance, not just that the button got disabled for some other reason.
    fireEvent.mouseOver(btn);
    expect(
      await screen.findByText(/upload large images as attachments instead/i),
    ).toBeInTheDocument();
  });
});
