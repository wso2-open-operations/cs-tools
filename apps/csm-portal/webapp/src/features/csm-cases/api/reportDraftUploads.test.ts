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
import { uploadReportDrafts, type StoredReportDraft } from "@features/csm-cases/api/reportDraftUploads";
import type { PendingAttachment } from "@components/attachments/pendingAttachment";

const upload = vi.fn();
// The real module reads the runtime API config at load, which vitest has none of.
vi.mock("@features/csm-cases/api/entityAttachments", () => ({
  uploadEntityAttachment: (...args: unknown[]) => upload(...args),
  entityAttachmentErrorMessage: (err: unknown, fallback: string) =>
    err instanceof Error && err.message ? err.message : fallback,
}));

const api = {} as BackendApi;
const pending = (name: string): PendingAttachment => {
  const raw = new File(["x"], name, { type: "application/pdf" });
  return { name, size: raw.size, raw };
};
const files = [pending("a.pdf"), pending("b.pdf")];

beforeEach(() => upload.mockReset());

describe("uploadReportDrafts", () => {
  it("uploads each file as a draft of the project, in order, and returns the ids", async () => {
    upload.mockResolvedValueOnce({ id: "id-a" }).mockResolvedValueOnce({ id: "id-b" });
    const result = await uploadReportDrafts(api, "proj-1", files, new Map());
    expect(result).toEqual({ ok: true, ids: ["id-a", "id-b"] });
    expect(upload.mock.calls[0][1]).toMatchObject({ type: "work_item", projectId: "proj-1", name: "a.pdf" });
    expect(upload.mock.calls[0][1]).not.toHaveProperty("parentId");
  });

  it("does not send a file twice after a retry, only the one that did not finish", async () => {
    const stored = new Map<File, StoredReportDraft>();
    upload.mockResolvedValueOnce({ id: "id-a" }).mockRejectedValueOnce(new Error("boom"));
    expect(await uploadReportDrafts(api, "proj-1", files, stored)).toEqual({
      ok: false,
      cancelled: false,
      message: "boom",
    });
    upload.mockResolvedValueOnce({ id: "id-b" });
    expect(await uploadReportDrafts(api, "proj-1", files, stored)).toEqual({
      ok: true,
      ids: ["id-a", "id-b"],
    });
    expect(upload).toHaveBeenCalledTimes(3);
  });

  it("uploads again when the project changed", async () => {
    upload.mockResolvedValue({ id: "id-x" });
    const stored = new Map<File, StoredReportDraft>([
      [files[0].raw, { projectId: "old", attachmentId: "id-old" }],
    ]);
    await uploadReportDrafts(api, "proj-1", [files[0]], stored);
    expect(upload).toHaveBeenCalledTimes(1);
  });

  it("reports a cancel as cancelled, not as an error, and stops", async () => {
    const abort = new Error("aborted");
    abort.name = "AbortError";
    upload.mockRejectedValueOnce(abort);
    const result = await uploadReportDrafts(api, "proj-1", files, new Map());
    expect(result).toMatchObject({ ok: false, cancelled: true });
    expect(upload).toHaveBeenCalledTimes(1);
  });

  it("reports progress for the file in flight and clears it at the end", async () => {
    upload.mockImplementationOnce(async (_api, args: { onProgress: (p: number) => void }) => {
      args.onProgress(50);
      return { id: "id-a" };
    });
    const seen: unknown[] = [];
    await uploadReportDrafts(api, "proj-1", [files[0]], new Map(), { onProgress: (p) => seen.push(p) });
    expect(seen).toEqual([
      { name: "a.pdf", percent: 0, waiting: 0 },
      { name: "a.pdf", percent: 50, waiting: 0 },
      null,
    ]);
  });
});
