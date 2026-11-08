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

import { useState } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import EditorUploadProgress from "@components/rich-text-editor/EditorUploadProgress";

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as CaseActivitiesFeed.test.tsx). Pulled in
// transitively via CsmUploadAttachmentModal -> useCsmCaseAttachments, just
// for its MAX_ATTACHMENT_SIZE_BYTES constant — the mock only exists to keep
// that import chain from throwing on missing runtime config.
vi.mock("@api/backend/client", () => {
  const api = { post: vi.fn() };
  return { useBackendApi: () => api };
});

// Also on the import chain (CsmUploadAttachmentModal -> useCsmCaseAttachments),
// and it reaches the runtime-config module the same way.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: null }),
}));

// Replaced whole, like the client above: the real module pulls in runtime config.
vi.mock("@features/csm-cases/api/entityAttachments", () => ({
  uploadEntityAttachment: vi.fn(),
  entityAttachmentErrorMessage: (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback,
}));

import { uploadEntityAttachment } from "@features/csm-cases/api/entityAttachments";
import { commentUploadProgress } from "@features/csm-cases/utils/commentUploadProgress";
import { PartialSendError } from "@features/csm-cases/utils/sendCommentWithAttachments";
import CsmCaseCommentInput, {
  type CommentAttachmentDraft,
} from "@features/csm-cases/components/CsmCaseCommentInput";

// This component isn't under test here; stub it to a plain textarea (same
// technique EditCaseDetailsDialog.test.tsx uses for the same dependency).
// Also surfaces `attachments.length` as text so drag-and-drop tests below
// can assert on it without needing the real editor's own attachment-chip UI.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
    attachments,
    uploads = [],
    uploadsWaiting = 0,
    onImageUpload,
    onPasteError,
  }: {
    value: string;
    onChange: (v: string) => void;
    attachments: File[];
    uploads?: { name: string; percent: number }[];
    uploadsWaiting?: number;
    onImageUpload?: (file: File) => string;
    onPasteError?: (reason: "size" | "type" | "count") => void;
  }) => (
    <div data-testid="editor-stub">
      <EditorUploadProgress uploads={uploads} waiting={uploadsWaiting} />
      {onPasteError && (
        <button type="button" onClick={() => onPasteError("count")}>
          paste-too-many
        </button>
      )}
      {onImageUpload && (
        <button
          type="button"
          onClick={() => {
            const src = onImageUpload(new File([new Uint8Array(4)], "shot.png", { type: "image/png" }));
            onChange(`${value}<img src="${src}" alt="Pasted Image">`);
          }}
        >
          paste-image
        </button>
      )}
      <textarea
        aria-label="comment-editor"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <span data-testid="attachment-count">{attachments.length}</span>
    </div>
  ),
}));

const PAUSED_REASON =
  "This case is paused — customer replies are disabled. Resume work to reply to the customer.";
const NOT_STARTED_REASON =
  "Customer replies are disabled unless the case is actively in progress.";

describe("CsmCaseCommentInput — resume-work quick-fix", () => {
  it("shows the inline resume link when the only lock reason is paused work, and calls onResumeWork", () => {
    const onResumeWork = vi.fn();
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={onResumeWork}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Resume work" }));
    expect(onResumeWork).toHaveBeenCalledTimes(1);
  });

  it("doesn't also repeat the raw lock reason in the send-row status line once the quick-fix covers it", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
      />,
    );

    // The quick-fix's own sentence is present once...
    expect(
      screen.getByText("Only resumed work can send public replies to the customer.", {
        exact: false,
      }),
    ).toBeInTheDocument();
    // ...and the send-row status line falls back to the normal hint instead
    // of repeating the raw backend reason a second time.
    expect(screen.queryByText(PAUSED_REASON)).not.toBeInTheDocument();
    expect(screen.getByText("Ctrl/Cmd + Enter to send.")).toBeInTheDocument();
  });

  it("does not show the resume link when the case hasn't started yet (not just paused)", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={NOT_STARTED_REASON}
        canResumeToUnlockPublicReply={false}
        onResumeWork={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Resume work" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText(NOT_STARTED_REASON)).toBeInTheDocument();
  });

  it("does not show the resume link when public replies are already allowed", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={null}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Resume work" }),
    ).not.toBeInTheDocument();
  });

  it("disables the resume link and shows a pending label while resuming", () => {
    render(
      <CsmCaseCommentInput
        onSubmit={vi.fn()}
        publicCommentDisabledReason={PAUSED_REASON}
        canResumeToUnlockPublicReply
        onResumeWork={vi.fn()}
        isResumingWork
      />,
    );

    expect(screen.getByRole("button", { name: "Resuming…" })).toBeDisabled();
  });
});

/** A File-like drop payload — jsdom's DataTransfer doesn't populate `files`
 * from a plain object literal the way a real browser drag event would. */
function fileDrop(files: File[]): { dataTransfer: { types: string[]; files: File[] } } {
  return { dataTransfer: { types: ["Files"], files } };
}

function makeFile(name: string, sizeBytes: number, type = "text/plain"): File {
  return new File([new Uint8Array(sizeBytes)], name, { type });
}

describe("CsmCaseCommentInput — drag-and-drop attachments", () => {
  it("shows the drop overlay only while a file is being dragged over the composer", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    expect(screen.queryByText("Drop files to attach")).not.toBeInTheDocument();

    fireEvent.dragEnter(composer, fileDrop([]));
    expect(screen.getByText("Drop files to attach")).toBeInTheDocument();

    fireEvent.dragLeave(composer, fileDrop([]));
    expect(screen.queryByText("Drop files to attach")).not.toBeInTheDocument();
  });

  it("attaches a dropped file without opening the naming modal", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.drop(composer, fileDrop([makeFile("screenshot.png", 1024, "image/png")]));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    expect(screen.queryByText("Attach a file")).not.toBeInTheDocument();
  });

  it("keeps only the first file of a multi-file drop, since a reply takes one attachment, and says so", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.drop(
      composer,
      fileDrop([makeFile("a.txt", 100), makeFile("b.txt", 200), makeFile("c.txt", 300)]),
    );

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    expect(screen.getByText(/Only one file allowed/)).toBeInTheDocument();
  });

  it("adds nothing more to a reply that already has its attachment", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.drop(composer, fileDrop([makeFile("a.txt", 100)]));
    fireEvent.drop(composer, fileDrop([makeFile("b.txt", 200)]));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    expect(screen.getByText(/Only one file allowed/)).toBeInTheDocument();
  });

  it("does not call the file already attached a second attachment", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    const same = makeFile("a.txt", 100);
    fireEvent.drop(composer, fileDrop([same]));
    fireEvent.drop(composer, fileDrop([same]));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    expect(screen.queryByText(/Only one file allowed/)).toBeNull();
  });

  describe("size limit", () => {
    afterEach(() => {
      window.config = {} as typeof window.config;
    });

    it("rejects a dropped file over the configured limit, but keeps the ones that fit", () => {
      window.config = { CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 25 } as unknown as typeof window.config;
      render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
      const composer = screen.getByTestId("csm-comment-composer");

      // 11 MB fits under 25 MB; 26 MB does not.
      fireEvent.drop(
        composer,
        fileDrop([makeFile("ok.zip", 11 * 1024 * 1024), makeFile("huge.zip", 26 * 1024 * 1024)]),
      );

      expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
      expect(screen.getByText(/huge\.zip.*too large.*25\.00 MB/i)).toBeInTheDocument();
    });

    it("keeps the default of 10 MB through the entity flow when nothing is configured", () => {
      render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
      const composer = screen.getByTestId("csm-comment-composer");

      fireEvent.drop(composer, fileDrop([makeFile("big.zip", 9 * 1024 * 1024)]));

      expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    });
  });

  it("still prevents the browser's default file handling when the composer is disabled, without attaching anything", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} disabled />);
    const composer = screen.getByTestId("csm-comment-composer");

    // fireEvent returns false when preventDefault() was called on a
    // cancelable event — a real file drag must still be prevented even
    // though the composer won't act on it, or the browser falls through to
    // navigating the tab to open the dropped file.
    const notPrevented = fireEvent.drop(composer, fileDrop([makeFile("a.txt", 100)]));

    expect(notPrevented).toBe(false);
    expect(screen.getByTestId("attachment-count")).toHaveTextContent("0");
  });

  it("dedupes a file that's already attached", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");
    const file = makeFile("dup.txt", 100);

    fireEvent.drop(composer, fileDrop([file]));
    fireEvent.drop(composer, fileDrop([file]));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
  });

  it("ignores a drag that isn't carrying files (e.g. dragging selected text)", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.dragEnter(composer, { dataTransfer: { types: ["text/plain"], files: [] } });
    expect(screen.queryByText("Drop files to attach")).not.toBeInTheDocument();
  });
});

/**
 * Stands in for the real parent (`CsmCaseDetailPage`), which lifts the draft
 * out of this component so it survives the Activities tab body unmounting on
 * a tab switch and remounting on switch-back. `mounted` toggles
 * `CsmCaseCommentInput` itself, the same way the page conditionally renders
 * the tab body — the draft state below lives in this harness the whole time,
 * exactly as it would in the parent page.
 */
function DraftLiftingHarness({
  onSubmit = vi.fn(),
}: {
  onSubmit?: (
    html: string,
    internal: boolean,
    attachments: CommentAttachmentDraft[],
  ) => Promise<unknown> | void;
}) {
  const [mounted, setMounted] = useState(true);
  const [html, setHtml] = useState("");
  const [attachments, setAttachments] = useState<CommentAttachmentDraft[]>([]);
  const [internal, setInternal] = useState(false);
  const [sourceMode, setSourceMode] = useState(false);

  return (
    <>
      <button type="button" onClick={() => setMounted((m) => !m)}>
        toggle tab
      </button>
      {mounted && (
        <CsmCaseCommentInput
          onSubmit={onSubmit}
          draftHtml={html}
          onDraftHtmlChange={setHtml}
          draftAttachments={attachments}
          onDraftAttachmentsChange={setAttachments}
          draftInternal={internal}
          onDraftInternalChange={setInternal}
          draftSourceMode={sourceMode}
          onDraftSourceModeChange={setSourceMode}
        />
      )}
    </>
  );
}

describe("CsmCaseCommentInput — lifted draft state survives unmount/remount", () => {
  it("keeps typed text after the composer unmounts and remounts (e.g. a tab switch)", () => {
    render(<DraftLiftingHarness />);

    fireEvent.change(screen.getByLabelText("comment-editor"), {
      target: { value: "a reply in progress" },
    });
    expect(screen.getByLabelText("comment-editor")).toHaveValue(
      "a reply in progress",
    );

    // Simulate switching to another case-detail tab and back.
    fireEvent.click(screen.getByRole("button", { name: "toggle tab" }));
    expect(screen.queryByLabelText("comment-editor")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "toggle tab" }));

    expect(screen.getByLabelText("comment-editor")).toHaveValue(
      "a reply in progress",
    );
  });

  it("keeps a staged attachment after unmount/remount", () => {
    render(<DraftLiftingHarness />);
    const composer = screen.getByTestId("csm-comment-composer");

    fireEvent.drop(composer, fileDrop([makeFile("draft.txt", 100)]));
    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");

    fireEvent.click(screen.getByRole("button", { name: "toggle tab" }));
    fireEvent.click(screen.getByRole("button", { name: "toggle tab" }));

    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
  });

  it("keeps the internal-note toggle state after unmount/remount", () => {
    render(<DraftLiftingHarness />);

    fireEvent.click(screen.getByRole("switch", { name: /Internal note/i }));
    expect(
      screen.getByRole("switch", { name: /Internal note/i }),
    ).toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: "toggle tab" }));
    fireEvent.click(screen.getByRole("button", { name: "toggle tab" }));

    expect(
      screen.getByRole("switch", { name: /Internal note/i }),
    ).toBeChecked();
  });

  it("clears the lifted draft on a successful submit", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<DraftLiftingHarness onSubmit={onSubmit} />);

    fireEvent.change(screen.getByLabelText("comment-editor"), {
      target: { value: "send this" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    await screen.findByText("Ctrl/Cmd + Enter to send.");
    expect(screen.getByLabelText("comment-editor")).toHaveValue("");
  });
});

describe("CsmCaseCommentInput — emptying once the comment is posted", () => {
  const sendWithFile = () => {
    fireEvent.change(screen.getByLabelText("comment-editor"), { target: { value: "see the file" } });
    fireEvent.drop(
      screen.getByTestId("csm-comment-composer"),
      fileDrop([makeFile("big.zip", 1024, "application/zip")]),
    );
    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));
  };

  it("empties the editor and its file list when the sender says the comment is posted, while the upload goes on", async () => {
    let finish!: () => void;
    const onSubmit = vi.fn(
      (_html: string, _internal: boolean, _files: unknown, controls: { commentSettled: () => void }) =>
        new Promise<void>((resolve) => {
          controls.commentSettled();
          finish = resolve;
        }),
    );
    render(<CsmCaseCommentInput onSubmit={onSubmit} />);
    sendWithFile();

    await waitFor(() => expect(screen.getByLabelText("comment-editor")).toHaveValue(""));
    expect(screen.getByTestId("attachment-count")).toHaveTextContent("0");
    // Still sending: the upload has not ended.
    expect(screen.getByRole("button", { name: /Sending/ })).toBeInTheDocument();
    finish();
    await screen.findByRole("button", { name: /Send to customer/ });
  });

  it("keeps the comment and the file when the comment was not posted", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new PartialSendError("The comment was not posted.", false, []));
    render(<CsmCaseCommentInput onSubmit={onSubmit} />);
    sendWithFile();

    expect(await screen.findByText("The comment was not posted.")).toBeInTheDocument();
    expect(screen.getByLabelText("comment-editor")).toHaveValue("see the file");
    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
  });

  it("puts a file that did not upload back after the editor was emptied, so Send repeats only it", async () => {
    const onSubmit = vi.fn(
      async (_html: string, _internal: boolean, _files: unknown[], controls: { commentSettled: () => void }) => {
        controls.commentSettled();
        throw new PartialSendError('"big.zip": the upload failed', true, []);
      },
    );
    render(<CsmCaseCommentInput onSubmit={onSubmit} />);
    sendWithFile();

    expect(await screen.findByText(/"big\.zip": the upload failed/)).toBeInTheDocument();
    expect(screen.getByLabelText("comment-editor")).toHaveValue("");
    expect(screen.getByTestId("attachment-count")).toHaveTextContent("1");
  });

  it("says how many files are waiting behind the one uploading", async () => {
    let finish!: () => void;
    const onSubmit = vi.fn(
      (_h: string, _i: boolean, _f: unknown, controls: { waiting: (n: number) => void }) =>
        new Promise<void>((resolve) => {
          controls.waiting(2);
          finish = resolve;
        }),
    );
    render(<CsmCaseCommentInput onSubmit={onSubmit} uploadProgress={{ name: "a.zip", percent: 10 }} />);
    sendWithFile();

    expect(await screen.findByText("2 more files waiting to upload")).toBeInTheDocument();
    finish();
    await waitFor(() => expect(screen.queryByText(/waiting to upload/)).toBeNull());
  });
});

describe("CsmCaseCommentInput — images added to the comment", () => {
  const CASE_ID = "22222222-2222-4222-8222-222222222222";
  const IMAGE_ID = "AAAAAAAA-1111-4111-8111-111111111111";
  const upload = vi.mocked(uploadEntityAttachment);
  let blobCount = 0;

  beforeEach(() => {
    blobCount = 0;
    upload.mockReset();
    URL.createObjectURL = vi.fn(() => `blob:https://csm.example.com/${(blobCount += 1)}`);
    URL.revokeObjectURL = vi.fn();
  });

  it("tells the user a comment can have at most 5 images when the editor refuses another", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} imageUploadParentId={CASE_ID} />);
    fireEvent.click(screen.getByRole("button", { name: "paste-too-many" }));
    expect(screen.getByText("A comment or description can have at most 5 images.")).toBeInTheDocument();
  });

  it("gives the editor no image uploader without a case id, so images are embedded as before", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    expect(screen.queryByRole("button", { name: "paste-image" })).not.toBeInTheDocument();
  });

  it("uploads a pasted image only when the comment is sent, then sends its id instead of the preview URL", async () => {
    let finish: (value: { id: string }) => void = () => {};
    upload.mockReturnValue(new Promise((resolve) => (finish = resolve)) as never);
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CsmCaseCommentInput onSubmit={onSubmit} imageUploadParentId={CASE_ID} />);

    fireEvent.change(screen.getByLabelText("comment-editor"), { target: { value: "see " } });
    fireEvent.click(screen.getByRole("button", { name: "paste-image" }));

    // Pasting sends nothing; the Send button stays available.
    expect(upload).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /Send to customer/ })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    expect(upload).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ type: "comment", parentId: CASE_ID, name: "shot.png" }),
    );
    expect(await screen.findByText("Uploading images…")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sending…" })).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();

    await act(async () => finish({ id: IMAGE_ID }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    const [html] = onSubmit.mock.calls[0];
    expect(html).toBe(`see <img src="/${IMAGE_ID.toLowerCase()}" alt="Pasted Image">`);
    expect(html).not.toContain("blob:");
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:https://csm.example.com/1");
  });

  it("uploads nothing for an image that was removed before Send", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CsmCaseCommentInput onSubmit={onSubmit} imageUploadParentId={CASE_ID} />);

    fireEvent.click(screen.getByRole("button", { name: "paste-image" }));
    fireEvent.change(screen.getByLabelText("comment-editor"), { target: { value: "no image after all" } });
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(upload).not.toHaveBeenCalled();
  });

  it("explains a failed image on the first Send and posts the comment without it on the second", async () => {
    upload.mockRejectedValue(new Error("Could not upload shot.png (HTTP 500). Please try again."));
    const onSubmit = vi.fn();
    render(<CsmCaseCommentInput onSubmit={onSubmit} imageUploadParentId={CASE_ID} />);

    fireEvent.change(screen.getByLabelText("comment-editor"), { target: { value: "see " } });
    fireEvent.click(screen.getByRole("button", { name: "paste-image" }));
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    expect(await screen.findByText(/Could not upload shot\.png.*Press Send again/)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    const [html] = onSubmit.mock.calls[0];
    expect(html).toBe("see ");
    expect(html).not.toContain("blob:");
    expect(upload).toHaveBeenCalledTimes(1); // the failed image is not tried again
  });

  it("asks again when the draft changes after the failed-image notice", async () => {
    upload.mockRejectedValue(new Error("Could not upload shot.png (HTTP 500). Please try again."));
    const onSubmit = vi.fn();
    render(<CsmCaseCommentInput onSubmit={onSubmit} imageUploadParentId={CASE_ID} />);

    fireEvent.change(screen.getByLabelText("comment-editor"), { target: { value: "see " } });
    fireEvent.click(screen.getByRole("button", { name: "paste-image" }));
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));
    await screen.findByText(/Press Send again/);

    fireEvent.change(screen.getByLabelText("comment-editor"), {
      target: { value: 'see this <img src="blob:https://csm.example.com/1" alt="x">' },
    });
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    expect(await screen.findByText(/Press Send again/)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("sends a comment that is only an image", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CsmCaseCommentInput onSubmit={onSubmit} imageUploadParentId={CASE_ID} />);

    fireEvent.click(screen.getByRole("button", { name: "paste-image" }));
    expect(screen.getByRole("button", { name: /Send to customer/ })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0][0]).toBe(`<img src="/${IMAGE_ID.toLowerCase()}" alt="Pasted Image">`);
  });

  it("sends the remaining image when the only other image failed and was dropped", async () => {
    upload
      .mockRejectedValueOnce(new Error("Could not upload shot.png (HTTP 500). Please try again."))
      .mockResolvedValue({ id: IMAGE_ID } as never);
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CsmCaseCommentInput onSubmit={onSubmit} imageUploadParentId={CASE_ID} />);

    fireEvent.click(screen.getByRole("button", { name: "paste-image" }));
    fireEvent.click(screen.getByRole("button", { name: "paste-image" }));
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));
    await screen.findByText(/Press Send again/);

    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0][0]).toBe(`<img src="/${IMAGE_ID.toLowerCase()}" alt="Pasted Image">`);
  });

  it("never sends a blob: image this page did not upload", async () => {
    const onSubmit = vi.fn();
    render(<CsmCaseCommentInput onSubmit={onSubmit} imageUploadParentId={CASE_ID} />);

    fireEvent.change(screen.getByLabelText("comment-editor"), {
      target: { value: 'x<img src="blob:https://csm.example.com/elsewhere">' },
    });
    fireEvent.click(screen.getByRole("button", { name: /Send to customer/ }));

    expect(await screen.findByText(/could not be uploaded/)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

describe("upload progress while a comment is sent", () => {
  it("shows the file's name and percentage as a bar inside the editor", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} uploadProgress={{ name: "10mb.pdf", percent: 42 }} />);
    expect(screen.getByText("Uploading 10mb.pdf")).toBeInTheDocument();
    expect(screen.getByText("42%")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Uploading 10mb.pdf" }).getAttribute("aria-valuenow")).toBe("42");
  });

  it("shows nothing when no upload runs", () => {
    render(<CsmCaseCommentInput onSubmit={vi.fn()} />);
    expect(screen.queryByText(/^Uploading /)).toBeNull();
  });
});

describe("commentUploadProgress", () => {
  it("is the running upload's name and percent, and null otherwise", () => {
    expect(
      commentUploadProgress({ isPending: true, uploadProgress: 7, variables: { name: "a.zip" } }),
    ).toEqual({ name: "a.zip", percent: 7 });
    expect(
      commentUploadProgress({ isPending: true, uploadProgress: 0, variables: { file: new File([], "f.txt") } }),
    ).toEqual({ name: "f.txt", percent: 0 });
    expect(commentUploadProgress({ isPending: false, uploadProgress: 7 })).toBeNull();
    expect(commentUploadProgress({ isPending: true, uploadProgress: null })).toBeNull();
  });
});
