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
  DOWNLOAD_RANGE_BYTES,
  downloadStoredAttachment,
  fetchInlineImageBlob,
  requestAttachmentLink,
  sendFileToUploadLink,
  toHyphenatedUuid,
  uploadStoredAttachment,
  UploadConfirmPendingError,
} from "@api/storedAttachments";
import { ApiError } from "@utils/ApiError";
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

  it("requests one link with the type, the id and the purpose", async () => {
    authFetch.mockResolvedValueOnce(
      json({ links: [{ id: "A-1", available: true, url: "https://f/x" }] }),
    );
    const link = await requestAttachmentLink(authFetch, {
      type: "work_item",
      id: "a-1",
      purpose: "preview",
    });
    expect(link?.url).toBe("https://f/x");
    const [url, init] = authFetch.mock.calls[0];
    expect(url).toBe(`${BASE}/attachments/download`);
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({
      type: "work_item",
      ids: ["a-1"],
      purpose: "preview",
    });
  });

  describe("downloadStoredAttachment", () => {
    let saved: { name: string; blob: Blob } | null;
    beforeEach(() => {
      saved = null;
      URL.createObjectURL = vi.fn(() => "blob:test");
      URL.revokeObjectURL = vi.fn();
      vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(
        function (this: HTMLAnchorElement) {
          saved = { name: this.download, blob: new Blob() };
        },
      );
    });

    it("fetches the link without credentials and saves it under the attachment's own name", async () => {
      authFetch.mockResolvedValueOnce(
        json({
          links: [
            {
              id: "a-1",
              name: "report.pdf",
              available: true,
              url: "https://files.test/api/v2/shares/s1?compress=false",
            },
          ],
        }),
      );
      rawFetch.mockResolvedValueOnce(new Response("pdf-bytes", { status: 200 }));

      await downloadStoredAttachment(authFetch, {
        type: "work_item",
        id: "a-1",
        name: "fallback.pdf",
      });

      expect(rawFetch).toHaveBeenCalledTimes(1);
      const [url, init] = rawFetch.mock.calls[0];
      expect(url).toBe("https://files.test/api/v2/shares/s1?compress=false");
      expect(init).toEqual({ credentials: "omit" });
      expect(saved?.name).toBe("report.pdf");
      expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
    });

    it("asks for one fresh link and retries once when the link fetch fails", async () => {
      const link = (u: string) =>
        json({ links: [{ id: "a-1", name: "r.pdf", available: true, url: u }] });
      authFetch
        .mockResolvedValueOnce(link("https://f/one"))
        .mockResolvedValueOnce(link("https://f/two"));
      rawFetch
        .mockResolvedValueOnce(new Response("gone", { status: 404 }))
        .mockResolvedValueOnce(new Response("ok", { status: 200 }));

      await downloadStoredAttachment(authFetch, { type: "work_item", id: "a-1" });

      expect(authFetch).toHaveBeenCalledTimes(2);
      expect(rawFetch.mock.calls.map((c) => c[0])).toEqual([
        "https://f/one",
        "https://f/two",
      ]);
      expect(saved?.name).toBe("r.pdf");
    });

    it("does not retry a second time", async () => {
      const link = (u: string) =>
        json({ links: [{ id: "a-1", available: true, url: u }] });
      authFetch
        .mockResolvedValueOnce(link("https://f/one"))
        .mockResolvedValueOnce(link("https://f/two"));
      rawFetch.mockResolvedValue(new Response("no", { status: 500 }));

      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: "a-1" }),
      ).rejects.toBeInstanceOf(ApiError);
      expect(authFetch).toHaveBeenCalledTimes(2);
      expect(rawFetch).toHaveBeenCalledTimes(2);
    });

    it("reports 404 when entity-service does not hold the attachment", async () => {
      authFetch.mockResolvedValueOnce(json({ links: [] }));
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: "a-1" }),
      ).rejects.toMatchObject({ status: 404 });
      expect(rawFetch).not.toHaveBeenCalled();
    });

    it("explains a link that is not available", async () => {
      authFetch.mockResolvedValueOnce(
        json({ links: [{ id: "a-1", available: false, reason: "DELETED" }] }),
      );
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: "a-1" }),
      ).rejects.toThrow("This attachment has been deleted.");
    });
  });

  it("sends the file as multipart form data in `filenames`, with no credentials or headers", async () => {
    rawFetch.mockResolvedValueOnce(new Response("{}", { status: 201 }));
    const file = new File(["hello"], "a.txt", { type: "text/plain" });

    await sendFileToUploadLink(
      { uploadUrl: "https://files.test/api/v2/shares/up1", fieldName: "filenames" },
      file,
    );

    const [url, init] = rawFetch.mock.calls[0];
    expect(url).toBe("https://files.test/api/v2/shares/up1");
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("omit");
    expect(init.headers).toBeUndefined();
    expect(init.body).toBeInstanceOf(FormData);
    const entries = [...(init.body as FormData).entries()];
    expect(entries).toHaveLength(1);
    expect(entries[0][0]).toBe("filenames");
    expect((entries[0][1] as File).name).toBe("a.txt");
  });

  describe("uploadStoredAttachment", () => {
    const file = new File(["hello"], "a.txt", { type: "text/plain" });
    const started = {
      attachmentId: "att-1",
      uploadUrl: "https://files.test/api/v2/shares/up1",
      fieldName: "filenames",
      maxBytes: entityAttachmentMaxBytes(),
    };

    it("runs start, send and confirm in order and reports the steps", async () => {
      authFetch
        .mockResolvedValueOnce(json(started, 201))
        .mockResolvedValueOnce(
          json({ id: "att-1", name: "a.txt", sizeBytes: 5, status: "COMPLETE" }),
        );
      rawFetch.mockResolvedValueOnce(new Response("{}", { status: 201 }));
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
      expect(authFetch.mock.calls[1][0]).toBe(
        `${BASE}/attachments/upload/att-1/confirm`,
      );
    });

    it("keeps the attachment id when only the confirm step fails, so confirm can be retried", async () => {
      authFetch
        .mockResolvedValueOnce(json(started, 201))
        .mockResolvedValueOnce(json({ message: "storage unavailable" }, 503));
      rawFetch.mockResolvedValueOnce(new Response("{}", { status: 201 }));

      const error = await uploadStoredAttachment(authFetch, {
        type: "work_item",
        parentId: "case-1",
        file,
        name: "a.txt",
      }).catch((e: unknown) => e);

      expect(error).toBeInstanceOf(UploadConfirmPendingError);
      expect((error as UploadConfirmPendingError).attachmentId).toBe("att-1");
      expect(rawFetch).toHaveBeenCalledTimes(1);

      authFetch.mockResolvedValueOnce(
        json({ id: "att-1", name: "a.txt", sizeBytes: 5, status: "COMPLETE" }),
      );
      const retried = await confirmStoredUpload(authFetch, {
        type: "work_item",
        attachmentId: "att-1",
      });
      expect(retried.status).toBe("COMPLETE");
      expect(rawFetch).toHaveBeenCalledTimes(1);
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
      expect(rawFetch).not.toHaveBeenCalled();
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

  describe("with the files held by the portal backend", () => {
    const CHUNK = 16 * 1024 * 1024;
    const withMode = () => {
      (window as unknown as { config: unknown }).config = {
        CUSTOMER_PORTAL_BACKEND_BASE_URL: `${BASE}/`,
        CUSTOMER_PORTAL_ATTACHMENT_STORAGE_MODE: "backend",
      };
    };
    const started = { attachmentId: "att-1", maxBytes: 100 * 1024 * 1024 };
    const bigFile = (size: number) =>
      ({
        size,
        name: "big.bin",
        type: "application/octet-stream",
        slice: (from: number, to: number) => new Blob([new Uint8Array(to - from)]),
      }) as unknown as File;

    it("sends the file in 16 MiB chunks, then confirms with the chunk count", async () => {
      withMode();
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
      withMode();
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
      withMode();
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
      withMode();
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

    const ID = "11111111-2222-3333-4444-555555555555";
    const stubSave = () => {
      const saved: Blob[] = [];
      const names: string[] = [];
      vi.stubGlobal("URL", {
        ...URL,
        createObjectURL: (blob: Blob) => {
          saved.push(blob);
          return "blob:x";
        },
        revokeObjectURL: vi.fn(),
      });
      const click = vi
        .spyOn(HTMLAnchorElement.prototype, "click")
        .mockImplementation(function (this: HTMLAnchorElement) {
          names.push(this.download);
        });
      return { saved, names, click };
    };
    const readBytes = (blob: Blob) =>
      new Promise<Uint8Array>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(new Uint8Array(reader.result as ArrayBuffer));
        reader.onerror = () => reject(reader.error);
        reader.readAsArrayBuffer(blob);
      });

    /** A file of `size` bytes where byte i is the letter for floor(i / RANGE), served by Range. */
    const rangeServer = (size: number, delay: (start: number) => number = () => 0) => {
      const byteAt = (i: number) => 65 + Math.floor(i / DOWNLOAD_RANGE_BYTES);
      let inFlight = 0;
      let peak = 0;
      const fetchRange = vi.fn(async (_url: string, init?: { headers?: Record<string, string> }) => {
        const match = /bytes=(\d+)-(\d+)/.exec(init?.headers?.Range ?? "");
        const start = Number(match?.[1] ?? 0);
        const end = Math.min(Number(match?.[2] ?? size - 1), size - 1);
        inFlight += 1;
        peak = Math.max(peak, inFlight);
        await new Promise((r) => setTimeout(r, delay(start)));
        inFlight -= 1;
        const body = new Uint8Array(end - start + 1).map((_, i) => byteAt(start + i));
        return new Response(body, {
          status: 206,
          headers: {
            "Content-Range": `bytes ${start}-${end}/${size}`,
            "Content-Disposition": 'attachment; filename="big.bin"',
          },
        });
      });
      return { fetchRange, peak: () => peak };
    };
    const secondRange = (init?: { headers?: Record<string, string> }) =>
      init?.headers?.Range?.startsWith(`bytes=${DOWNLOAD_RANGE_BYTES}-`) === true;

    it("downloads through the backend route and saves it under the given name", async () => {
      withMode();
      const { saved, names, click } = stubSave();
      authFetch.mockResolvedValueOnce(
        new Response("0123456789", {
          status: 206,
          headers: { "Content-Range": "bytes 0-9/10" },
        }),
      );
      await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "report.pdf" });
      expect(authFetch).toHaveBeenCalledTimes(1);
      expect(authFetch.mock.calls[0][0]).toBe(
        `${BASE}/attachments/download/${ID}/content?type=work_item`,
      );
      expect(authFetch.mock.calls[0][1].headers).toEqual({
        Range: `bytes=0-${DOWNLOAD_RANGE_BYTES - 1}`,
      });
      expect(saved).toHaveLength(1);
      expect(names).toEqual(["report.pdf"]);
      expect(rawFetch).not.toHaveBeenCalled();
      click.mockRestore();
    });

    it("takes the file name from the response when none is given", async () => {
      withMode();
      const { names, click } = stubSave();
      authFetch.mockResolvedValueOnce(
        new Response("0123456789", {
          status: 200,
          headers: { "Content-Disposition": 'attachment; filename="from-server.txt"' },
        }),
      );
      await downloadStoredAttachment(authFetch, { type: "work_item", id: ID });
      expect(names).toEqual(["from-server.txt"]);
      click.mockRestore();
    });

    it("reads the whole body when the server ignores the Range header", async () => {
      withMode();
      const { saved, click } = stubSave();
      authFetch.mockResolvedValueOnce(new Response("0123456789", { status: 200 }));
      await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "a.txt" });
      expect(authFetch).toHaveBeenCalledTimes(1);
      expect(saved[0].size).toBe(10);
      click.mockRestore();
    });

    it("fetches a large file as ranges and joins them in order, whatever order they finish in", async () => {
      withMode();
      const size = DOWNLOAD_RANGE_BYTES * 3 + 5;
      // The later ranges answer first.
      const server = rangeServer(size, (start) =>
        start === 0 ? 0 : 40 - (start / DOWNLOAD_RANGE_BYTES) * 10,
      );
      authFetch.mockImplementation(server.fetchRange);
      const { saved, names, click } = stubSave();
      await downloadStoredAttachment(authFetch, { type: "work_item", id: ID });
      expect(server.fetchRange).toHaveBeenCalledTimes(4);
      expect(saved[0].size).toBe(size);
      const bytes = await readBytes(saved[0]);
      for (const [i, letter] of [
        [0, 65],
        [DOWNLOAD_RANGE_BYTES, 66],
        [DOWNLOAD_RANGE_BYTES * 2, 67],
        [size - 1, 68],
      ]) {
        expect(bytes[i]).toBe(letter);
      }
      expect(names).toEqual(["big.bin"]);
      click.mockRestore();
    });

    it("never fetches more ranges at once than the configured parallelism", async () => {
      withMode();
      const size = DOWNLOAD_RANGE_BYTES * 6;
      const { click } = stubSave();
      for (const [configured, expected] of [
        [undefined, 4],
        [2, 2],
        ["1", 1],
      ] as const) {
        (window as unknown as { config: unknown }).config = {
          CUSTOMER_PORTAL_BACKEND_BASE_URL: `${BASE}/`,
          CUSTOMER_PORTAL_ATTACHMENT_STORAGE_MODE: "backend",
          CUSTOMER_PORTAL_DOWNLOAD_PARALLELISM: configured,
        };
        const server = rangeServer(size, () => 5);
        authFetch.mockReset();
        authFetch.mockImplementation(server.fetchRange);
        await downloadStoredAttachment(authFetch, { type: "work_item", id: ID });
        expect(server.fetchRange).toHaveBeenCalledTimes(6);
        expect(server.peak()).toBeLessThanOrEqual(expected);
        if (expected > 1) expect(server.peak()).toBeGreaterThan(1);
      }
      click.mockRestore();
    });

    it("fetches a range again when its connection breaks or its body is short", async () => {
      withMode();
      const size = DOWNLOAD_RANGE_BYTES + 3;
      const server = rangeServer(size);
      let failures = 0;
      authFetch.mockImplementation(async (url: string, init?: { headers?: Record<string, string> }) => {
        if (secondRange(init) && failures === 0) {
          failures += 1;
          throw new TypeError("network error");
        }
        if (secondRange(init) && failures === 1) {
          failures += 1;
          return new Response(new Uint8Array(1), {
            status: 206,
            headers: { "Content-Range": `bytes ${DOWNLOAD_RANGE_BYTES}-${size - 1}/${size}` },
          });
        }
        return server.fetchRange(url, init);
      });
      const { saved, click } = stubSave();
      await downloadStoredAttachment(authFetch, { type: "work_item", id: ID });
      expect(authFetch).toHaveBeenCalledTimes(4); // first range, a break, a short body, the good one
      expect(saved[0].size).toBe(size);
      click.mockRestore();
    });

    it("gives up with an interruption error after three failed attempts at a range", async () => {
      withMode();
      const size = DOWNLOAD_RANGE_BYTES + 3;
      const server = rangeServer(size);
      authFetch.mockImplementation(async (url: string, init?: { headers?: Record<string, string> }) => {
        if (secondRange(init)) throw new TypeError("network error");
        return server.fetchRange(url, init);
      });
      const { saved, click } = stubSave();
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: ID }),
      ).rejects.toMatchObject({ message: "The download was interrupted. Please try again." });
      expect(authFetch).toHaveBeenCalledTimes(1 + 3);
      expect(saved).toHaveLength(0);
      click.mockRestore();
    });

    it("rejects a first range that is shorter than the file allows, even when it agrees with itself", async () => {
      withMode();
      const size = DOWNLOAD_RANGE_BYTES + 3;
      authFetch.mockImplementation(
        async () =>
          new Response(new Uint8Array(10), {
            status: 206,
            headers: { "Content-Range": `bytes 0-9/${size}` },
          }),
      );
      const { saved, click } = stubSave();
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: ID }),
      ).rejects.toMatchObject({ message: "The download was interrupted. Please try again." });
      expect(saved).toHaveLength(0);
      click.mockRestore();
    });

    it("rejects a later range whose Content-Range is not the one asked for", async () => {
      withMode();
      const size = DOWNLOAD_RANGE_BYTES + 3;
      const server = rangeServer(size);
      authFetch.mockImplementation(async (url: string, init?: { headers?: Record<string, string> }) => {
        if (secondRange(init)) {
          // The right number of bytes, but from the wrong place.
          return new Response(new Uint8Array(3), {
            status: 206,
            headers: { "Content-Range": `bytes 0-2/${size}` },
          });
        }
        return server.fetchRange(url, init);
      });
      const { saved, click } = stubSave();
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: ID }),
      ).rejects.toMatchObject({ message: "The download was interrupted. Please try again." });
      expect(saved).toHaveLength(0);
      click.mockRestore();
    });

    it("stops at the first refusal of a later range, with the backend's message", async () => {
      withMode();
      const size = DOWNLOAD_RANGE_BYTES * 5;
      const server = rangeServer(size, () => 5);
      authFetch.mockImplementation(async (url: string, init?: { headers?: Record<string, string> }) => {
        if (secondRange(init)) return json({ message: "This attachment was removed." }, 410);
        return server.fetchRange(url, init);
      });
      const { saved, click } = stubSave();
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: ID }),
      ).rejects.toMatchObject({ status: 410, message: "This attachment was removed." });
      expect(authFetch.mock.calls.length).toBeLessThan(6); // ranges not yet started never are
      expect(saved).toHaveLength(0);
      click.mockRestore();
    });

    it("does not retry a cancelled download", async () => {
      withMode();
      authFetch.mockRejectedValue(Object.assign(new Error("aborted"), { name: "AbortError" }));
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: ID }),
      ).rejects.toMatchObject({ name: "AbortError" });
      expect(authFetch).toHaveBeenCalledTimes(1);
    });

    it("reports 404 so the older flow can be tried, and 410 for a removed file", async () => {
      withMode();
      authFetch.mockResolvedValueOnce(json({ message: "Not found" }, 404));
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: "a" }),
      ).rejects.toMatchObject({ status: 404 });
      authFetch.mockResolvedValueOnce(json({ message: "This attachment was removed." }, 410));
      await expect(
        downloadStoredAttachment(authFetch, { type: "work_item", id: "a" }),
      ).rejects.toMatchObject({ status: 410 });
    });

    describe("straight to disk", () => {
      const readFirstByte = async (blob: Blob) => (await readBytes(blob.slice(0, 1)))[0];

      /** A fake save dialog whose file records every write. */
      const fakeDisk = () => {
        const writes: { position: number; size: number; first: number }[] = [];
        const writable = {
          write: vi.fn(async ({ position, data }: { position: number; data: Blob }) => {
            writes.push({ position, size: data.size, first: await readFirstByte(data) });
          }),
          close: vi.fn(async () => undefined),
          abort: vi.fn(async () => undefined),
        };
        const picker = vi.fn<(options?: { suggestedName?: string }) => Promise<{ createWritable: () => Promise<typeof writable> }>>(async () => ({
          createWritable: vi.fn(async () => writable),
        }));
        (window as unknown as { showSaveFilePicker?: unknown }).showSaveFilePicker = picker;
        return { picker, writable, writes };
      };

      afterEach(() => {
        delete (window as unknown as { showSaveFilePicker?: unknown }).showSaveFilePicker;
      });

      it("asks where to save before the first request and writes each range at its own offset", async () => {
        withMode();
        const disk = fakeDisk();
        const { saved, click } = stubSave();
        const size = DOWNLOAD_RANGE_BYTES * 3 + 5;
        // The later ranges answer first.
        const server = rangeServer(size, (start) =>
          start === 0 ? 0 : 40 - (start / DOWNLOAD_RANGE_BYTES) * 10,
        );
        authFetch.mockImplementation(server.fetchRange);
        await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "big.bin" });
        expect(disk.picker).toHaveBeenCalledWith({ suggestedName: "big.bin" });
        expect(disk.picker.mock.invocationCallOrder[0]).toBeLessThan(
          authFetch.mock.invocationCallOrder[0],
        );
        expect([...disk.writes].sort((a, b) => a.position - b.position)).toEqual([
          { position: 0, size: DOWNLOAD_RANGE_BYTES, first: 65 },
          { position: DOWNLOAD_RANGE_BYTES, size: DOWNLOAD_RANGE_BYTES, first: 66 },
          { position: DOWNLOAD_RANGE_BYTES * 2, size: DOWNLOAD_RANGE_BYTES, first: 67 },
          { position: DOWNLOAD_RANGE_BYTES * 3, size: 5, first: 68 },
        ]);
        expect(disk.writable.close).toHaveBeenCalledTimes(1);
        expect(disk.writable.abort).not.toHaveBeenCalled();
        expect(saved).toHaveLength(0); // nothing is held for a Blob
        click.mockRestore();
      });

      it("drops the half-written file when a range is refused", async () => {
        withMode();
        const disk = fakeDisk();
        const { saved, click } = stubSave();
        const server = rangeServer(DOWNLOAD_RANGE_BYTES * 2 + 5);
        authFetch.mockImplementation(async (url: string, init?: { headers?: Record<string, string> }) => {
          if (secondRange(init)) return json({ message: "Not allowed." }, 403);
          return server.fetchRange(url, init);
        });
        await expect(
          downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "big.bin" }),
        ).rejects.toMatchObject({ status: 403, message: "Not allowed." });
        expect(disk.writable.abort).toHaveBeenCalledTimes(1);
        expect(disk.writable.close).not.toHaveBeenCalled();
        expect(saved).toHaveLength(0);
        click.mockRestore();
      });

      it("does nothing, and shows no error, when the save dialog is closed", async () => {
        withMode();
        const disk = fakeDisk();
        disk.picker.mockRejectedValue(new DOMException("The user aborted a request.", "AbortError"));
        const { saved, click } = stubSave();
        await expect(
          downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "big.bin" }),
        ).resolves.toBeUndefined();
        expect(authFetch).not.toHaveBeenCalled();
        expect(saved).toHaveLength(0);
        click.mockRestore();
      });

      it("uses the in-memory save when the dialog is refused for another reason", async () => {
        withMode();
        const disk = fakeDisk();
        disk.picker.mockRejectedValue(new DOMException("Needs a click.", "SecurityError"));
        const { saved, click } = stubSave();
        const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
        authFetch.mockImplementation(server.fetchRange);
        await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "big.bin" });
        expect(saved).toHaveLength(1);
        expect(saved[0].size).toBe(DOWNLOAD_RANGE_BYTES + 5);
        click.mockRestore();
      });

      it("uses the in-memory save when the browser has no save dialog", async () => {
        withMode();
        const { saved, click } = stubSave();
        const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
        authFetch.mockImplementation(server.fetchRange);
        await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "big.bin" });
        expect(saved).toHaveLength(1);
        click.mockRestore();
      });

      it("skips the dialog for a small file whose size is known", async () => {
        withMode();
        const disk = fakeDisk();
        const { saved, click } = stubSave();
        authFetch.mockResolvedValueOnce(
          new Response("0123456789", { status: 206, headers: { "Content-Range": "bytes 0-9/10" } }),
        );
        await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "s.txt", size: 10 });
        expect(disk.picker).not.toHaveBeenCalled();
        expect(saved).toHaveLength(1);
        click.mockRestore();
      });

      it("asks for a save location when the size is not known", async () => {
        withMode();
        const disk = fakeDisk();
        authFetch.mockResolvedValueOnce(
          new Response("0123456789", { status: 206, headers: { "Content-Range": "bytes 0-9/10" } }),
        );
        await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "s.txt" });
        expect(disk.picker).toHaveBeenCalledTimes(1);
        expect(disk.writes).toEqual([{ position: 0, size: 10, first: 48 }]);
      });

      it("skips the dialog when CUSTOMER_PORTAL_DOWNLOAD_TO_DISK is false", async () => {
        (window as unknown as { config: unknown }).config = {
          CUSTOMER_PORTAL_BACKEND_BASE_URL: `${BASE}/`,
          CUSTOMER_PORTAL_ATTACHMENT_STORAGE_MODE: "backend",
          CUSTOMER_PORTAL_DOWNLOAD_TO_DISK: false,
        };
        const disk = fakeDisk();
        const { saved, click } = stubSave();
        const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
        authFetch.mockImplementation(server.fetchRange);
        await downloadStoredAttachment(authFetch, { type: "work_item", id: ID, name: "big.bin" });
        expect(disk.picker).not.toHaveBeenCalled();
        expect(saved).toHaveLength(1);
        click.mockRestore();
      });
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
      requestAttachmentLink(authFetch, { type: "work_item", id: "a-1" }),
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
