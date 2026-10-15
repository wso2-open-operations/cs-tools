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
import "@testing-library/jest-dom/vitest";
import AnnouncementAttachmentsField from "@features/csm-announcements/components/AnnouncementAttachmentsField";
import type { AnnouncementAttachmentItem } from "@features/csm-announcements/api/useAnnouncementAttachments";

function item(name: string, overrides: Partial<AnnouncementAttachmentItem> = {}): AnnouncementAttachmentItem {
  return { key: `k-${name}`, name, sizeBytes: 2048, status: "saved", ...overrides };
}

const fileInput = (): HTMLInputElement => document.querySelector('input[type="file"]') as HTMLInputElement;

describe("AnnouncementAttachmentsField", () => {
  it("offers adding files and says what is allowed and who sees them", () => {
    render(<AnnouncementAttachmentsField items={[]} onAddFiles={vi.fn()} onRemove={vi.fn()} />);

    expect(screen.getByText("Attachments")).toBeInTheDocument();
    expect(screen.getByText("Add files")).toBeInTheDocument();
    expect(screen.getByText(/optional · pdf, images, word, excel, powerpoint, text · up to 5 files/i)).toBeInTheDocument();
    expect(screen.getByText(/customers see them on every announcement case/i)).toBeInTheDocument();
    expect(fileInput().accept).toContain(".pdf");
    expect(fileInput().multiple).toBe(true);
  });

  it("lists each file with its size", () => {
    render(
      <AnnouncementAttachmentsField
        items={[item("advisory.pdf"), item("notes.pdf", { sizeBytes: 500 })]}
        onAddFiles={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    expect(screen.getByText("advisory.pdf")).toBeInTheDocument();
    expect(screen.getByText("2.0 KB")).toBeInTheDocument();
    expect(screen.getByText("500 B")).toBeInTheDocument();
  });

  it("hands the picked files to the caller and clears the picker so the same file can be picked again", () => {
    const onAddFiles = vi.fn();
    render(<AnnouncementAttachmentsField items={[]} onAddFiles={onAddFiles} onRemove={vi.fn()} />);

    const picked = [new File(["a"], "a.pdf"), new File(["b"], "b.pdf")];
    fireEvent.change(fileInput(), { target: { files: picked } });

    expect(onAddFiles).toHaveBeenCalledWith(picked);
    expect(fileInput().value).toBe("");
  });

  it("does nothing when the picker is cancelled", () => {
    const onAddFiles = vi.fn();
    render(<AnnouncementAttachmentsField items={[]} onAddFiles={onAddFiles} onRemove={vi.fn()} />);
    fireEvent.change(fileInput(), { target: { files: [] } });
    expect(onAddFiles).not.toHaveBeenCalled();
  });

  it("removes a file through the caller", () => {
    const onRemove = vi.fn();
    const advisory = item("advisory.pdf");
    render(<AnnouncementAttachmentsField items={[advisory]} onAddFiles={vi.fn()} onRemove={onRemove} />);
    fireEvent.click(screen.getByRole("button", { name: "Remove advisory.pdf" }));
    expect(onRemove).toHaveBeenCalledWith(advisory);
  });

  it("offers Download only for a saved file, and only when the caller can download", () => {
    const onDownload = vi.fn();
    const saved = item("saved.pdf");
    render(
      <AnnouncementAttachmentsField
        items={[saved, item("waiting.pdf", { status: "staged" }), item("going.pdf", { status: "uploading" })]}
        onAddFiles={vi.fn()}
        onRemove={vi.fn()}
        onDownload={onDownload}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Download saved.pdf" }));
    expect(onDownload).toHaveBeenCalledWith(saved);
    expect(screen.queryByRole("button", { name: "Download waiting.pdf" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Download going.pdf" })).not.toBeInTheDocument();
  });

  it("shows no Download at all without a download handler", () => {
    render(<AnnouncementAttachmentsField items={[item("saved.pdf")]} onAddFiles={vi.fn()} onRemove={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /^download/i })).not.toBeInTheDocument();
  });

  it("says where a file is: waiting for the draft, or uploading", () => {
    render(
      <AnnouncementAttachmentsField
        items={[item("waiting.pdf", { status: "staged" }), item("going.pdf", { status: "uploading" })]}
        onAddFiles={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    expect(screen.getByText("Saved with the draft")).toBeInTheDocument();
    expect(screen.getByText("Uploading…")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove going.pdf" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove waiting.pdf" })).not.toBeDisabled();
  });

  it("disables a file whose removal is already under way", () => {
    render(
      <AnnouncementAttachmentsField
        items={[item("advisory.pdf", { removing: true })]}
        onAddFiles={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Remove advisory.pdf" })).toBeDisabled();
  });

  it("disables adding and removing while the form is busy", () => {
    render(<AnnouncementAttachmentsField items={[item("advisory.pdf")]} onAddFiles={vi.fn()} onRemove={vi.fn()} disabled />);
    expect(fileInput()).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove advisory.pdf" })).toBeDisabled();
  });

  it("stops offering to add files once the limit is reached", () => {
    const five = ["a", "b", "c", "d", "e"].map((n) => item(`${n}.pdf`));
    render(<AnnouncementAttachmentsField items={five} onAddFiles={vi.fn()} onRemove={vi.fn()} />);
    expect(fileInput()).toBeDisabled();
  });

  it("shows an extra note under the hint", () => {
    render(
      <AnnouncementAttachmentsField items={[]} onAddFiles={vi.fn()} onRemove={vi.fn()} note="Changing files sends it back to draft." />,
    );
    expect(screen.getByText("Changing files sends it back to draft.")).toBeInTheDocument();
  });

  it("says it is loading rather than showing an empty list, and reports a failed load", () => {
    const { rerender } = render(
      <AnnouncementAttachmentsField items={[]} onAddFiles={vi.fn()} onRemove={vi.fn()} isLoading />,
    );
    expect(screen.getByText(/loading attachments/i)).toBeInTheDocument();

    rerender(<AnnouncementAttachmentsField items={[]} onAddFiles={vi.fn()} onRemove={vi.fn()} loadFailed />);
    expect(screen.getByText(/couldn't load this request's attachments/i)).toBeInTheDocument();
  });

  describe("read-only", () => {
    it("lists the files without offering to add or remove any", () => {
      render(<AnnouncementAttachmentsField readOnly items={[item("advisory.pdf")]} onDownload={vi.fn()} />);
      expect(screen.getByText("advisory.pdf")).toBeInTheDocument();
      expect(screen.queryByText("Add files")).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Remove advisory.pdf" })).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Download advisory.pdf" })).toBeInTheDocument();
    });

    it("says there are none, but never while the list is still loading or failed to load", () => {
      const { rerender } = render(<AnnouncementAttachmentsField readOnly items={[]} />);
      expect(screen.getByText("No attachments.")).toBeInTheDocument();

      rerender(<AnnouncementAttachmentsField readOnly items={[]} isLoading />);
      expect(screen.queryByText("No attachments.")).not.toBeInTheDocument();

      rerender(<AnnouncementAttachmentsField readOnly items={[]} loadFailed />);
      expect(screen.queryByText("No attachments.")).not.toBeInTheDocument();
    });
  });
});
