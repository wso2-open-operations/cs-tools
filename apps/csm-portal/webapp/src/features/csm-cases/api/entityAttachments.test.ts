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

vi.mock("@api/backend/client", () => {
  class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  }
  return { BackendApiError, useBackendApi: vi.fn() };
});

import { BackendApiError, type BackendApi } from "@api/backend/client";
import {
  confirmEntityAttachment,
  deleteEntityAttachment,
  DOWNLOAD_RANGE_BYTES,
  downloadEntityAttachment,
  ENTITY_ATTACHMENT_RATE_LIMIT_MESSAGE,
  entityAttachmentErrorMessage,
  EntityAttachmentUnavailableError,
  EntityUploadConfirmError,
  UPLOAD_CHUNK_BYTES,
  UPLOAD_CANCELLED_MESSAGE,
  fetchEntityAttachmentBlob,
  fetchEntityInlineImage,
  requestDownloadLink,
  uploadEntityAttachment,
  type FetchLike,
} from "@features/csm-cases/api/entityAttachments";

const ID = "11111111-1111-4111-8111-111111111111";

function apiError(status: number, message = "x"): Error {
  return new (BackendApiError as unknown as new (s: number, m: string) => Error)(status, message);
}

function fakeApi(post: ReturnType<typeof vi.fn>, getBlob = vi.fn()): BackendApi {
  return { post, getBlob } as unknown as BackendApi;
}

function link(over: Record<string, unknown> = {}) {
  return {
    id: ID,
    number: "WIA0000001",
    name: "report.pdf",
    available: true,
    url: "https://files.test/api/v2/shares/abc?compress=false",
    ...over,
  };
}

const okFetch = (body = "bytes"): FetchLike =>
  vi.fn(async () => new Response(body, { status: 200 })) as unknown as FetchLike;

describe("requestDownloadLink", () => {
  it("asks for exactly one id with its purpose and finds it case-insensitively", async () => {
    const post = vi.fn().mockResolvedValue({ links: [link({ id: ID.toUpperCase() })] });
    const got = await requestDownloadLink(fakeApi(post), "work_item", ID, "preview");
    expect(post).toHaveBeenCalledWith("/attachments/download", {
      type: "work_item",
      ids: [ID],
      purpose: "preview",
    });
    expect(got?.name).toBe("report.pdf");
  });

  it("is undefined when the row is absent from the response", async () => {
    const post = vi.fn().mockResolvedValue({ links: [] });
    expect(await requestDownloadLink(fakeApi(post), "work_item", ID, "download")).toBeUndefined();
  });
});

describe("fetchEntityAttachmentBlob", () => {
  it("fetches the link with no credentials and returns the bytes", async () => {
    const post = vi.fn().mockResolvedValue({ links: [link()] });
    const fetchImpl = okFetch("hello");
    const got = await fetchEntityAttachmentBlob(
      fakeApi(post),
      { type: "work_item", id: ID, purpose: "download" },
      fetchImpl,
    );
    expect(fetchImpl).toHaveBeenCalledWith(
      "https://files.test/api/v2/shares/abc?compress=false",
      { method: "GET", credentials: "omit" },
    );
    expect(await got?.blob.text()).toBe("hello");
  });

  it("asks for a fresh link once when the first is refused, then succeeds", async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce({ links: [link({ url: "https://files.test/old" })] })
      .mockResolvedValueOnce({ links: [link({ url: "https://files.test/new" })] });
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce(new Response("gone", { status: 404 }))
      .mockResolvedValueOnce(new Response("ok", { status: 200 }));
    const got = await fetchEntityAttachmentBlob(
      fakeApi(post),
      { type: "work_item", id: ID, purpose: "download" },
      fetchImpl as unknown as FetchLike,
    );
    expect(post).toHaveBeenCalledTimes(2);
    expect(fetchImpl.mock.calls.map((c) => c[0])).toEqual([
      "https://files.test/old",
      "https://files.test/new",
    ]);
    expect(await got?.blob.text()).toBe("ok");
  });

  it("gives up after one retry", async () => {
    const post = vi.fn().mockResolvedValue({ links: [link()] });
    const fetchImpl = vi.fn(async () => new Response("no", { status: 403 }));
    await expect(
      fetchEntityAttachmentBlob(
        fakeApi(post),
        { type: "work_item", id: ID, purpose: "download" },
        fetchImpl as unknown as FetchLike,
      ),
    ).rejects.toThrow(/HTTP 403/);
    expect(post).toHaveBeenCalledTimes(2);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it("returns undefined for an unknown or not-stored attachment so the caller can fall back", async () => {
    for (const links of [[], [link({ available: false, url: undefined, reason: "NOT_STORED" })]]) {
      const post = vi.fn().mockResolvedValue({ links });
      const fetchImpl = vi.fn();
      expect(
        await fetchEntityAttachmentBlob(
          fakeApi(post),
          { type: "work_item", id: ID, purpose: "download" },
          fetchImpl as unknown as FetchLike,
        ),
      ).toBeUndefined();
      expect(fetchImpl).not.toHaveBeenCalled();
    }
  });

  it("throws for a deleted attachment instead of falling back", async () => {
    const post = vi.fn().mockResolvedValue({
      links: [link({ available: false, url: undefined, reason: "DELETED" })],
    });
    await expect(
      fetchEntityAttachmentBlob(fakeApi(post), { type: "work_item", id: ID, purpose: "download" }, okFetch()),
    ).rejects.toBeInstanceOf(EntityAttachmentUnavailableError);
  });
});

describe("downloadEntityAttachment", () => {
  it("saves the bytes under the attachment's own name, not the stored one", async () => {
    const post = vi.fn().mockResolvedValue({ links: [link({ name: "My Report.pdf" })] });
    const save = vi.fn();
    const done = await downloadEntityAttachment(fakeApi(post), { type: "work_item", id: ID }, okFetch(), save);
    expect(done).toBe(true);
    expect(save).toHaveBeenCalledTimes(1);
    expect(save.mock.calls[0][1]).toBe("My Report.pdf");
  });

  it("reports false (and saves nothing) when entity-service does not hold the file", async () => {
    const post = vi.fn().mockResolvedValue({ links: [] });
    const save = vi.fn();
    expect(await downloadEntityAttachment(fakeApi(post), { type: "work_item", id: ID }, okFetch(), save)).toBe(false);
    expect(save).not.toHaveBeenCalled();
  });
});

describe("uploadEntityAttachment", () => {
  const file = new File(["12345"], "report.pdf", { type: "application/pdf" });
  const start = {
    attachmentId: "att-1",
    number: "WIA0000001",
    uploadUrl: "https://files.test/api/v2/shares/up1",
    fieldName: "filenames",
    expiresAt: "2026-10-27T10:15:00Z",
    maxBytes: 100 * 1024 * 1024,
  };
  const confirmed = { id: "att-1", number: "WIA0000001", name: "report.pdf", sizeBytes: 5, status: "COMPLETE" };
  const args = { type: "work_item" as const, parentId: "wi-1", file };

  it("runs start, then a credential-free multipart post of one file in 'filenames', then confirm", async () => {
    const post = vi.fn().mockResolvedValueOnce(start).mockResolvedValueOnce(confirmed);
    const fetchImpl = vi.fn(async () => new Response("", { status: 201 }));
    const progress: number[] = [];

    const out = await uploadEntityAttachment(
      fakeApi(post),
      { ...args, onProgress: (p) => progress.push(p) },
      { fetchImpl: fetchImpl as unknown as FetchLike },
    );

    expect(out.status).toBe("COMPLETE");
    expect(post.mock.calls[0]).toEqual([
      "/attachments/upload",
      { type: "work_item", parentId: "wi-1", name: "report.pdf", contentType: "application/pdf", sizeBytes: 5 },
    ]);
    const [url, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(start.uploadUrl);
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("omit");
    expect(init.headers).toBeUndefined();
    const form = init.body as FormData;
    expect([...form.keys()]).toEqual(["filenames"]);
    expect((form.get("filenames") as File).name).toBe("report.pdf");
    expect((form.get("filenames") as File).size).toBe(5);
    expect(post.mock.calls[1]).toEqual(["/attachments/upload/att-1/confirm", { type: "work_item" }]);
    expect(progress[0]).toBe(0);
    expect(progress[progress.length - 1]).toBe(100);
  });

  it("refuses a file over the limit the server reports, before sending it", async () => {
    const post = vi.fn().mockResolvedValueOnce({ ...start, maxBytes: 3 });
    const fetchImpl = vi.fn();
    await expect(
      uploadEntityAttachment(fakeApi(post), args, { fetchImpl: fetchImpl as unknown as FetchLike }),
    ).rejects.toThrow(/too large/);
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it("stops without confirming when SFTPGo refuses the file", async () => {
    const post = vi.fn().mockResolvedValueOnce(start);
    const fetchImpl = vi.fn(async () => new Response("", { status: 413 }));
    await expect(
      uploadEntityAttachment(fakeApi(post), args, { fetchImpl: fetchImpl as unknown as FetchLike }),
    ).rejects.toThrow(/HTTP 413/);
    expect(post).toHaveBeenCalledTimes(1);
  });

  it("retries a 503 confirm without sending the file again", async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce(start)
      .mockRejectedValueOnce(apiError(503))
      .mockResolvedValueOnce(confirmed);
    const fetchImpl = vi.fn(async () => new Response("", { status: 201 }));
    const out = await uploadEntityAttachment(fakeApi(post), args, {
      fetchImpl: fetchImpl as unknown as FetchLike,
      confirmDelayMs: 0,
    });
    expect(out.status).toBe("COMPLETE");
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledTimes(3);
  });

  it("throws a resumable error when confirm keeps answering 503", async () => {
    const post = vi.fn().mockResolvedValueOnce(start).mockRejectedValue(apiError(503));
    const fetchImpl = vi.fn(async () => new Response("", { status: 201 }));
    const err = await uploadEntityAttachment(fakeApi(post), args, {
      fetchImpl: fetchImpl as unknown as FetchLike,
      confirmDelayMs: 0,
    }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(EntityUploadConfirmError);
    expect((err as EntityUploadConfirmError).attachmentId).toBe("att-1");
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it("surfaces a closed-case 409 from start untouched and sends nothing", async () => {
    const post = vi.fn().mockRejectedValueOnce(apiError(409, "This case is closed, so new attachments can't be added."));
    const fetchImpl = vi.fn();
    await expect(
      uploadEntityAttachment(fakeApi(post), args, { fetchImpl: fetchImpl as unknown as FetchLike }),
    ).rejects.toThrow("This case is closed, so new attachments can't be added.");
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it("does not retry a confirm that is refused for another reason", async () => {
    const post = vi.fn().mockResolvedValueOnce(start).mockRejectedValueOnce(apiError(409, "closed"));
    const fetchImpl = vi.fn(async () => new Response("", { status: 201 }));
    await expect(
      uploadEntityAttachment(fakeApi(post), args, { fetchImpl: fetchImpl as unknown as FetchLike, confirmDelayMs: 0 }),
    ).rejects.toThrow("closed");
    expect(post).toHaveBeenCalledTimes(2);
  });
});

describe("confirm, delete and inline image", () => {
  it("confirms by id with the type in the body", async () => {
    const post = vi.fn().mockResolvedValue({});
    await confirmEntityAttachment(fakeApi(post), "deployment", "a/b");
    expect(post).toHaveBeenCalledWith("/attachments/upload/a%2Fb/confirm", { type: "deployment" });
  });

  it("deletes with type and id", async () => {
    const post = vi.fn().mockResolvedValue(null);
    await deleteEntityAttachment(fakeApi(post), { type: "work_item", id: ID });
    expect(post).toHaveBeenCalledWith("/attachments/delete", { type: "work_item", id: ID });
  });

  it("reads a comment image from the streamed route through the authenticated client", async () => {
    const getBlob = vi.fn().mockResolvedValue(new Blob(["x"]));
    await fetchEntityInlineImage(fakeApi(vi.fn(), getBlob), ID);
    expect(getBlob).toHaveBeenCalledWith("/attachments/inline/" + ID + "/content");
  });
});

describe("entityAttachmentErrorMessage", () => {
  it("tells the user to wait a minute on a 429", () => {
    expect(entityAttachmentErrorMessage(apiError(429, "too many"), "f")).toBe(ENTITY_ATTACHMENT_RATE_LIMIT_MESSAGE);
  });
  it("otherwise uses the message, then the fallback", () => {
    expect(entityAttachmentErrorMessage(apiError(403, "Only the person who uploaded an attachment can delete it."), "f")).toBe(
      "Only the person who uploaded an attachment can delete it.",
    );
    expect(entityAttachmentErrorMessage("boom", "fallback")).toBe("fallback");
  });
});

describe("uploadEntityAttachment, size limit message", () => {
  const start = { attachmentId: "att-1", number: "WIA0000001", expiresAt: "2026-10-27T10:15:00Z", maxBytes: 100 };
  const file = new File(["12345"], "big.bin", { type: "application/octet-stream" });
  const args = { type: "work_item" as const, parentId: "wi-1", file };
  it("states the size limit in GB for a big maxBytes", async () => {
    const post = vi.fn().mockResolvedValueOnce({ ...start, maxBytes: 3 * 1024 * 1024 * 1024 });
    const big = { size: 4 * 1024 * 1024 * 1024, name: "huge.bin", type: "" } as unknown as File;
    await expect(uploadEntityAttachment(fakeApi(post), { ...args, file: big })).rejects.toThrow(/3 GB/);
  });
});

describe("with the files held by the portal backend", () => {
  const CHUNK = UPLOAD_CHUNK_BYTES;
  const started = { attachmentId: "att-1", number: "WIA0000001", expiresAt: "2026-10-27T10:15:00Z", maxBytes: 100 * 1024 * 1024 };
  const confirmed = { id: "att-1", number: "WIA0000001", name: "big.bin", sizeBytes: 0, status: "COMPLETE" };
  const bigFile = (size: number) =>
    ({
      size,
      name: "big.bin",
      type: "application/octet-stream",
      slice: (from: number, to: number) => new Blob([new Uint8Array(to - from)]),
    }) as unknown as File;
  const args = (file: File) => ({ type: "work_item" as const, parentId: "wi-1", file });

  beforeEach(() => {
    (window as unknown as { config?: unknown }).config = { CSM_PORTAL_ATTACHMENT_STORAGE_MODE: "backend" };
  });
  afterEach(() => {
    (window as unknown as { config?: unknown }).config = undefined;
    vi.useRealTimers();
    window.config = {} as typeof window.config;
  });

  const apiWith = (putBinary: ReturnType<typeof vi.fn>, post: ReturnType<typeof vi.fn>, getResponse = vi.fn()) =>
    ({ post, putBinary, getResponse }) as unknown as BackendApi;

  it("sends the file in 8 MiB chunks, then confirms with the chunk count", async () => {
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const putBinary = vi.fn().mockResolvedValue(undefined);
    const progress: number[] = [];
    await uploadEntityAttachment(apiWith(putBinary, post), {
      ...args(bigFile(2 * CHUNK + 5)),
      onProgress: (p) => progress.push(p),
    });
    const paths = putBinary.mock.calls.map((c) => String(c[0]));
    expect(paths.map((p) => p.match(/chunks\/(\d+)/)?.[1]).sort()).toEqual(["0", "1", "2"]);
    expect(paths.every((p) => p.endsWith("?type=work_item"))).toBe(true);
    expect(post.mock.calls[1][0]).toBe("/attachments/upload/att-1/confirm");
    expect(post.mock.calls[1][1]).toEqual({ type: "work_item", chunks: 3 });
    expect(progress[progress.length - 1]).toBe(100);
  });

  it("tries a chunk again after a passing failure", async () => {
    vi.useFakeTimers();
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const putBinary = vi.fn().mockRejectedValueOnce(apiError(503)).mockResolvedValue(undefined);
    const done = uploadEntityAttachment(apiWith(putBinary, post), args(bigFile(10)));
    await vi.runAllTimersAsync();
    await expect(done).resolves.toMatchObject({ status: "COMPLETE" });
    expect(putBinary).toHaveBeenCalledTimes(2);
  });

  it("does not retry a refusal, and stops taking new chunks after a failure", async () => {
    const post = vi.fn().mockResolvedValueOnce(started);
    const sent: number[] = [];
    const putBinary = vi.fn(async (path: string) => {
      const index = Number(path.match(/chunks\/(\d+)/)?.[1]);
      sent.push(index);
      if (index === 0) throw apiError(403, "Only the person who started the upload can send it.");
      await new Promise((resolve) => setTimeout(resolve, 30));
    });
    await expect(
      uploadEntityAttachment(apiWith(putBinary as unknown as ReturnType<typeof vi.fn>, post), args(bigFile(6 * CHUNK))),
    ).rejects.toThrow("Only the person who started the upload can send it.");
    expect(sent.length).toBeLessThanOrEqual(3);
    expect(post).toHaveBeenCalledTimes(1); // never confirmed
  });

  it("stops with the cancelled error when the signal fires", async () => {
    const post = vi.fn().mockResolvedValueOnce(started);
    const controller = new AbortController();
    const putBinary = vi.fn(async () => {
      controller.abort();
      throw Object.assign(new Error("aborted"), { name: "AbortError" });
    });
    await expect(
      uploadEntityAttachment(apiWith(putBinary as unknown as ReturnType<typeof vi.fn>, post), {
        ...args(bigFile(10)),
        signal: controller.signal,
      }),
    ).rejects.toThrow(UPLOAD_CANCELLED_MESSAGE);
  });

  it("downloads through the backend route and saves it under the name it suggests", async () => {
    const getResponse = vi.fn().mockResolvedValue(
      new Response("0123456789", {
        status: 206,
        headers: {
          "Content-Range": "bytes 0-9/10",
          "Content-Disposition": 'attachment; filename="report.pdf"',
        },
      }),
    );
    const post = vi.fn();
    const save = vi.fn();
    const done = await downloadEntityAttachment(apiWith(vi.fn(), post, getResponse), { type: "work_item", id: ID }, undefined, save);
    expect(done).toBe(true);
    expect(getResponse).toHaveBeenCalledTimes(1);
    expect(getResponse.mock.calls[0][0]).toBe(`/attachments/download/${ID}/content?type=work_item`);
    expect(getResponse.mock.calls[0][1]).toEqual({ headers: { Range: `bytes=0-${DOWNLOAD_RANGE_BYTES - 1}` } });
    expect(save.mock.calls[0][1]).toBe("report.pdf");
    expect(post).not.toHaveBeenCalled(); // no link is requested
  });

  const blobText = (blob: Blob) =>
    new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(reader.error);
      reader.readAsText(blob);
    });

  /** A file of `size` bytes where byte i is the letter for floor(i / RANGE), served by Range. */
  const rangeServer = (size: number, delay: (start: number) => number = () => 0) => {
    const byteAt = (i: number) => 65 + Math.floor(i / DOWNLOAD_RANGE_BYTES);
    let inFlight = 0;
    let peak = 0;
    const getResponse = vi.fn(async (_path: string, options?: { headers?: Record<string, string> }) => {
      const match = /bytes=(\d+)-(\d+)/.exec(options?.headers?.Range ?? "");
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
    return { getResponse, peak: () => peak };
  };

  it("fetches a large file as ranges and joins them in order, whatever order they finish in", async () => {
    const size = DOWNLOAD_RANGE_BYTES * 3 + 5;
    // The later ranges answer first.
    const server = rangeServer(size, (start) => (start === 0 ? 0 : 40 - (start / DOWNLOAD_RANGE_BYTES) * 10));
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID }, undefined, save);
    expect(server.getResponse).toHaveBeenCalledTimes(4);
    const blob = save.mock.calls[0][0] as Blob;
    expect(blob.size).toBe(size);
    // jsdom's Blob has no arrayBuffer(), so read the bytes through a FileReader.
    const bytes = new Uint8Array(
      await new Promise<ArrayBuffer>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as ArrayBuffer);
        reader.onerror = () => reject(reader.error);
        reader.readAsArrayBuffer(blob);
      }),
    );
    for (const [i, letter] of [[0, 65], [DOWNLOAD_RANGE_BYTES, 66], [DOWNLOAD_RANGE_BYTES * 2, 67], [size - 1, 68]]) {
      expect(bytes[i]).toBe(letter);
    }
    expect(save.mock.calls[0][1]).toBe("big.bin");
  });

  it("never fetches more ranges at once than the configured parallelism", async () => {
    const size = DOWNLOAD_RANGE_BYTES * 6;
    for (const [configured, expected] of [[undefined, 4], [2, 2], ["1", 1]] as const) {
      (window as unknown as { config: unknown }).config = {
        CSM_PORTAL_ATTACHMENT_STORAGE_MODE: "backend",
        CSM_PORTAL_DOWNLOAD_PARALLELISM: configured,
      };
      const server = rangeServer(size, () => 5);
      await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID }, undefined, vi.fn());
      expect(server.getResponse).toHaveBeenCalledTimes(6);
      expect(server.peak()).toBeLessThanOrEqual(expected);
      if (expected > 1) expect(server.peak()).toBeGreaterThan(1);
    }
  }, 30_000);

  it("reads the whole body when the server ignores the Range header", async () => {
    const getResponse = vi.fn().mockResolvedValue(new Response("0123456789", { status: 200 }));
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, undefined, save);
    expect(getResponse).toHaveBeenCalledTimes(1);
    expect(await blobText(save.mock.calls[0][0] as Blob)).toBe("0123456789");
    expect(save.mock.calls[0][1]).toBe("attachment");
  });

  it("fetches a range again when its connection breaks", async () => {
    const size = DOWNLOAD_RANGE_BYTES + 3;
    const server = rangeServer(size);
    let broke = false;
    const getResponse = vi.fn(async (path: string, options?: { headers?: Record<string, string> }) => {
      if (!broke && options?.headers?.Range?.startsWith(`bytes=${DOWNLOAD_RANGE_BYTES}-`)) {
        broke = true;
        throw new TypeError("network error");
      }
      return server.getResponse(path, options);
    });
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, undefined, save);
    expect(getResponse).toHaveBeenCalledTimes(3); // first range, the broken one, its retry
    expect((save.mock.calls[0][0] as Blob).size).toBe(size);
  });

  it("fetches a range again when its body is short", async () => {
    const size = DOWNLOAD_RANGE_BYTES + 3;
    const server = rangeServer(size);
    let shortOnce = true;
    const getResponse = vi.fn(async (path: string, options?: { headers?: Record<string, string> }) => {
      if (shortOnce && options?.headers?.Range?.startsWith(`bytes=${DOWNLOAD_RANGE_BYTES}-`)) {
        shortOnce = false;
        return new Response(new Uint8Array(1), {
          status: 206,
          headers: { "Content-Range": `bytes ${DOWNLOAD_RANGE_BYTES}-${size - 1}/${size}` },
        });
      }
      return server.getResponse(path, options);
    });
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, undefined, save);
    expect((save.mock.calls[0][0] as Blob).size).toBe(size);
  });

  it("gives up with an interruption error after three failed attempts at a range", async () => {
    const size = DOWNLOAD_RANGE_BYTES + 3;
    const server = rangeServer(size);
    const getResponse = vi.fn(async (path: string, options?: { headers?: Record<string, string> }) => {
      if (options?.headers?.Range?.startsWith(`bytes=${DOWNLOAD_RANGE_BYTES}-`)) throw new TypeError("network error");
      return server.getResponse(path, options);
    });
    const save = vi.fn();
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, undefined, save),
    ).rejects.toThrow("The download was interrupted.");
    expect(getResponse).toHaveBeenCalledTimes(1 + 3);
    expect(save).not.toHaveBeenCalled();
  });

  it("rejects a first range that is shorter than the file allows, even when it agrees with itself", async () => {
    const size = DOWNLOAD_RANGE_BYTES + 3;
    const getResponse = vi.fn(
      async () =>
        new Response(new Uint8Array(10), { status: 206, headers: { "Content-Range": `bytes 0-9/${size}` } }),
    );
    const save = vi.fn();
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, undefined, save),
    ).rejects.toThrow("The download was interrupted.");
    expect(save).not.toHaveBeenCalled();
  });

  it("rejects a later range whose Content-Range is not the one asked for", async () => {
    const size = DOWNLOAD_RANGE_BYTES + 3;
    const server = rangeServer(size);
    const getResponse = vi.fn(async (path: string, options?: { headers?: Record<string, string> }) => {
      if (options?.headers?.Range?.startsWith(`bytes=${DOWNLOAD_RANGE_BYTES}-`)) {
        // The right number of bytes, but from the wrong place.
        return new Response(new Uint8Array(3), {
          status: 206,
          headers: { "Content-Range": `bytes 0-2/${size}` },
        });
      }
      return server.getResponse(path, options);
    });
    const save = vi.fn();
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, undefined, save),
    ).rejects.toThrow("The download was interrupted.");
    expect(save).not.toHaveBeenCalled();
  });

  it("stops at the first refusal of a later range, with the backend's message", async () => {
    const size = DOWNLOAD_RANGE_BYTES * 5;
    const server = rangeServer(size, () => 5);
    const getResponse = vi.fn(async (path: string, options?: { headers?: Record<string, string> }) => {
      if (options?.headers?.Range?.startsWith(`bytes=${DOWNLOAD_RANGE_BYTES}-`)) throw apiError(410, "This attachment was removed.");
      return server.getResponse(path, options);
    });
    const save = vi.fn();
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, undefined, save),
    ).rejects.toThrow("This attachment was removed.");
    expect(getResponse.mock.calls.length).toBeLessThan(6); // the ranges not yet started never are
    expect(save).not.toHaveBeenCalled();
  });

  it("reports false on a 404 so the older flow can be tried, and throws for a removed file", async () => {
    const save = vi.fn();
    const missing = vi.fn().mockRejectedValue(apiError(404));
    expect(await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), missing), { type: "work_item", id: ID }, undefined, save)).toBe(false);
    const gone = vi.fn().mockRejectedValue(apiError(410, "This attachment was removed."));
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), gone), { type: "work_item", id: ID }, undefined, save),
    ).rejects.toThrow("This attachment was removed.");
    expect(save).not.toHaveBeenCalled();
  });
  describe("straight to disk", () => {
    const readFirstByte = (blob: Blob) =>
      new Promise<number>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(new Uint8Array(reader.result as ArrayBuffer)[0]);
        reader.onerror = () => reject(reader.error);
        reader.readAsArrayBuffer(blob.slice(0, 1));
      });

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
    const run = (getResponse: Parameters<typeof apiWith>[2], save: (blob: Blob, name: string) => void, extra: { size?: number } = {}) =>
      downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), getResponse),
        { type: "work_item", id: ID, name: "big.bin", ...extra },
        undefined,
        save,
      );

    afterEach(() => {
      delete (window as unknown as { showSaveFilePicker?: unknown }).showSaveFilePicker;
    });

    it("asks where to save before the first request and writes each range at its own offset", async () => {
      const size = DOWNLOAD_RANGE_BYTES * 3 + 5;
      const disk = fakeDisk();
      // The later ranges answer first.
      const server = rangeServer(size, (start) => (start === 0 ? 0 : 40 - (start / DOWNLOAD_RANGE_BYTES) * 10));
      const save = vi.fn();
      expect(await run(server.getResponse, save)).toBe(true);
      expect(disk.picker).toHaveBeenCalledWith({ suggestedName: "big.bin" });
      expect(disk.picker.mock.invocationCallOrder[0]).toBeLessThan(server.getResponse.mock.invocationCallOrder[0]);
      const byPosition = [...disk.writes].sort((a, b) => a.position - b.position);
      expect(byPosition).toEqual([
        { position: 0, size: DOWNLOAD_RANGE_BYTES, first: 65 },
        { position: DOWNLOAD_RANGE_BYTES, size: DOWNLOAD_RANGE_BYTES, first: 66 },
        { position: DOWNLOAD_RANGE_BYTES * 2, size: DOWNLOAD_RANGE_BYTES, first: 67 },
        { position: DOWNLOAD_RANGE_BYTES * 3, size: 5, first: 68 },
      ]);
      expect(disk.writable.close).toHaveBeenCalledTimes(1);
      expect(disk.writable.abort).not.toHaveBeenCalled();
      expect(save).not.toHaveBeenCalled(); // nothing is held for a Blob
    });

    it("drops the half-written file when a range is refused", async () => {
      const disk = fakeDisk();
      const server = rangeServer(DOWNLOAD_RANGE_BYTES * 2 + 5);
      const failing = vi.fn(async (path: string, options?: { headers?: Record<string, string> }) => {
        if (!/bytes=0-/.test(options?.headers?.Range ?? "")) throw apiError(403, "Not allowed.");
        return server.getResponse(path, options);
      });
      const save = vi.fn();
      await expect(run(failing, save)).rejects.toThrow("Not allowed.");
      expect(disk.writable.abort).toHaveBeenCalledTimes(1);
      expect(disk.writable.close).not.toHaveBeenCalled();
      expect(save).not.toHaveBeenCalled();
    });

    it("drops the file and reports false on a 404, so the older flow can be tried", async () => {
      const disk = fakeDisk();
      const save = vi.fn();
      expect(await run(vi.fn().mockRejectedValue(apiError(404)), save)).toBe(false);
      expect(disk.writable.abort).toHaveBeenCalledTimes(1);
    });

    it("does nothing, and shows no error, when the save dialog is closed", async () => {
      const disk = fakeDisk();
      disk.picker.mockRejectedValue(new DOMException("The user aborted a request.", "AbortError"));
      const server = rangeServer(DOWNLOAD_RANGE_BYTES * 2);
      const save = vi.fn();
      expect(await run(server.getResponse, save)).toBe(true);
      expect(server.getResponse).not.toHaveBeenCalled();
      expect(save).not.toHaveBeenCalled();
    });

    it("uses the in-memory save when the dialog is refused for another reason", async () => {
      const disk = fakeDisk();
      disk.picker.mockRejectedValue(new DOMException("Needs a click.", "SecurityError"));
      const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
      const save = vi.fn();
      expect(await run(server.getResponse, save)).toBe(true);
      expect((save.mock.calls[0][0] as Blob).size).toBe(DOWNLOAD_RANGE_BYTES + 5);
    });

    it("uses the in-memory save when the browser has no save dialog", async () => {
      const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
      const save = vi.fn();
      expect(await run(server.getResponse, save)).toBe(true);
      expect((save.mock.calls[0][0] as Blob).size).toBe(DOWNLOAD_RANGE_BYTES + 5);
    });

    it("skips the dialog for a small file whose size is known", async () => {
      const disk = fakeDisk();
      const save = vi.fn();
      const small = vi.fn().mockResolvedValue(
        new Response("0123456789", { status: 200, headers: { "Content-Disposition": 'attachment; filename="s.txt"' } }),
      );
      await run(small, save, { size: 10 });
      expect(disk.picker).not.toHaveBeenCalled();
      expect(save).toHaveBeenCalledTimes(1);
    });

    it("asks for a save location when the size is not known", async () => {
      const disk = fakeDisk();
      const small = vi.fn().mockResolvedValue(new Response("0123456789", { status: 200 }));
      await run(small, vi.fn());
      expect(disk.picker).toHaveBeenCalledTimes(1);
      expect(disk.writes).toEqual([{ position: 0, size: 10, first: 48 }]);
    });

    it("skips the dialog when CSM_PORTAL_DOWNLOAD_TO_DISK is false", async () => {
      window.config = { CSM_PORTAL_ATTACHMENT_STORAGE_MODE: "backend", CSM_PORTAL_DOWNLOAD_TO_DISK: false } as typeof window.config;
      const disk = fakeDisk();
      const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
      const save = vi.fn();
      await run(server.getResponse, save);
      expect(disk.picker).not.toHaveBeenCalled();
      expect(save).toHaveBeenCalledTimes(1);
    });
  });
});
