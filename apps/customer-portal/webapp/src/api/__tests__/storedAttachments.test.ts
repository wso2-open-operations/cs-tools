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
  requestAttachmentLink,
  sendFileToUploadLink,
  STORED_ATTACHMENT_MAX_BYTES,
  uploadStoredAttachment,
  UploadConfirmPendingError,
} from "@api/storedAttachments";
import { ApiError } from "@utils/ApiError";

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
      maxBytes: STORED_ATTACHMENT_MAX_BYTES,
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
      const big = { size: STORED_ATTACHMENT_MAX_BYTES + 1, name: "big.bin", type: "" } as File;
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
    authFetch.mockResolvedValueOnce(new Response("", { status: 503 }));
    expect(await fetchInlineImageBlob(authFetch, "i-1")).toEqual({ status: "error" });
  });
});
