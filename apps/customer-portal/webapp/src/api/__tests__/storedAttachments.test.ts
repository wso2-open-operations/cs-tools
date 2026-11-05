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
import {
  confirmStoredUpload,
  deleteStoredAttachment,
  downloadStoredAttachment,
  fetchInlineImageBlob,
  toHyphenatedUuid,
  uploadStoredAttachment,
  UploadConfirmPendingError,
} from "@api/storedAttachments";
import {
  entityAttachmentMaxBytes,
  inlineImageMaxBytes,
} from "@config/attachmentLimitsConfig";

const BASE = "https://api.test";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("storedAttachments", () => {
  const authFetch = vi.fn();
  const rawFetch = vi.fn();

  beforeEach(() => {
    authFetch.mockReset();
    rawFetch.mockReset();
    vi.stubGlobal("fetch", rawFetch);
    (window as unknown as { config: unknown }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: `${BASE}/`,
    };
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  describe("uploadStoredAttachment", () => {
    const file = new File(["hello"], "a.txt", { type: "text/plain" });
    const started = {
      attachmentId: "att-1",
      maxBytes: entityAttachmentMaxBytes(),
    };
    const confirmed = { id: "att-1", name: "a.txt", sizeBytes: 5, status: "COMPLETE" };
    const chunkCalls = () =>
      authFetch.mock.calls.filter(([u]) => String(u).includes("/chunks/"));

    it("runs start, send and confirm in order and reports the steps", async () => {
      authFetch.mockImplementation(async (url: string) => {
        if (url.endsWith("/attachments/upload")) return json(started, 201);
        if (url.includes("/chunks/")) return new Response(null, { status: 204 });
        return json(confirmed);
      });
      const steps: string[] = [];

      const done = await uploadStoredAttachment(authFetch, {
        type: "work_item",
        parentId: "case-1",
        file,
        name: "a.txt",
        onStep: (s) => steps.push(s),
      });

      expect(done.status).toBe("COMPLETE");
      expect(steps).toEqual(["preparing", "uploading", "finishing"]);
      const startBody = JSON.parse(authFetch.mock.calls[0][1].body);
      expect(startBody).toEqual({
        type: "work_item",
        parentId: "case-1",
        name: "a.txt",
        contentType: "text/plain",
        sizeBytes: 5,
      });
      expect(chunkCalls()).toHaveLength(1);
      const confirm = authFetch.mock.calls.find(([u]) => String(u).endsWith("/confirm"))!;
      expect(confirm[0]).toBe(`${BASE}/attachments/upload/att-1/confirm`);
      expect(JSON.parse(confirm[1].body)).toEqual({ type: "work_item", chunks: 1 });
    });

    it("keeps the attachment id when only the confirm step fails, so confirm can be retried", async () => {
      let confirms = 0;
      authFetch.mockImplementation(async (url: string) => {
        if (url.endsWith("/attachments/upload")) return json(started, 201);
        if (url.includes("/chunks/")) return new Response(null, { status: 204 });
        confirms++;
        return confirms === 1 ? json({ message: "storage unavailable" }, 503) : json(confirmed);
      });

      const error = await uploadStoredAttachment(authFetch, {
        type: "work_item",
        parentId: "case-1",
        file,
        name: "a.txt",
      }).catch((e: unknown) => e);

      expect(error).toBeInstanceOf(UploadConfirmPendingError);
      expect((error as UploadConfirmPendingError).attachmentId).toBe("att-1");
      expect(chunkCalls()).toHaveLength(1);

      const retried = await confirmStoredUpload(authFetch, {
        type: "work_item",
        attachmentId: "att-1",
        chunks: 1,
      });
      expect(retried.status).toBe("COMPLETE");
      expect(chunkCalls()).toHaveLength(1);
    });

    it("refuses a file over the limit before anything is sent", async () => {
      const big = { size: entityAttachmentMaxBytes() + 1, name: "big.bin", type: "" } as File;
      await expect(
        uploadStoredAttachment(authFetch, {
          type: "work_item",
          parentId: "case-1",
          file: big,
          name: "big.bin",
        }),
      ).rejects.toThrow("File size exceeds 100 MB limit.");
      expect(authFetch).not.toHaveBeenCalled();
    });

    it("uses the configured limits, and says the configured number", async () => {
      (window as unknown as { config?: unknown }).config = {
        CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: "25",
        CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: 4,
      };
      try {
        const bigFile = { size: 25 * 1024 * 1024 + 1, name: "b.bin", type: "" } as File;
        await expect(
          uploadStoredAttachment(authFetch, { type: "work_item", parentId: "case-1", file: bigFile, name: "b.bin" }),
        ).rejects.toThrow("File size exceeds 25 MB limit.");
        const bigImage = { size: 4 * 1024 * 1024 + 1, name: "p.png", type: "image/png" } as File;
        await expect(
          uploadStoredAttachment(authFetch, { type: "comment", parentId: "case-1", file: bigImage, name: "p.png" }),
        ).rejects.toThrow("File size exceeds 4 MB limit.");
        expect(authFetch).not.toHaveBeenCalled();
      } finally {
        (window as unknown as { config?: unknown }).config = undefined;
      }
    });

    it("refuses a comment image over the 10 MB default before anything is sent", async () => {
      const big = { size: inlineImageMaxBytes() + 1, name: "p.png", type: "image/png" } as File;
      await expect(
        uploadStoredAttachment(authFetch, {
          type: "comment",
          parentId: "case-1",
          file: big,
          name: "p.png",
        }),
      ).rejects.toThrow("File size exceeds 10 MB limit.");
      expect(authFetch).not.toHaveBeenCalled();
    });

    it("does not apply the image limit to a work item file (only the server's limit)", async () => {
      const mid = { size: inlineImageMaxBytes() * 6, name: "a.bin", type: "" } as File;
      authFetch.mockResolvedValueOnce(json({ ...started, maxBytes: 5 * 1024 * 1024 }, 201));
      await expect(
        uploadStoredAttachment(authFetch, {
          type: "work_item",
          parentId: "case-1",
          file: mid,
          name: "a.bin",
        }),
      ).rejects.toThrow(/exceeds the 5 MB limit/);
      expect(authFetch).toHaveBeenCalledTimes(1);
    });

    it("refuses a file over the server's maxBytes before sending it", async () => {
      authFetch.mockResolvedValueOnce(json({ ...started, maxBytes: 3 }, 201));
      await expect(
        uploadStoredAttachment(authFetch, {
          type: "work_item",
          parentId: "case-1",
          file,
          name: "a.txt",
        }),
      ).rejects.toThrow(/exceeds/);
      expect(authFetch).toHaveBeenCalledTimes(1);
    });

    it("surfaces the closed-case message from the server", async () => {
      authFetch.mockResolvedValueOnce(
        json({ message: "This case is closed, so new attachments can't be added." }, 409),
      );
      await expect(
        uploadStoredAttachment(authFetch, {
          type: "work_item",
          parentId: "case-1",
          file,
          name: "a.txt",
        }),
      ).rejects.toMatchObject({
        status: 409,
        message: "This case is closed, so new attachments can't be added.",
      });
    });
  });

  describe("chunked upload and streamed download", () => {
    const CHUNK = 16 * 1024 * 1024;
    const started = { attachmentId: "att-1", maxBytes: 100 * 1024 * 1024 };
    const bigFile = (size: number) =>
      ({
        size,
        name: "big.bin",
        type: "application/octet-stream",
        slice: (from: number, to: number) => new Blob([new Uint8Array(to - from)]),
      }) as unknown as File;

    it("sends the file in 16 MiB chunks, then confirms with the chunk count", async () => {
      authFetch.mockImplementation(async (url: string) => {
        if (url.endsWith("/attachments/upload")) return json(started, 201);
        if (url.includes("/chunks/")) return new Response(null, { status: 204 });
        return json({ id: "att-1", name: "big.bin", sizeBytes: 0, status: "COMPLETE" });
      });
      const progress: number[] = [];
      await uploadStoredAttachment(authFetch, {
        type: "work_item",
        parentId: "case-1",
        file: bigFile(2 * CHUNK + 5),
        name: "big.bin",
        onProgress: (sent) => progress.push(sent),
      });
      const chunkCalls = authFetch.mock.calls.filter(([u]) => String(u).includes("/chunks/"));
      expect(chunkCalls.map(([u]) => String(u).match(/chunks\/(\d+)/)?.[1]).sort()).toEqual(["0", "1", "2"]);
      expect(chunkCalls.every(([u, i]) => String(u).endsWith("?type=work_item") && i.method === "PUT")).toBe(true);
      expect(rawFetch).not.toHaveBeenCalled();
      const confirm = authFetch.mock.calls.find(([u]) => String(u).endsWith("/confirm"))!;
      expect(JSON.parse(confirm[1].body)).toEqual({ type: "work_item", chunks: 3 });
      expect(progress[progress.length - 1]).toBe(2 * CHUNK + 5);
    });

    it("tries a chunk again after a passing failure", async () => {
      vi.useFakeTimers();
      try {
        let tries = 0;
        authFetch.mockImplementation(async (url: string) => {
          if (url.endsWith("/attachments/upload")) return json(started, 201);
          if (url.endsWith("/confirm"))
            return json({ id: "att-1", name: "a.bin", sizeBytes: 10, status: "COMPLETE" });
          tries++;
          return tries === 1 ? new Response("", { status: 503 }) : new Response(null, { status: 204 });
        });
        const done = uploadStoredAttachment(authFetch, {
          type: "work_item",
          parentId: "case-1",
          file: bigFile(10),
          name: "a.bin",
        });
        await vi.runAllTimersAsync();
        await expect(done).resolves.toMatchObject({ status: "COMPLETE" });
        expect(tries).toBeGreaterThanOrEqual(2);
      } finally {
        vi.useRealTimers();
      }
    });

    it("stops taking new chunks after one fails, and throws that failure", async () => {
      const started10 = { attachmentId: "att-1", maxBytes: 100 * 1024 * 1024 };
      const sent: number[] = [];
      authFetch.mockImplementation(async (url: string) => {
        if (url.endsWith("/attachments/upload")) return json(started10, 201);
        const index = Number(String(url).match(/chunks\/(\d+)/)?.[1]);
        sent.push(index);
        if (index === 0)
          return json({ message: "Only the person who started the upload can send it." }, 403);
        // The others are still in flight when chunk 0 is refused.
        await new Promise((resolve) => setTimeout(resolve, 30));
        return new Response(null, { status: 204 });
      });
      await expect(
        uploadStoredAttachment(authFetch, {
          type: "work_item",
          parentId: "c",
          file: bigFile(6 * CHUNK),
          name: "a.bin",
        }),
      ).rejects.toThrow("Only the person who started the upload can send it.");
      // Three workers each took one chunk before the failure; none took a fourth.
      expect(sent.length).toBeLessThanOrEqual(3);
    });

    it("does not retry a refusal", async () => {
      let tries = 0;
      authFetch.mockImplementation(async (url: string) => {
        if (url.endsWith("/attachments/upload")) return json(started, 201);
        tries++;
        return json({ message: "Only the person who started the upload can send it." }, 403);
      });
      await expect(
        uploadStoredAttachment(authFetch, { type: "work_item", parentId: "c", file: bigFile(10), name: "a.bin" }),
      ).rejects.toThrow("Only the person who started the upload can send it.");
      expect(tries).toBe(1);
    });

    it("downloads through the backend route and saves it under the given name", async () => {
      const save = vi.fn();
      vi.stubGlobal("URL", { ...URL, createObjectURL: () => "blob:x", revokeObjectURL: vi.fn() });
      const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => save());
      authFetch.mockResolvedValueOnce(
        new Response("0123456789", { status: 200, headers: { "Content-Length": "10" } }),
      );
      await downloadStoredAttachment(authFetch, {
        type: "work_item",
        id: "11111111-2222-3333-4444-555555555555",
        name: "report.pdf",
      });
      expect(authFetch.mock.calls[0][0]).toBe(
        `${BASE}/attachments/download/11111111-2222-3333-4444-555555555555/content?type=work_item`,
      );
      expect(save).toHaveBeenCalledTimes(1);
      expect(rawFetch).not.toHaveBeenCalled();
      click.mockRestore();
    });

    it("reports 404 so the older flow can be tried, and 410 for a removed file", async () => {
      authFetch.mockResolvedValueOnce(json({ message: "Not found" }, 404));
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: "a" }),
      ).rejects.toMatchObject({ status: 404 });
      authFetch.mockResolvedValueOnce(json({ message: "This attachment was removed." }, 410));
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: "a" }),
      ).rejects.toMatchObject({ status: 410 });
    });
  });

  it("deletes with the type and id and surfaces a 403 message", async () => {
    authFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await deleteStoredAttachment(authFetch, { type: "deployment", id: "d-1" });
    expect(JSON.parse(authFetch.mock.calls[0][1].body)).toEqual({
      type: "deployment",
      id: "d-1",
    });
    expect(authFetch.mock.calls[0][0]).toBe(`${BASE}/attachments/delete`);

    authFetch.mockResolvedValueOnce(
      json({ message: "Only the person who uploaded an attachment can delete it." }, 403),
    );
    await expect(
      deleteStoredAttachment(authFetch, { type: "work_item", id: "a-1" }),
    ).rejects.toMatchObject({
      status: 403,
      message: "Only the person who uploaded an attachment can delete it.",
    });
  });

  it("tells the user to wait a minute on a 429", async () => {
    authFetch.mockResolvedValueOnce(json({ message: "slow down" }, 429));
    await expect(
      deleteStoredAttachment(authFetch, { type: "work_item", id: "a-1" }),
    ).rejects.toThrow(/wait a minute/);
  });

  it("fetchInlineImageBlob returns the blob, not-found for 404 and error otherwise", async () => {
    authFetch.mockResolvedValueOnce(new Response("IMG", { status: 200 }));
    const ok = await fetchInlineImageBlob(authFetch, "i-1");
    expect(ok.status).toBe("ok");
    expect(authFetch.mock.calls[0][0]).toBe(`${BASE}/attachments/inline/i-1/content`);
    authFetch.mockResolvedValueOnce(new Response("", { status: 404 }));
    expect(await fetchInlineImageBlob(authFetch, "i-1")).toEqual({ status: "not-found" });
    authFetch.mockResolvedValueOnce(new Response("", { status: 410 }));
    expect(await fetchInlineImageBlob(authFetch, "i-1")).toEqual({ status: "removed" });
    authFetch.mockResolvedValueOnce(new Response("", { status: 503 }));
    expect(await fetchInlineImageBlob(authFetch, "i-1")).toEqual({ status: "error" });
  });

  it("fetchInlineImageBlob asks for the hyphenated id even when given 32 hex characters", async () => {
    authFetch.mockResolvedValue(new Response("IMG", { status: 200 }));
    await fetchInlineImageBlob(authFetch, "0f15cbccc36b8310af2f404599013196");
    expect(authFetch.mock.calls[0][0]).toBe(
      `${BASE}/attachments/inline/0f15cbcc-c36b-8310-af2f-404599013196/content`,
    );
  });

  it("toHyphenatedUuid only reshapes a bare 32 hex id", () => {
    expect(toHyphenatedUuid("0F15CBCCC36B8310AF2F404599013196")).toBe(
      "0f15cbcc-c36b-8310-af2f-404599013196",
    );
    expect(toHyphenatedUuid("0f15cbcc-c36b-8310-af2f-404599013196")).toBe(
      "0f15cbcc-c36b-8310-af2f-404599013196",
    );
    expect(toHyphenatedUuid("i-1")).toBe("i-1");
  });
});
