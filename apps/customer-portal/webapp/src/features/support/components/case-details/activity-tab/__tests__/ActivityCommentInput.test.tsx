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

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ActivityCommentInput from "@case-details-activity/ActivityCommentInput";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import LoggerProvider from "@context/logger/LoggerProvider";

const mockMutate = vi.fn();
const mockUpload = vi.fn();
const mockConfirm = vi.fn();
const entityAttachmentsEnabled = vi.fn(() => true);

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => vi.fn(),
}));

vi.mock("@/config/entityAttachmentsConfig", () => ({
  isEntityAttachmentsEnabled: () => entityAttachmentsEnabled(),
}));

vi.mock("@api/storedAttachments", async () => {
  const actual =
    await vi.importActual<typeof import("@api/storedAttachments")>(
      "@api/storedAttachments",
    );
  return {
    ...actual,
    uploadStoredAttachment: (...args: unknown[]) => mockUpload(...args),
    confirmStoredUpload: (...args: unknown[]) => mockConfirm(...args),
  };
});

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

const useCustomerPermissionsMock = vi.fn(() => ({
  canCreateCase: true,
  isLoading: false,
}));

vi.mock("@hooks/useCustomerPermissions", () => ({
  useCustomerPermissions: () => useCustomerPermissionsMock(),
}));

// Stub the real (Lexical) editor with a plain textarea so tests can drive
// `onChange` directly, e.g. to simulate a comment body over the size guard's
// threshold without needing to type that many characters through Lexical.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    onChange,
    overlayElement,
    onImageUpload,
  }: {
    onChange?: (html: string) => void;
    overlayElement?: React.ReactNode;
    onImageUpload?: (file: File, previewUrl: string) => Promise<void>;
  }) => (
    <div>
      <textarea
        data-testid="case-description-editor"
        onChange={(e) => onChange?.(e.target.value)}
      />
      {onImageUpload && (
        <button
          type="button"
          onClick={() =>
            // The real editor removes the image when this rejects.
            onImageUpload(
              new File(["x"], "image.png", { type: "image/png" }),
              "blob:http://localhost/preview-1",
            ).catch(() => {})
          }
        >
          add image
        </button>
      )}
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
  beforeEach(() => {
    mockMutate.mockReset();
    mockUpload.mockReset();
    mockConfirm.mockReset();
    entityAttachmentsEnabled.mockReset();
    entityAttachmentsEnabled.mockReturnValue(true);
    Object.assign(URL, { revokeObjectURL: vi.fn() });
    useCustomerPermissionsMock.mockReset();
    useCustomerPermissionsMock.mockReturnValue({
      canCreateCase: true,
      isLoading: false,
    });
  });

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

  it("renders nothing for a read-only (isReadOnly INTERNAL) caller", () => {
    useCustomerPermissionsMock.mockReturnValue({
      canCreateCase: false,
      isLoading: false,
    });
    const { container } = renderInput();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing while permissions are still loading", () => {
    useCustomerPermissionsMock.mockReturnValue({
      canCreateCase: true,
      isLoading: true,
    });
    const { container } = renderInput();
    expect(container).toBeEmptyDOMElement();
  });

  describe("images added to the comment", () => {
    const PREVIEW = "blob:http://localhost/preview-1";
    const ID = "0f15cbcc-c36b-4310-af2f-404599013196";
    const withImage = `<p>see</p><img src="${PREVIEW}" alt="Pasted Image">`;

    function typeComment(html: string) {
      fireEvent.change(screen.getByTestId("case-description-editor"), {
        target: { value: html },
      });
    }
    const sendButton = () => screen.getByRole("button", { name: /send comment/i });

    it("uploads the image as a comment attachment of the case and sends only its /<id> reference", async () => {
      mockUpload.mockResolvedValue({ id: ID.toUpperCase(), status: "COMPLETE" });
      renderInput("case-001");

      fireEvent.click(screen.getByRole("button", { name: "add image" }));
      typeComment(withImage);
      await waitFor(() => expect(sendButton()).not.toBeDisabled());
      fireEvent.click(sendButton());

      expect(mockUpload).toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({
          type: "comment",
          parentId: "case-001",
          name: "image.png",
        }),
      );
      expect(mockMutate).toHaveBeenCalledTimes(1);
      const content = mockMutate.mock.calls[0][0].body.content as string;
      expect(content).toContain(`src="/${ID}"`);
      expect(content).not.toContain("blob:");
      expect(content).not.toContain("base64");
    });

    it("keeps send disabled while the image is still uploading", async () => {
      let finish: (v: { id: string }) => void = () => {};
      mockUpload.mockReturnValue(new Promise((r) => (finish = r)));
      renderInput();

      fireEvent.click(screen.getByRole("button", { name: "add image" }));
      typeComment(withImage);
      expect(sendButton()).toBeDisabled();
      fireEvent.click(sendButton());
      expect(mockMutate).not.toHaveBeenCalled();

      await act(async () => finish({ id: ID }));
      await waitFor(() => expect(sendButton()).not.toBeDisabled());
    });

    it("finishes an upload whose confirm step failed instead of uploading again", async () => {
      const { UploadConfirmPendingError } = await vi.importActual<
        typeof import("@api/storedAttachments")
      >("@api/storedAttachments");
      mockUpload.mockRejectedValue(
        new UploadConfirmPendingError("comment", "att-9", "confirm failed"),
      );
      mockConfirm.mockResolvedValue({ id: ID, status: "COMPLETE" });
      renderInput();

      fireEvent.click(screen.getByRole("button", { name: "add image" }));
      typeComment(withImage);
      await waitFor(() => expect(sendButton()).not.toBeDisabled());
      fireEvent.click(sendButton());

      expect(mockConfirm).toHaveBeenCalledWith(expect.anything(), {
        type: "comment",
        attachmentId: "att-9",
      });
      expect(mockUpload).toHaveBeenCalledTimes(1);
      expect(mockMutate.mock.calls[0][0].body.content).toContain(`src="/${ID}"`);
    });

    it("does not send a comment whose image failed to upload", async () => {
      mockUpload.mockRejectedValue(new Error("The file could not be uploaded."));
      renderInput();

      fireEvent.click(screen.getByRole("button", { name: "add image" }));
      typeComment(withImage);
      await waitFor(() => expect(sendButton()).not.toBeDisabled());
      fireEvent.click(sendButton());

      expect(mockMutate).not.toHaveBeenCalled();
      expect(
        await screen.findByText(/could not be uploaded/i),
      ).toBeInTheDocument();
    });

    it("does not upload through entity-service when attachments are off (images stay embedded)", () => {
      entityAttachmentsEnabled.mockReturnValue(false);
      renderInput();
      expect(
        screen.queryByRole("button", { name: "add image" }),
      ).not.toBeInTheDocument();
    });
  });
});
