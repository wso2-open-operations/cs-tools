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

import { afterEach, describe, expect, it, vi } from "vitest";

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
  downloadEntityAttachment,
  ENTITY_ATTACHMENT_RATE_LIMIT_MESSAGE,
  entityAttachmentErrorMessage,
  EntityUploadConfirmError,
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

  afterEach(() => {
    vi.useRealTimers();
  });

  it("sends the file in 16 MiB chunks, then confirms with the chunk count", async () => {
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
        status: 200,
        headers: { "Content-Length": "10", "Content-Disposition": 'attachment; filename="report.pdf"' },
      }),
    );
    const post = vi.fn();
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), post, getResponse), { type: "work_item", id: ID }, save);
    expect(getResponse.mock.calls[0][0]).toBe(`/attachments/download/${ID}/content?type=work_item`);
    expect(save.mock.calls[0][1]).toBe("report.pdf");
    expect(post).not.toHaveBeenCalled(); // no link is requested
  });

  // A response that delivers `text` and then breaks, like a dropped connection.
  const brokenResponse = (text: string, total: number) => {
    let sentFirst = false;
    return new Response(
      new ReadableStream<Uint8Array>({
        // Deliver the bytes on the first read and fail on the next, as a dropped
        // connection does (erroring straight after enqueue would drop the bytes).
        pull(controller) {
          if (sentFirst) controller.error(new Error("connection reset"));
          else controller.enqueue(new TextEncoder().encode(text));
          sentFirst = true;
        },
      }),
      { status: 200, headers: { "Content-Length": String(total), "Content-Disposition": 'attachment; filename="r.bin"' } },
    );
  };
  const blobText = (blob: Blob) =>
    new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(reader.error);
      reader.readAsText(blob);
    });

  it("resumes a broken download with a Range request and joins the bytes", async () => {
    const getResponse = vi
      .fn()
      .mockResolvedValueOnce(brokenResponse("0123", 10))
      .mockResolvedValueOnce(new Response("456789", { status: 206 }));
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save);
    expect(getResponse).toHaveBeenCalledTimes(2);
    expect(getResponse.mock.calls[1][1]).toEqual({ headers: { Range: "bytes=4-" } });
    expect(await blobText(save.mock.calls[0][0] as Blob)).toBe("0123456789");
    expect(save.mock.calls[0][1]).toBe("r.bin");
  });

  it("starts over when the resumed response is a 200 instead of a 206", async () => {
    const getResponse = vi
      .fn()
      .mockResolvedValueOnce(brokenResponse("0123", 10))
      .mockResolvedValueOnce(new Response("0123456789", { status: 200 }));
    const save = vi.fn();
    await downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save);
    expect(await blobText(save.mock.calls[0][0] as Blob)).toBe("0123456789");
  });

  it("gives up with an interruption error after three failed attempts", async () => {
    const getResponse = vi.fn().mockImplementation(async () => brokenResponse("01", 10));
    const save = vi.fn();
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), getResponse), { type: "work_item", id: ID }, save),
    ).rejects.toThrow("The download was interrupted.");
    expect(getResponse).toHaveBeenCalledTimes(3);
    expect(save).not.toHaveBeenCalled();
  });

  it("throws the backend's message for a missing or removed file", async () => {
    const save = vi.fn();
    const missing = vi.fn().mockRejectedValue(apiError(404, "attachment not found"));
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), missing), { type: "work_item", id: ID }, save),
    ).rejects.toThrow("attachment not found");
    const gone = vi.fn().mockRejectedValue(apiError(410, "This attachment was removed."));
    await expect(
      downloadEntityAttachment(apiWith(vi.fn(), vi.fn(), gone), { type: "work_item", id: ID }, save),
    ).rejects.toThrow("This attachment was removed.");
    expect(save).not.toHaveBeenCalled();
  });
});
