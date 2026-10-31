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
  hasBlobSource,
  inspectInlineImages,
  releaseInlineImages,
  resolveInlineImageSources,
  startInlineImageUpload,
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

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));
const img = (src: string) => `<p>hi</p><img src="${src}" alt="x">`;

describe("startInlineImageUpload", () => {
  it("uploads the file as a comment attachment of the case and returns a local preview URL", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const src = startInlineImageUpload(api, CASE_ID, file);

    expect(src).toBe("blob:https://csm.example.com/1");
    expect(upload).toHaveBeenCalledWith(api, {
      type: "comment",
      parentId: CASE_ID,
      file,
      name: "shot.png",
    });
    expect(inspectInlineImages(img(src))).toEqual({ pending: 1, failedMessage: null, unknown: 0 });
    await flush();
    expect(inspectInlineImages(img(src))).toEqual({ pending: 0, failedMessage: null, unknown: 0 });
  });

  it("swaps the preview URL for the lowercase attachment id once the upload is done", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const src = startInlineImageUpload(api, CASE_ID, file);
    await flush();

    const out = resolveInlineImageSources(img(src));
    expect(out).toBe(img(`/${IMAGE_ID.toLowerCase()}`));
    expect(hasBlobSource(out)).toBe(false);
  });

  it("reports a failed upload with its message and never resolves it", async () => {
    upload.mockRejectedValue(new Error("Could not upload shot.png (HTTP 500). Please try again."));
    const src = startInlineImageUpload(api, CASE_ID, file);
    await flush();

    expect(inspectInlineImages(img(src)).failedMessage).toContain("Could not upload shot.png");
    expect(hasBlobSource(resolveInlineImageSources(img(src)))).toBe(true);
  });

  it("flags a blob: image it did not upload", () => {
    expect(inspectInlineImages(img("blob:https://csm.example.com/other")).unknown).toBe(1);
  });

  it("ignores images that are not local previews", () => {
    const html = img("/11111111-1111-4111-8111-111111111111") + img("data:image/png;base64,AAAA");
    expect(inspectInlineImages(html)).toEqual({ pending: 0, failedMessage: null, unknown: 0 });
    expect(resolveInlineImageSources(html)).toBe(html);
  });
});

describe("releaseInlineImages", () => {
  it("revokes the preview URLs and forgets them", async () => {
    upload.mockResolvedValue({ id: IMAGE_ID } as never);
    const src = startInlineImageUpload(api, CASE_ID, file);
    await flush();

    releaseInlineImages(img(src));

    expect(URL.revokeObjectURL).toHaveBeenCalledWith(src);
    expect(inspectInlineImages(img(src)).unknown).toBe(1);
  });

  it("ignores an upload that finishes after its image was released", async () => {
    let finish: (v: { id: string }) => void = () => {};
    upload.mockReturnValue(new Promise((resolve) => (finish = resolve)) as never);
    const src = startInlineImageUpload(api, CASE_ID, file);

    releaseInlineImages(img(src));
    finish({ id: IMAGE_ID });
    await flush();

    // Still unknown: the late success did not bring the entry back.
    expect(inspectInlineImages(img(src))).toEqual({ pending: 0, failedMessage: null, unknown: 1 });
  });

  it("ignores an upload that fails after its image was released", async () => {
    let fail: (e: Error) => void = () => {};
    upload.mockReturnValue(new Promise((_, reject) => (fail = reject)) as never);
    const src = startInlineImageUpload(api, CASE_ID, file);

    releaseInlineImages(img(src));
    fail(new Error("boom"));
    await flush();

    expect(inspectInlineImages(img(src))).toEqual({ pending: 0, failedMessage: null, unknown: 1 });
  });
});
