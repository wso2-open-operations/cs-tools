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

import { describe, expect, it, vi } from "vitest";

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
  EntityAttachmentUnavailableError,
  EntityUploadConfirmError,
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
