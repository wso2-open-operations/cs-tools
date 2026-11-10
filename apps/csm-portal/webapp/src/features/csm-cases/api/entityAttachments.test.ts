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
import { isDownloadAbort } from "@features/csm-cases/api/downloadAbort";
import {
  confirmEntityAttachment,
  deleteEntityAttachment,
  DOWNLOAD_RANGE_BYTES,
  downloadEntityAttachment,
  ENTITY_ATTACHMENT_RATE_LIMIT_MESSAGE,
  entityAttachmentErrorMessage,
  EntityUploadConfirmError,
  DIRECT_TRANSFER_MIN_BYTES,
  UPLOAD_CHUNK_BYTES,
  UPLOAD_CANCELLED_MESSAGE,
  fetchEntityAttachmentBlob,
  fetchEntityInlineImage,
  uploadEntityAttachment,
} from "@features/csm-cases/api/entityAttachments";

const ID = "11111111-1111-4111-8111-111111111111";

function apiError(status: number, message = "x"): Error {
  return new (BackendApiError as unknown as new (s: number, m: string) => Error)(status, message);
}

function fakeApi(post: ReturnType<typeof vi.fn>, getBlob = vi.fn()): BackendApi {
  return { post, getBlob } as unknown as BackendApi;
}

const apiWith = (putBinary: ReturnType<typeof vi.fn>, post: ReturnType<typeof vi.fn>, getResponse = vi.fn()) =>
  ({ post, putBinary, getResponse }) as unknown as BackendApi;

describe("fetchEntityAttachmentBlob", () => {
  it("streams the file through the backend route and returns the bytes", async () => {
    const blob = new Blob(["hello"]);
    const getBlob = vi.fn().mockResolvedValue(blob);
    const got = await fetchEntityAttachmentBlob(fakeApi(vi.fn(), getBlob), { type: "work_item", id: ID });
    expect(getBlob).toHaveBeenCalledWith(`/attachments/download/${ID}/content?type=work_item`);
    expect(got).toBe(blob);
  });

  it("throws any other failure, such as a removed file", async () => {
    const getBlob = vi.fn().mockRejectedValue(apiError(410, "This attachment was removed."));
    await expect(fetchEntityAttachmentBlob(fakeApi(vi.fn(), getBlob), { type: "work_item", id: ID })).rejects.toThrow(
      "This attachment was removed.",
    );
  });
});

describe("uploadEntityAttachment", () => {
  const file = new File(["12345"], "report.pdf", { type: "application/pdf" });
  const start = {
    attachmentId: "att-1",
    number: "WIA0000001",
    expiresAt: "2026-10-27T10:15:00Z",
    maxBytes: 100 * 1024 * 1024,
  };
  const confirmed = { id: "att-1", number: "WIA0000001", name: "report.pdf", sizeBytes: 5, status: "COMPLETE" };
  const args = { type: "work_item" as const, parentId: "wi-1", file };

  it("runs start, then the chunk, then confirm with the chunk count", async () => {
    const post = vi.fn().mockResolvedValueOnce(start).mockResolvedValueOnce(confirmed);
    const putBinary = vi.fn().mockResolvedValue(undefined);
    const progress: number[] = [];

    const out = await uploadEntityAttachment(apiWith(putBinary, post), {
      ...args,
      onProgress: (p) => progress.push(p),
    });

    expect(out.status).toBe("COMPLETE");
    expect(post.mock.calls[0]).toEqual([
      "/attachments/upload",
      { type: "work_item", parentId: "wi-1", name: "report.pdf", contentType: "application/pdf", sizeBytes: 5 },
    ]);
    expect(putBinary).toHaveBeenCalledTimes(1);
    expect(putBinary.mock.calls[0][0]).toBe("/attachments/upload/att-1/chunks/0?type=work_item");
    expect(post.mock.calls[1]).toEqual(["/attachments/upload/att-1/confirm", { type: "work_item", chunks: 1 }]);
    expect(progress[0]).toBe(0);
    expect(progress[progress.length - 1]).toBe(100);
  });

  it("sends projectId instead of parentId for a draft image, and the chunk and confirm by id", async () => {
    const post = vi.fn().mockResolvedValueOnce(start).mockResolvedValueOnce(confirmed);
    const putBinary = vi.fn().mockResolvedValue(undefined);

    await uploadEntityAttachment(apiWith(putBinary, post), {
      type: "comment",
      projectId: "proj-1",
      file,
    });

    expect(post.mock.calls[0][1]).toEqual({
      type: "comment",
      projectId: "proj-1",
      name: "report.pdf",
      contentType: "application/pdf",
      sizeBytes: 5,
    });
    expect(putBinary.mock.calls[0][0]).toBe("/attachments/upload/att-1/chunks/0?type=comment");
    expect(post.mock.calls[1]).toEqual(["/attachments/upload/att-1/confirm", { type: "comment", chunks: 1 }]);
  });

  it("refuses to start without exactly one of parentId and projectId", async () => {
    const post = vi.fn();
    const putBinary = vi.fn();
    const bad = { type: "comment" as const, file, parentId: "p", projectId: "q" };
    await expect(uploadEntityAttachment(apiWith(putBinary, post), bad as never)).rejects.toThrow(/exactly one/);
    await expect(
      uploadEntityAttachment(apiWith(putBinary, post), { type: "comment", file } as never),
    ).rejects.toThrow(/exactly one/);
    expect(post).not.toHaveBeenCalled();
  });

  it("refuses a file over the limit the server reports, before sending it", async () => {
    const post = vi.fn().mockResolvedValueOnce({ ...start, maxBytes: 3 });
    const putBinary = vi.fn();
    await expect(uploadEntityAttachment(apiWith(putBinary, post), args)).rejects.toThrow(/too large/);
    expect(putBinary).not.toHaveBeenCalled();
  });

  it("stops without confirming when a chunk is refused", async () => {
    const post = vi.fn().mockResolvedValueOnce(start);
    const putBinary = vi.fn().mockRejectedValue(apiError(413, "The upload is larger than allowed."));
    await expect(uploadEntityAttachment(apiWith(putBinary, post), args)).rejects.toThrow(
      "The upload is larger than allowed.",
    );
    expect(post).toHaveBeenCalledTimes(1);
  });

  it("retries a 503 confirm without sending the file again", async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce(start)
      .mockRejectedValueOnce(apiError(503))
      .mockResolvedValueOnce(confirmed);
    const putBinary = vi.fn().mockResolvedValue(undefined);
    const out = await uploadEntityAttachment(apiWith(putBinary, post), args, { confirmDelayMs: 0 });
    expect(out.status).toBe("COMPLETE");
    expect(putBinary).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledTimes(3);
  });

  it("throws a resumable error when confirm keeps answering 503", async () => {
    const post = vi.fn().mockResolvedValueOnce(start).mockRejectedValue(apiError(503));
    const putBinary = vi.fn().mockResolvedValue(undefined);
    const err = await uploadEntityAttachment(apiWith(putBinary, post), args, { confirmDelayMs: 0 }).catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(EntityUploadConfirmError);
    expect((err as EntityUploadConfirmError).attachmentId).toBe("att-1");
    expect(putBinary).toHaveBeenCalledTimes(1);
  });

  it("surfaces a closed-case 409 from start untouched and sends nothing", async () => {
    const post = vi.fn().mockRejectedValueOnce(apiError(409, "This case is closed, so new attachments can't be added."));
    const putBinary = vi.fn();
    await expect(uploadEntityAttachment(apiWith(putBinary, post), args)).rejects.toThrow(
      "This case is closed, so new attachments can't be added.",
    );
    expect(putBinary).not.toHaveBeenCalled();
  });

  it("does not retry a confirm that is refused for another reason", async () => {
    const post = vi.fn().mockResolvedValueOnce(start).mockRejectedValueOnce(apiError(409, "closed"));
    const putBinary = vi.fn().mockResolvedValue(undefined);
    await expect(uploadEntityAttachment(apiWith(putBinary, post), args, { confirmDelayMs: 0 })).rejects.toThrow("closed");
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

describe("chunked upload and streamed download", () => {
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
    window.config = {} as typeof window.config;
  });
  afterEach(() => {
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
    const done = await downloadEntityAttachment(apiWith(vi.fn(), post, getResponse), { type: "work_item", id: ID }, save);
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
  const rangeServer = (size: number, delay: (start: number) => number = () => 0, hideHeaders = false) => {
    const byteAt = (i: number) => 65 + Math.floor(i / DOWNLOAD_RANGE_BYTES);
    let inFlight = 0;
    let peak = 0;
    const getResponse = vi.fn(async (_path: string, options?: { headers?: Record<string, string> }) => {
      const match = /bytes=(\d+)-(\d+)/.exec(options?.headers?.Range ?? "");
      const start = Number(match?.[1] ?? 0);
      if (start >= size) throw apiError(416, "Range Not Satisfiable"); // past the end of the file
      const end = Math.min(Number(match?.[2] ?? size - 1), size - 1);
      inFlight += 1;
      peak = Math.max(peak, inFlight);
      await new Promise((r) => setTimeout(r, delay(start)));
      inFlight -= 1;
      const body = new Uint8Array(end - start + 1).map((_, i) => byteAt(start + i));
      // A cross-origin response without Access-Control-Expose-Headers: the browser hands page
      // scripts neither Content-Range nor Content-Disposition (Content-Length is always readable).
      return new Response(body, {
        status: 206,
        headers: hideHeaders
          ? { "Content-Length": String(body.length) }
          : {
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
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID }, save);
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
        CSM_PORTAL_DOWNLOAD_PARALLELISM: configured,
      };
      const server = rangeServer(size, () => 5);
      await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID }, vi.fn());
      expect(server.getResponse).toHaveBeenCalledTimes(6);
      expect(server.peak()).toBeLessThanOrEqual(expected);
      if (expected > 1) expect(server.peak()).toBeGreaterThan(1);
    }
  }, 30_000);

  it("reads the whole body when the server ignores the Range header", async () => {
    const getResponse = vi.fn().mockResolvedValue(new Response("0123456789", { status: 200 }));
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save);
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
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save);
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
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save);
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
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save),
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
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save),
    ).rejects.toThrow("The download was interrupted.");
    expect(save).not.toHaveBeenCalled();
  });

  describe("cancelling a download", () => {
    const hiddenForCancel = (size: number) => rangeServer(size, () => 0, true);
    /** The browser's own behaviour: a fetch whose signal is aborted rejects with an AbortError. */
    const abortable = (server: ReturnType<typeof rangeServer>, onCall: (n: number) => void = () => {}) => {
      let calls = 0;
      return vi.fn(async (path: string, options?: { headers?: Record<string, string>; signal?: AbortSignal }) => {
        if (options?.signal?.aborted) throw new DOMException("aborted", "AbortError");
        calls += 1;
        onCall(calls);
        if (options?.signal?.aborted) throw new DOMException("aborted", "AbortError");
        return server.getResponse(path, options);
      });
    };

    it("stops, saves nothing and throws an AbortError when cancelled part-way, without retrying", async () => {
      const server = rangeServer(DOWNLOAD_RANGE_BYTES * 4);
      const controller = new AbortController();
      const getResponse = abortable(server, (n) => n === 2 && controller.abort());
      const save = vi.fn();
      const outcome = await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), getResponse),
        { type: "work_item", id: ID, signal: controller.signal },
        save,
      ).catch((e: unknown) => e);
      expect(isDownloadAbort(outcome)).toBe(true);
      expect(save).not.toHaveBeenCalled();
      // The cancelled request is not tried again, and no further ranges are asked for.
      expect(getResponse.mock.calls.length).toBeLessThanOrEqual(1 + 8);
    });

    it("does not send a request when it is already cancelled", async () => {
      const getResponse = vi.fn();
      const controller = new AbortController();
      controller.abort();
      const outcome = await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), getResponse),
        { type: "work_item", id: ID, signal: controller.signal },
        vi.fn(),
      ).catch((e: unknown) => e);
      expect(isDownloadAbort(outcome)).toBe(true);
      expect(getResponse).not.toHaveBeenCalled();
    });

    it("never calls onSaved for a cancelled download", async () => {
      const server = rangeServer(DOWNLOAD_RANGE_BYTES * 2);
      const controller = new AbortController();
      const onSaved = vi.fn();
      await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), abortable(server, (n) => n === 2 && controller.abort())),
        { type: "work_item", id: ID, onSaved, signal: controller.signal },
        vi.fn(),
      ).catch(() => {});
      expect(onSaved).not.toHaveBeenCalled();
    });

    it("cancels a download of unknown size that is being read to its end", async () => {
      const server = hiddenForCancel(DOWNLOAD_RANGE_BYTES * 4);
      const controller = new AbortController();
      const save = vi.fn();
      const outcome = await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), abortable(server, (n) => n === 2 && controller.abort())),
        { type: "work_item", id: ID, signal: controller.signal },
        save,
      ).catch((e: unknown) => e);
      expect(isDownloadAbort(outcome)).toBe(true);
      expect(save).not.toHaveBeenCalled();
    });
  });

  describe("when the browser hides Content-Range and Content-Disposition", () => {
    const hidden = (size: number) => rangeServer(size, () => 0, true);

    it("saves a small file under the attachment's own name, using its known size", async () => {
      const server = hidden(1_100_144);
      const save = vi.fn();
      const done = await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), server.getResponse),
        { type: "work_item", id: ID, name: "1mb.pdf", size: 1_100_144 },
        save,
      );
      expect(done).toBe(true);
      expect(server.getResponse).toHaveBeenCalledTimes(1); // no retries
      expect((save.mock.calls[0][0] as Blob).size).toBe(1_100_144);
      expect(save.mock.calls[0][1]).toBe("1mb.pdf");
    });

    it("takes a body shorter than the range asked for as the whole file when no size is known", async () => {
      const server = hidden(1000);
      const save = vi.fn();
      await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID }, save);
      expect(server.getResponse).toHaveBeenCalledTimes(1);
      expect((save.mock.calls[0][0] as Blob).size).toBe(1000);
      expect(save.mock.calls[0][1]).toBe("attachment");
    });

    it("fetches a large file as ranges, the total taken from the known size", async () => {
      const size = DOWNLOAD_RANGE_BYTES * 2 + 7;
      const server = hidden(size);
      const save = vi.fn();
      const progress = vi.fn();
      await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), server.getResponse),
        { type: "work_item", id: ID, name: "big.bin", size, onProgress: progress },
        save,
      );
      expect(server.getResponse).toHaveBeenCalledTimes(3);
      expect((save.mock.calls[0][0] as Blob).size).toBe(size);
      expect(progress).toHaveBeenLastCalledWith(size, size);
    });

    it("still rejects a first range whose length is not what the known size allows", async () => {
      const server = hidden(1000);
      const save = vi.fn();
      await expect(
        downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID, size: 5000 }, save),
      ).rejects.toThrow("The download was interrupted.");
      expect(save).not.toHaveBeenCalled();
    });

    it("reads on until the file ends when no size is known and the first range is full", async () => {
      const size = DOWNLOAD_RANGE_BYTES * 2 + 11;
      const server = hidden(size);
      const save = vi.fn();
      await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID }, save);
      expect((save.mock.calls[0][0] as Blob).size).toBe(size);
    });

    it("stops on the refusal past the end when the file is an exact multiple of the range", async () => {
      const size = DOWNLOAD_RANGE_BYTES * 2;
      const server = hidden(size);
      const save = vi.fn();
      await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), server.getResponse), { type: "work_item", id: ID }, save);
      expect((save.mock.calls[0][0] as Blob).size).toBe(size);
    });

    it("gives up on a backend that never ends the file, instead of reading for ever", async () => {
      // Every range is full, however far on: there is no end to find.
      const getResponse = vi.fn(
        async (_path: string, options?: { headers?: Record<string, string> }) => {
          const start = Number(/bytes=(\d+)-/.exec(options?.headers?.Range ?? "")?.[1] ?? 0);
          const body = new Uint8Array(DOWNLOAD_RANGE_BYTES);
          return new Response(body, { status: 206, headers: { "Content-Length": String(body.length), "X-Start": String(start) } });
        },
      );
      const save = vi.fn();
      await expect(
        downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save),
      ).rejects.toThrow("The download was interrupted.");
      expect(save).not.toHaveBeenCalled();
      // The 1024 MB ceiling in 8 MiB ranges, plus the one for the refusal, then it stops.
      expect(getResponse.mock.calls.length).toBeLessThanOrEqual(1 + Math.ceil(1024 / 8) + 1 + 8);
    });

    it("reads a range again, rather than ending the file, when its body is cut short of its Content-Length", async () => {
      const size = DOWNLOAD_RANGE_BYTES + 50;
      const server = hidden(size);
      let cut = true;
      const getResponse = vi.fn(async (path: string, options?: { headers?: Record<string, string> }) => {
        const response = await server.getResponse(path, options);
        if (cut && /bytes=\d+-/.exec(options?.headers?.Range ?? "")?.[0] !== "bytes=0-") {
          cut = false;
          // The connection dropped half-way: Content-Length promises more than arrives.
          const full = new Uint8Array(await response.arrayBuffer());
          return new Response(full.slice(0, 10), { status: 206, headers: { "Content-Length": String(full.length) } });
        }
        return response;
      });
      const save = vi.fn();
      await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save);
      expect((save.mock.calls[0][0] as Blob).size).toBe(size);
    });
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
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save),
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
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save),
    ).rejects.toThrow("This attachment was removed.");
    expect(getResponse.mock.calls.length).toBeLessThan(6); // the ranges not yet started never are
    expect(save).not.toHaveBeenCalled();
  });

  it("reports false on a 404 so the older flow can be tried, and throws for a removed file", async () => {
    const save = vi.fn();
    const missing = vi.fn().mockRejectedValue(apiError(404));
    expect(await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), missing), { type: "work_item", id: ID }, save)).toBe(false);
    const gone = vi.fn().mockRejectedValue(apiError(410, "This attachment was removed."));
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), gone), { type: "work_item", id: ID }, save),
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

    it("reports `saved` only when the file was written to the picked file", async () => {
      fakeDisk();
      const onSaved = vi.fn();
      const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
      await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), server.getResponse),
        { type: "work_item", id: ID, name: "big.bin", onSaved },
        vi.fn(),
      );
      expect(onSaved).toHaveBeenCalledExactlyOnceWith("saved");
    });

    it("reports `started`, not a completed save, when the Blob is handed to the browser", async () => {
      delete (window as unknown as { showSaveFilePicker?: unknown }).showSaveFilePicker;
      const onSaved = vi.fn();
      const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
      await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), server.getResponse),
        { type: "work_item", id: ID, name: "big.bin", onSaved },
        vi.fn(),
      );
      expect(onSaved).toHaveBeenCalledExactlyOnceWith("started");
    });

    it("never reports a closed save dialog or a failed download", async () => {
      const disk = fakeDisk();
      disk.picker.mockRejectedValue(new DOMException("Closed.", "AbortError"));
      const onSaved = vi.fn();
      await downloadEntityAttachment(
        apiWith(vi.fn(), vi.fn(), rangeServer(DOWNLOAD_RANGE_BYTES + 5).getResponse),
        { type: "work_item", id: ID, name: "big.bin", onSaved },
        vi.fn(),
      );
      expect(onSaved).not.toHaveBeenCalled();
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
      window.config = { CSM_PORTAL_DOWNLOAD_TO_DISK: false } as typeof window.config;
      const disk = fakeDisk();
      const server = rangeServer(DOWNLOAD_RANGE_BYTES + 5);
      const save = vi.fn();
      await run(server.getResponse, save);
      expect(disk.picker).not.toHaveBeenCalled();
      expect(save).toHaveBeenCalledTimes(1);
    });
  });
});

describe("direct transfers of large files", () => {
  const CHUNK = UPLOAD_CHUNK_BYTES;
  const started = {
    attachmentId: "att-1",
    number: "WIA0000001",
    expiresAt: "2026-10-27T10:15:00Z",
    maxBytes: 100 * 1024 * 1024,
    directUpload: true,
  };
  const confirmed = { id: "att-1", number: "WIA0000001", name: "big.bin", sizeBytes: 0, status: "COMPLETE" };
  const bigFile = (size: number) =>
    ({
      size,
      name: "big.bin",
      type: "application/octet-stream",
      slice: (from: number, to: number) => new Blob([new Uint8Array(to - from)]),
    }) as unknown as File;
  const args = (file: File) => ({ type: "work_item" as const, parentId: "wi-1", file });
  const linksFor = (count: number) => ({
    links: Array.from({ length: count }, (_, i) => `https://blob.example/x?sig=s&comp=block&blockid=${i}`),
    chunkBytes: CHUNK,
    expiresAt: "2026-10-27T10:45:00Z",
  });
  const apiFor = (over: Partial<Record<"post" | "postEmpty" | "putBinary" | "get" | "getResponse", ReturnType<typeof vi.fn>>>) =>
    ({ post: vi.fn(), postEmpty: vi.fn(), putBinary: vi.fn().mockResolvedValue(undefined), get: vi.fn(), getResponse: vi.fn(), ...over }) as unknown as BackendApi;
  const okResponse = () => Promise.resolve(new Response(null, { status: 201 }));

  beforeEach(() => {
    window.config = {} as typeof window.config;
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("sends each chunk to its own signed link without the session's token, then confirms", async () => {
    const fetchMock = vi.fn(okResponse);
    vi.stubGlobal("fetch", fetchMock);
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const postEmpty = vi.fn().mockResolvedValue(linksFor(3));
    const putBinary = vi.fn();
    await uploadEntityAttachment(apiFor({ post, postEmpty, putBinary }), args(bigFile(2 * CHUNK + 5)));
    expect(postEmpty).toHaveBeenCalledWith("/attachments/upload/att-1/direct?type=work_item");
    expect(putBinary).not.toHaveBeenCalled();
    const sent = fetchMock.mock.calls.map((c) => String(c[0])).sort();
    expect(sent).toEqual([0, 1, 2].map((i) => `https://blob.example/x?sig=s&comp=block&blockid=${i}`));
    for (const call of fetchMock.mock.calls) {
      expect((call[1] as RequestInit).method).toBe("PUT");
      expect((call[1] as RequestInit).headers).toBeUndefined();
    }
    expect(post.mock.calls[1][0]).toBe("/attachments/upload/att-1/confirm");
    expect(post.mock.calls[1][1]).toEqual({ type: "work_item", chunks: 3 });
  });

  it("never asks for links for a file that fits in one chunk, even when the server allows direct", async () => {
    const fetchMock = vi.fn(okResponse);
    vi.stubGlobal("fetch", fetchMock);
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const postEmpty = vi.fn();
    const putBinary = vi.fn().mockResolvedValue(undefined);
    await uploadEntityAttachment(apiFor({ post, postEmpty, putBinary }), args(bigFile(DIRECT_TRANSFER_MIN_BYTES)));
    expect(postEmpty).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(putBinary).toHaveBeenCalledTimes(1);
  });

  it("sends the chunks through the backend when the links cannot be had", async () => {
    vi.stubGlobal("fetch", vi.fn(okResponse));
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const postEmpty = vi.fn().mockRejectedValue(apiError(400, "This upload cannot be sent directly."));
    const putBinary = vi.fn().mockResolvedValue(undefined);
    await uploadEntityAttachment(apiFor({ post, postEmpty, putBinary }), args(bigFile(2 * CHUNK)));
    expect(putBinary).toHaveBeenCalledTimes(2);
    expect(post.mock.calls[1][1]).toEqual({ type: "work_item", chunks: 2 });
  });

  it("asks for new links when they expire part-way, and carries on", async () => {
    // The first links are refused as expired; the new ones work.
    const fetchMock = vi.fn((url: string) =>
      Promise.resolve(new Response(null, { status: String(url).includes("fresh") ? 201 : 403 })),
    );
    vi.stubGlobal("fetch", fetchMock);
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const fresh = { ...linksFor(3), links: linksFor(3).links.map((l) => `${l}&fresh=1`) };
    const postEmpty = vi.fn().mockResolvedValueOnce(linksFor(3)).mockResolvedValue(fresh);
    const putBinary = vi.fn();
    await uploadEntityAttachment(apiFor({ post, postEmpty, putBinary }), args(bigFile(2 * CHUNK + 5)));
    expect(postEmpty).toHaveBeenCalledTimes(2); // one replacement, shared by every chunk in flight
    expect(putBinary).not.toHaveBeenCalled(); // never fell back
    expect(post.mock.calls[1][1]).toEqual({ type: "work_item", chunks: 3 });
    const okUrls = fetchMock.mock.calls.map((c) => String(c[0])).filter((u) => u.includes("fresh"));
    expect(okUrls).toHaveLength(3);
  });

  it("falls back to the backend when Azure keeps refusing, after replacing the links a few times", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(new Response(null, { status: 403 })));
    vi.stubGlobal("fetch", fetchMock);
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const postEmpty = vi.fn().mockResolvedValue(linksFor(2));
    const putBinary = vi.fn().mockResolvedValue(undefined);
    await uploadEntityAttachment(apiFor({ post, postEmpty, putBinary }), args(bigFile(2 * CHUNK)));
    expect(postEmpty).toHaveBeenCalledTimes(4); // the first links and three replacements
    expect(putBinary).toHaveBeenCalledTimes(2);
    expect(post.mock.calls[1][1]).toEqual({ type: "work_item", chunks: 2 });
  });

  it("tries a chunk again when Azure is busy", async () => {
    vi.useFakeTimers();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockImplementation(okResponse);
    vi.stubGlobal("fetch", fetchMock);
    const post = vi.fn().mockResolvedValueOnce(started).mockResolvedValueOnce(confirmed);
    const postEmpty = vi.fn().mockResolvedValue(linksFor(2));
    const putBinary = vi.fn();
    const done = uploadEntityAttachment(apiFor({ post, postEmpty, putBinary }), args(bigFile(2 * CHUNK)));
    await vi.runAllTimersAsync();
    await expect(done).resolves.toMatchObject({ status: "COMPLETE" });
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(putBinary).not.toHaveBeenCalled();
  });

  it("stops, and does not fall back, when the upload is cancelled", async () => {
    const controller = new AbortController();
    vi.stubGlobal(
      "fetch",
      vi.fn(() => {
        controller.abort();
        return Promise.reject(Object.assign(new Error("aborted"), { name: "AbortError" }));
      }),
    );
    const post = vi.fn().mockResolvedValueOnce(started);
    const postEmpty = vi.fn().mockResolvedValue(linksFor(2));
    const putBinary = vi.fn();
    await expect(
      uploadEntityAttachment(apiFor({ post, postEmpty, putBinary }), { ...args(bigFile(2 * CHUNK)), signal: controller.signal }),
    ).rejects.toThrow(UPLOAD_CANCELLED_MESSAGE);
    expect(putBinary).not.toHaveBeenCalled();
    expect(post).toHaveBeenCalledTimes(1); // never confirmed
  });

  describe("download", () => {
    const click = () => vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

    it("hands a large file's signed link to the browser as a download", async () => {
      const clicked = click();
      const get = vi.fn().mockResolvedValue({ url: "https://blob.example/x?sig=s", name: "big.bin" });
      const getResponse = vi.fn();
      const onSaved = vi.fn();
      const done = await downloadEntityAttachment(apiFor({ get, getResponse }), {
        type: "work_item",
        id: ID,
        name: "big.bin",
        size: DIRECT_TRANSFER_MIN_BYTES + 1,
        onSaved,
      });
      expect(done).toBe(true);
      expect(get).toHaveBeenCalledWith(`/attachments/download/${ID}/link?type=work_item`);
      expect(clicked).toHaveBeenCalledTimes(1);
      expect(onSaved).toHaveBeenCalledWith("started");
      expect(getResponse).not.toHaveBeenCalled(); // nothing streamed through the backend
    });

    it("streams through the backend when no link is available", async () => {
      const clicked = click();
      const body = () =>
        new Response("0123456789", {
          status: 206,
          headers: { "Content-Range": "bytes 0-9/10", "Content-Disposition": 'attachment; filename="big.bin"' },
        });
      for (const get of [vi.fn().mockResolvedValue(null), vi.fn().mockRejectedValue(apiError(400, "off"))]) {
        const getResponse = vi.fn().mockResolvedValue(body());
        const save = vi.fn();
        const done = await downloadEntityAttachment(
          apiFor({ get, getResponse }),
          { type: "work_item", id: ID, size: DIRECT_TRANSFER_MIN_BYTES + 1 },
          save,
        );
        expect(done).toBe(true);
        expect(getResponse).toHaveBeenCalledTimes(1);
      }
      expect(clicked).not.toHaveBeenCalled();
    });

    it("does not ask for a link for a small file, or when the size is not known", async () => {
      click();
      for (const size of [DIRECT_TRANSFER_MIN_BYTES, undefined]) {
        const get = vi.fn();
        const getResponse = vi.fn().mockResolvedValue(
          new Response("0123456789", { status: 206, headers: { "Content-Range": "bytes 0-9/10" } }),
        );
        await downloadEntityAttachment(apiFor({ get, getResponse }), { type: "work_item", id: ID, size }, vi.fn());
        expect(get).not.toHaveBeenCalled();
      }
    });
  });
});
