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
import { uploadReportDrafts, type StoredReportDraft } from "@features/support/utils/reportDraftUploads";
import { UploadConfirmPendingError } from "@api/storedAttachments";

const upload = vi.fn();
const confirm = vi.fn();
vi.mock("@api/storedAttachments", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@api/storedAttachments")>()),
  uploadStoredAttachment: (...args: unknown[]) => upload(...args),
  confirmStoredUpload: (...args: unknown[]) => confirm(...args),
}));

const authFetch = vi.fn();
const file = (name: string) => new File(["x"], name, { type: "application/pdf" });
const files = [
  { key: "a", file: file("a.pdf"), name: "a.pdf" },
  { key: "b", file: file("b.pdf"), name: "b.pdf" },
];

beforeEach(() => {
  upload.mockReset();
  confirm.mockReset();
});

describe("uploadReportDrafts", () => {
  it("uploads each file as a draft of the project, in order, and returns the ids", async () => {
    upload.mockResolvedValueOnce({ id: "id-a" }).mockResolvedValueOnce({ id: "id-b" });
    const stored = new Map<string, StoredReportDraft>();
    const result = await uploadReportDrafts(authFetch, "proj-1", files, stored);
    expect(result).toEqual({ ok: true, ids: ["id-a", "id-b"] });
    expect(upload).toHaveBeenCalledTimes(2);
    expect(upload.mock.calls[0][1]).toMatchObject({ type: "work_item", projectId: "proj-1", name: "a.pdf" });
    expect(upload.mock.calls[0][1]).not.toHaveProperty("parentId");
  });

  it("does not send a file twice after a retry, only the one that did not finish", async () => {
    upload.mockResolvedValueOnce({ id: "id-a" }).mockRejectedValueOnce(new Error("boom"));
    const stored = new Map<string, StoredReportDraft>();
    const first = await uploadReportDrafts(authFetch, "proj-1", files, stored);
    expect(first).toEqual({ ok: false, cancelled: false, message: "boom" });

    upload.mockResolvedValueOnce({ id: "id-b" });
    const second = await uploadReportDrafts(authFetch, "proj-1", files, stored);
    expect(second).toEqual({ ok: true, ids: ["id-a", "id-b"] });
    expect(upload).toHaveBeenCalledTimes(3);
  });

  it("uploads again when the project changed", async () => {
    upload.mockResolvedValue({ id: "id-x" });
    const stored = new Map<string, StoredReportDraft>([["a", { projectId: "old", attachmentId: "id-old" }]]);
    await uploadReportDrafts(authFetch, "proj-1", [files[0]], stored);
    expect(upload).toHaveBeenCalledTimes(1);
  });

  it("finishes the upload without repeating it when only the confirm failed", async () => {
    upload.mockRejectedValueOnce(new UploadConfirmPendingError("work_item", "id-a", "confirm failed", 1));
    confirm.mockResolvedValueOnce({ id: "id-a" });
    const result = await uploadReportDrafts(authFetch, "proj-1", [files[0]], new Map());
    expect(result).toEqual({ ok: true, ids: ["id-a"] });
    expect(confirm.mock.calls[0][1]).toEqual({ type: "work_item", attachmentId: "id-a", chunks: 1 });
  });

  it("reports a cancel as cancelled, not as an error, and stops", async () => {
    const abort = new Error("aborted");
    abort.name = "AbortError";
    upload.mockRejectedValueOnce(abort);
    const result = await uploadReportDrafts(authFetch, "proj-1", files, new Map());
    expect(result).toMatchObject({ ok: false, cancelled: true });
    expect(upload).toHaveBeenCalledTimes(1);
  });

  it("reports progress for the file in flight and clears it at the end", async () => {
    upload.mockImplementationOnce(async (_f, input: { onProgress: (s: number, t: number) => void }) => {
      input.onProgress(50, 100);
      return { id: "id-a" };
    });
    const seen: Array<unknown> = [];
    await uploadReportDrafts(authFetch, "proj-1", [files[0]], new Map(), { onProgress: (p) => seen.push(p) });
    expect(seen).toEqual([
      { name: "a.pdf", percent: 0, waiting: 0 },
      { name: "a.pdf", percent: 50, waiting: 0 },
      null,
    ]);
  });
});
