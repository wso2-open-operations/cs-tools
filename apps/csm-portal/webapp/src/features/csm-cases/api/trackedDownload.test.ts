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
import type { BackendApi } from "@api/backend/client";

const downloadEntityAttachment = vi.fn();
vi.mock("@features/csm-cases/api/entityAttachments", () => ({
  downloadEntityAttachment: (...args: unknown[]) => downloadEntityAttachment(...args),
}));

import {
  cancelAttachmentDownload,
  isAttachmentDownloading,
} from "@features/csm-cases/api/attachmentDownloadProgress";
import { downloadWithProgress } from "@features/csm-cases/api/trackedDownload";

const api = {} as BackendApi;
const args = { type: "work_item" as const, id: "att-1", name: "a.bin", size: 10 };

describe("downloadWithProgress", () => {
  // Braces matter: a function returned from beforeEach is run as a cleanup after the test.
  beforeEach(() => {
    downloadEntityAttachment.mockReset();
  });

  it("tracks the download in the shared store and clears it when done", async () => {
    let during: boolean | undefined;
    downloadEntityAttachment.mockImplementation(async () => {
      during = isAttachmentDownloading("att-1");
    });
    await downloadWithProgress(api, args);
    expect(during).toBe(true);
    expect(isAttachmentDownloading("att-1")).toBe(false);
  });

  it("is cancelled by cancelAttachmentDownload, quietly and with the store cleared", async () => {
    downloadEntityAttachment.mockImplementation((_api: unknown, input: { signal: AbortSignal }) => {
      return new Promise((_resolve, reject) => {
        input.signal.addEventListener("abort", () => {
          const error = new Error("The download was cancelled.");
          error.name = "AbortError";
          reject(error);
        });
      });
    });
    const running = downloadWithProgress(api, args);
    expect(isAttachmentDownloading("att-1")).toBe(true);
    cancelAttachmentDownload("att-1");
    await expect(running).resolves.toBeUndefined();
    expect(isAttachmentDownloading("att-1")).toBe(false);
  });

  it("still throws a real failure, and clears the store", async () => {
    downloadEntityAttachment.mockRejectedValue(new Error("boom"));
    await expect(downloadWithProgress(api, args)).rejects.toThrow("boom");
    expect(isAttachmentDownloading("att-1")).toBe(false);
  });

  it("ignores a second call while the attachment is already downloading", async () => {
    let release!: () => void;
    downloadEntityAttachment.mockImplementation(() => new Promise<void>((r) => (release = r)));
    const first = downloadWithProgress(api, args);
    await downloadWithProgress(api, args);
    expect(downloadEntityAttachment).toHaveBeenCalledTimes(1);
    release();
    await first;
  });
});
