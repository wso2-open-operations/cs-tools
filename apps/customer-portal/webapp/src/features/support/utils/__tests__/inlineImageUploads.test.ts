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


import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  INLINE_IMAGE_MISSING_MESSAGE,
  uploadInlineImagesBeforeCreate,
  type InlineImageDraft,
} from "@features/support/utils/inlineImageUploads";

const mocks = vi.hoisted(() => ({
  uploadStoredAttachment: vi.fn(),
  confirmStoredUpload: vi.fn(),
}));

vi.mock("@api/storedAttachments", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@api/storedAttachments")>();
  return {
    ...actual,
    uploadStoredAttachment: mocks.uploadStoredAttachment,
    confirmStoredUpload: mocks.confirmStoredUpload,
  };
});

import { UploadConfirmPendingError } from "@api/storedAttachments";

const authFetch = vi.fn();
const file = (name: string) => new File(["x"], name, { type: "image/png" });
const html = (...urls: string[]) =>
  `<p>text</p>${urls.map((u) => `<img src="${u}" alt="a">`).join("")}`;

function draftOf(entries: Record<string, string>): InlineImageDraft {
  return {
    queued: new Map(Object.entries(entries).map(([url, name]) => [url, file(name)])),
    ids: new Map(),
  };
}

describe("uploadInlineImagesBeforeCreate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    let n = 0;
    mocks.uploadStoredAttachment.mockImplementation(async () => ({
      id: `id-${++n}`,
    }));
  });

  it("does nothing when the description has no local image", async () => {
    const draft = draftOf({});
    const source = '<p>hi</p><img src="/abcdef12-0000-0000-0000-000000000000">';
    const result = await uploadInlineImagesBeforeCreate(authFetch, "p1", source, draft);
    expect(result).toEqual({ ok: true, html: source });
    expect(mocks.uploadStoredAttachment).not.toHaveBeenCalled();
  });

  it("uploads each image to the project as a comment image and rewrites the sources", async () => {
    const draft = draftOf({ "blob:a": "a.png", "blob:b": "b.png" });
    const result = await uploadInlineImagesBeforeCreate(
      authFetch,
      "p1",
      html("blob:a", "blob:b"),
      draft,
    );
    expect(result.ok).toBe(true);
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(2);
    for (const [, input] of mocks.uploadStoredAttachment.mock.calls) {
      expect(input).toMatchObject({ type: "comment", projectId: "p1" });
      expect(input).not.toHaveProperty("parentId");
    }
    const out = (result as { html: string }).html;
    expect(out).not.toContain("blob:");
    expect(out).toContain('src="/id-1"');
    expect(out).toContain('src="/id-2"');
  });

  it("starts all uploads before any finishes (in parallel)", async () => {
    const release: Array<() => void> = [];
    mocks.uploadStoredAttachment.mockImplementation(
      () => new Promise((resolve) => release.push(() => resolve({ id: `id-${release.length}` }))),
    );
    const draft = draftOf({ "blob:a": "a.png", "blob:b": "b.png", "blob:c": "c.png" });
    const pending = uploadInlineImagesBeforeCreate(
      authFetch,
      "p1",
      html("blob:a", "blob:b", "blob:c"),
      draft,
    );
    await Promise.resolve();
    await Promise.resolve();
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(3);
    release.forEach((r) => r());
    expect((await pending).ok).toBe(true);
  });

  it("uploads an image used twice only once", async () => {
    const draft = draftOf({ "blob:a": "a.png" });
    const result = await uploadInlineImagesBeforeCreate(
      authFetch,
      "p1",
      html("blob:a", "blob:a"),
      draft,
    );
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(1);
    expect((result as { html: string }).html.match(/src="\/id-1"/g)).toHaveLength(2);
  });

  it("ignores a queued image that the description no longer shows", async () => {
    const draft = draftOf({ "blob:a": "a.png", "blob:gone": "gone.png" });
    await uploadInlineImagesBeforeCreate(authFetch, "p1", html("blob:a"), draft);
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(1);
  });

  it("fails with the first error, keeps the images that did upload, and retries only the rest", async () => {
    mocks.uploadStoredAttachment.mockImplementation(async (_f, input) => {
      if (input.name === "bad.png") throw new Error("Upload failed.");
      return { id: `id-${input.name}` };
    });
    const draft = draftOf({ "blob:a": "a.png", "blob:b": "bad.png" });
    const source = html("blob:a", "blob:b");

    const first = await uploadInlineImagesBeforeCreate(authFetch, "p1", source, draft);
    expect(first).toEqual({ ok: false, message: "Upload failed." });
    expect(draft.ids.get("blob:a")).toBe("id-a.png");
    expect(draft.queued.has("blob:b")).toBe(true);

    mocks.uploadStoredAttachment.mockClear();
    mocks.uploadStoredAttachment.mockResolvedValue({ id: "id-b" });
    const second = await uploadInlineImagesBeforeCreate(authFetch, "p1", source, draft);
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(1);
    expect(second.ok).toBe(true);
    expect((second as { html: string }).html).toContain('src="/id-a.png"');
    expect((second as { html: string }).html).toContain('src="/id-b"');
  });

  it("finishes the upload (and does not upload again) when only the confirm step failed", async () => {
    mocks.uploadStoredAttachment.mockRejectedValueOnce(
      new UploadConfirmPendingError("comment", "att-9", "confirm failed", 1),
    );
    mocks.confirmStoredUpload.mockResolvedValue({ id: "att-9" });
    const draft = draftOf({ "blob:a": "a.png" });
    const result = await uploadInlineImagesBeforeCreate(authFetch, "p1", html("blob:a"), draft);
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(1);
    expect(mocks.confirmStoredUpload).toHaveBeenCalledWith(authFetch, {
      type: "comment",
      attachmentId: "att-9",
      chunks: 1,
    });
    expect((result as { html: string }).html).toContain('src="/att-9"');
  });

  it("refuses a local image that has no file behind it", async () => {
    const result = await uploadInlineImagesBeforeCreate(
      authFetch,
      "p1",
      html("blob:unknown"),
      draftOf({}),
    );
    expect(result).toEqual({ ok: false, message: INLINE_IMAGE_MISSING_MESSAGE });
    expect(mocks.uploadStoredAttachment).not.toHaveBeenCalled();
  });

  it("reports progress as images finish", async () => {
    const draft = draftOf({ "blob:a": "a.png", "blob:b": "b.png" });
    const seen: number[] = [];
    await uploadInlineImagesBeforeCreate(authFetch, "p1", html("blob:a", "blob:b"), draft, (n) =>
      seen.push(n),
    );
    expect(seen).toEqual([2, 1, 0]);
  });
});
