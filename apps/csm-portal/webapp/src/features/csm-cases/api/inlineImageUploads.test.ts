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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Replaced whole: the real module pulls in the backend client, which needs runtime config.
vi.mock("@features/csm-cases/api/entityAttachments", () => ({
  uploadEntityAttachment: vi.fn(),
  entityAttachmentErrorMessage: (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback,
}));

import type { BackendApi } from "@api/backend/client";
import { uploadEntityAttachment } from "@features/csm-cases/api/entityAttachments";
import {
  dropFailedImages,
  hasBlobSource,
  inspectInlineImages,
  queueInlineImage,
  releaseInlineImages,
  resolveInlineImageSources,
  uploadQueuedImages,
} from "@features/csm-cases/api/inlineImageUploads";

const CASE_ID = "22222222-2222-4222-8222-222222222222";
const IMAGE_ID = "AAAAAAAA-1111-4111-8111-111111111111";
const api = {} as BackendApi;
const file = new File([new Uint8Array(8)], "shot.png", { type: "image/png" });

const upload = vi.mocked(uploadEntityAttachment);
let counter = 0;
const created: string[] = [];

beforeEach(() => {
  counter = 0;
  created.length = 0;
  URL.createObjectURL = vi.fn(() => {
    counter += 1;
    const url = `blob:https://csm.example.com/${counter}`;
    created.push(url);
    return url;
  });
  URL.revokeObjectURL = vi.fn();
  upload.mockReset();
});

afterEach(() => {
  created.forEach((url) => releaseInlineImages(`<img src="${url}">`));
});

const img = (src: string) => `<p>hi</p><img src="${src}" alt="x">`;

const none = { pending: 0, queued: 0, failedMessage: null, unknown: 0 };

describe("queueInlineImage", () => {
  it("keeps the file and returns a local preview URL without uploading anything", () => {
    const src = queueInlineImage(file);

    expect(src).toBe("blob:https://csm.example.com/1");
    expect(upload).not.toHaveBeenCalled();
    expect(inspectInlineImages(img(src))).toEqual({ ...none, queued: 1 });
  });
});

describe("uploadQueuedImages", () => {
  it("uploads the file as a comment attachment of the case", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const src = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(src));

    expect(upload).toHaveBeenCalledWith(api, {
      type: "comment",
      parentId: CASE_ID,
      file,
      name: "shot.png",
    });
    expect(inspectInlineImages(img(src))).toEqual(none);
  });

  it("swaps the preview URL for the lowercase attachment id once the upload is done", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const src = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(src));

    const out = resolveInlineImageSources(img(src));
    expect(out).toBe(img(`/${IMAGE_ID.toLowerCase()}`));
    expect(hasBlobSource(out)).toBe(false);
  });

  it("uploads only the images the comment still shows", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const kept = queueInlineImage(file);
    const removed = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(kept));

    expect(upload).toHaveBeenCalledTimes(1);
    expect(inspectInlineImages(img(removed)).queued).toBe(1);
  });

  it("reports a failed upload with its message, never resolves it and does not try it again", async () => {
    upload.mockRejectedValue(new Error("Could not upload shot.png (HTTP 500). Please try again."));
    const src = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(src));

    expect(inspectInlineImages(img(src)).failedMessage).toContain("Could not upload shot.png");
    expect(hasBlobSource(resolveInlineImageSources(img(src)))).toBe(true);

    await uploadQueuedImages(api, CASE_ID, img(src));
    expect(upload).toHaveBeenCalledTimes(1);
  });

  it("goes on with the next image after one fails", async () => {
    upload.mockRejectedValueOnce(new Error("nope")).mockResolvedValue({ id: IMAGE_ID } as never);
    const first = queueInlineImage(file);
    const second = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(first) + img(second));

    expect(inspectInlineImages(img(first)).failedMessage).toBe("nope");
    expect(inspectInlineImages(img(second))).toEqual(none);
  });

  it("flags a blob: image it did not queue", () => {
    expect(inspectInlineImages(img("blob:https://csm.example.com/other")).unknown).toBe(1);
  });

  it("ignores images that are not local previews", () => {
    const html = img("/11111111-1111-4111-8111-111111111111") + img("data:image/png;base64,AAAA");
    expect(inspectInlineImages(html)).toEqual(none);
    expect(resolveInlineImageSources(html)).toBe(html);
  });
});

describe("dropFailedImages", () => {
  it("removes only the images whose upload failed", async () => {
    upload.mockRejectedValueOnce(new Error("nope"));
    const failed = queueInlineImage(file);
    const queued = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(failed));
    const html = `<p>a</p><img src="${failed}" alt="x"><img src='${queued}'>`;
    expect(dropFailedImages(html)).toBe(`<p>a</p><img src="${queued}">`);
  });

  it("removes the whole element when an attribute holds a >", async () => {
    upload.mockRejectedValueOnce(new Error("nope"));
    const failed = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(failed));
    const html = `<p>a</p><img alt="a>b" src="${failed}" title='x > y'><p>c</p>`;
    expect(dropFailedImages(html)).toBe("<p>a</p><p>c</p>");
  });

  it("keeps uploaded, unknown and ordinary images", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const done = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(done));
    const html = img(done) + img("blob:https://csm.example.com/other") + img("/11111111-1111-4111-8111-111111111111");
    expect(dropFailedImages(html)).toBe(html);
  });
});

describe("releaseInlineImages", () => {
  it("revokes the preview URLs and forgets them", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const src = queueInlineImage(file);
    await uploadQueuedImages(api, CASE_ID, img(src));

    releaseInlineImages(img(src));

    expect(URL.revokeObjectURL).toHaveBeenCalledWith(src);
    expect(inspectInlineImages(img(src)).unknown).toBe(1);
  });

  it("ignores an upload that finishes after its image was released", async () => {
    let finish: (v: { id: string }) => void = () => {};
    upload.mockReturnValue(new Promise((resolve) => (finish = resolve)) as never);
    const src = queueInlineImage(file);
    const running = uploadQueuedImages(api, CASE_ID, img(src));

    releaseInlineImages(img(src));
    finish({ id: IMAGE_ID });
    await running;

    // Still unknown: the late success did not bring the entry back.
    expect(inspectInlineImages(img(src))).toEqual({ ...none, unknown: 1 });
  });

  it("ignores an upload that fails after its image was released", async () => {
    let fail: (e: Error) => void = () => {};
    upload.mockReturnValue(new Promise((_, reject) => (fail = reject)) as never);
    const src = queueInlineImage(file);
    const running = uploadQueuedImages(api, CASE_ID, img(src));

    releaseInlineImages(img(src));
    fail(new Error("boom"));
    await running;

    expect(inspectInlineImages(img(src))).toEqual({ ...none, unknown: 1 });
  });
});
