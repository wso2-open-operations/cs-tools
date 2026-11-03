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

type TusOptions = {
  endpoint: string;
  metadata: Record<string, string>;
  chunkSize?: number;
  retryDelays: number[];
  storeFingerprintForResuming: boolean;
  removeFingerprintOnSuccess: boolean;
  uploadDataDuringCreation?: boolean;
  headers?: unknown;
  onProgress: (sent: number, total: number) => void;
  onError: (error: unknown) => void;
  onSuccess: () => void;
  onShouldRetry: (
    error: { originalResponse?: { getStatus: () => number } },
    attempt: number,
  ) => boolean;
};

const tus = vi.hoisted(() => ({
  instances: [] as Array<{
    file: unknown;
    options: TusOptions;
    url: string | null;
    start: ReturnType<typeof vi.fn>;
    abort: ReturnType<typeof vi.fn>;
  }>,
}));

vi.mock("tus-js-client", () => ({
  Upload: class {
    url: string | null = null;
    start = vi.fn();
    abort = vi.fn().mockResolvedValue(undefined);
    constructor(
      public file: unknown,
      public options: TusOptions,
    ) {
      tus.instances.push(this);
    }
  },
}));

import {
  downloadStoredAttachment,
  TUS_CREATE_FAILED_MESSAGE,
  TUS_INTERRUPTED_MESSAGE,
  uploadStoredAttachment,
} from "@api/storedAttachments";

const BASE = "https://api.test";
const SECRET = "share-secret-123";
const UPLOAD_URL = "https://sftpgo.test/api/v2/shares-chunked-uploads";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const tusStart = {
  attachmentId: "att-1",
  uploadUrl: UPLOAD_URL,
  fieldName: "filenames",
  uploadMethod: "tus",
  shareId: SECRET,
  path: "/upload.bin",
  chunkSize: 8388608,
  maxBytes: 10 * 1024 ** 3,
};
const confirmed = { id: "att-1", name: "big.bin", sizeBytes: 5, status: "COMPLETE" };

describe("storedAttachments TUS upload and large download", () => {
  const authFetch = vi.fn();
  const rawFetch = vi.fn();
  const file = new File(["hello"], "big.bin");

  beforeEach(() => {
    tus.instances.length = 0;
    authFetch.mockReset();
    rawFetch.mockReset();
    vi.stubGlobal("fetch", rawFetch);
    (window as unknown as { config: unknown }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: `${BASE}/`,
      CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 10240,
    };
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  const run = (extra: Record<string, unknown> = {}) =>
    uploadStoredAttachment(authFetch, {
      type: "work_item",
      parentId: "case-1",
      file,
      name: "big.bin",
      ...extra,
    });
  const tick = () => new Promise((r) => setTimeout(r, 0));

  it("uses tus-js-client for uploadMethod tus, then confirms", async () => {
    authFetch
      .mockResolvedValueOnce(json(tusStart, 201))
      .mockResolvedValueOnce(json(confirmed));
    const steps: string[] = [];
    const progress: Array<[number, number]> = [];
    const promise = run({
      onStep: (s: string) => steps.push(s),
      onProgress: (a: number, b: number) => progress.push([a, b]),
    });
    await tick();

    expect(tus.instances).toHaveLength(1);
    const up = tus.instances[0];
    expect(up.file).toBe(file);
    expect(up.options.endpoint).toBe(UPLOAD_URL);
    expect(up.options.metadata).toEqual({ path: "/upload.bin", share_id: SECRET });
    expect(Object.keys(up.options.metadata).sort()).toEqual(["path", "share_id"]);
    expect(up.options.chunkSize).toBe(8388608);
    expect(up.options.retryDelays).toEqual([0, 1000, 3000, 5000, 10000]);
    expect(up.options.storeFingerprintForResuming).toBe(false);
    expect(up.options.removeFingerprintOnSuccess).toBe(true);
    expect(up.options.uploadDataDuringCreation).toBeUndefined();
    expect(up.options.headers).toBeUndefined();
    expect(up.start).toHaveBeenCalledTimes(1);
    expect(authFetch).toHaveBeenCalledTimes(1); // not confirmed yet
    expect(rawFetch).not.toHaveBeenCalled();

    up.options.onProgress(5, 10);
    up.options.onSuccess();
    const done = await promise;

    expect(done.status).toBe("COMPLETE");
    expect(progress).toEqual([[5, 10]]);
    expect(steps).toEqual(["preparing", "uploading", "finishing"]);
    expect(authFetch.mock.calls[1][0]).toBe(`${BASE}/attachments/upload/att-1/confirm`);
  });

  it.each([undefined, "multipart"])(
    "does not use tus when uploadMethod is %s",
    async (method) => {
      authFetch
        .mockResolvedValueOnce(
          json(
            {
              attachmentId: "att-1",
              uploadUrl: "https://files.test/up",
              fieldName: "filenames",
              maxBytes: 10 * 1024 ** 3,
              ...(method ? { uploadMethod: method } : {}),
            },
            201,
          ),
        )
        .mockResolvedValueOnce(json(confirmed));
      rawFetch.mockResolvedValueOnce(new Response("{}", { status: 201 }));
      await run();
      expect(tus.instances).toHaveLength(0);
      expect(rawFetch).toHaveBeenCalledTimes(1);
      expect(rawFetch.mock.calls[0][1].method).toBe("POST");
    },
  );

  it("says the link was used or expired when the upload cannot be created, never leaking the share id", async () => {
    authFetch.mockResolvedValueOnce(json(tusStart, 201));
    const promise = run();
    const settled = promise.then(
      () => null,
      (e: Error) => e,
    );
    await tick();
    // No upload URL yet: the creation request failed.
    tus.instances[0].options.onError(
      new Error(`500 unable to validate share ${SECRET} at ${UPLOAD_URL}`),
    );
    const error = (await settled)!;
    expect(error.message).toBe(
      "The upload link has been used or has expired. Please start the upload again.",
    );
    expect(error.message).toBe(TUS_CREATE_FAILED_MESSAGE);
    expect(JSON.stringify(error)).not.toContain(SECRET);
    expect(String(error.stack)).not.toContain(SECRET);
    expect(authFetch).toHaveBeenCalledTimes(1); // no confirm
  });

  it("says the upload was interrupted for a failure after it was created", async () => {
    authFetch.mockResolvedValueOnce(json(tusStart, 201));
    const settled = run().then(
      () => null,
      (e: Error) => e,
    );
    await tick();
    const up = tus.instances[0];
    up.url = "https://sftpgo.test/api/v2/shares-chunked-uploads/abc";
    up.options.onError(new Error("network"));
    const error = (await settled)!;
    expect(error.message).toBe("The upload was interrupted. Please try again.");
    expect(error.message).toBe(TUS_INTERRUPTED_MESSAGE);
    expect(authFetch).toHaveBeenCalledTimes(1);
  });

  it("never retries a creation the server answered, but retries chunk 5xx and network errors", async () => {
    authFetch.mockResolvedValueOnce(json(tusStart, 201));
    void run().catch(() => undefined);
    await tick();
    const up = tus.instances[0];
    const resp = (status: number) => ({ originalResponse: { getStatus: () => status } });
    expect(up.options.onShouldRetry(resp(500), 0)).toBe(false);
    expect(up.options.onShouldRetry(resp(403), 0)).toBe(false);
    expect(up.options.onShouldRetry({}, 0)).toBe(true);
    up.url = "https://sftpgo.test/api/v2/shares-chunked-uploads/abc";
    expect(up.options.onShouldRetry(resp(500), 0)).toBe(true);
    expect(up.options.onShouldRetry(resp(404), 0)).toBe(false);
  });

  it("aborts the tus upload when the signal fires, without confirming", async () => {
    authFetch.mockResolvedValueOnce(json(tusStart, 201));
    const controller = new AbortController();
    const settled = run({ signal: controller.signal }).then(
      () => null,
      (e: Error) => e,
    );
    await tick();
    controller.abort();
    const error = (await settled)!;
    expect(error.name).toBe("AbortError");
    expect(tus.instances[0].abort).toHaveBeenCalledTimes(1);
    expect(tus.instances[0].abort).toHaveBeenCalledWith(false);
    expect(authFetch).toHaveBeenCalledTimes(1);
  });

  it("does not start tus when already aborted before the link arrived", async () => {
    authFetch.mockResolvedValueOnce(json(tusStart, 201));
    const controller = new AbortController();
    controller.abort();
    await expect(run({ signal: controller.signal })).rejects.toMatchObject({
      name: "AbortError",
    });
    expect(tus.instances).toHaveLength(0);
  });

  describe("large download", () => {
    let clicked: HTMLAnchorElement | null;
    beforeEach(() => {
      clicked = null;
      URL.createObjectURL = vi.fn(() => "blob:test");
      vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(
        function (this: HTMLAnchorElement) {
          clicked = this;
        },
      );
    });

    it("hands a largeFile link to the browser without fetching it", async () => {
      authFetch.mockResolvedValueOnce(
        json({
          links: [
            { id: "a-1", name: "big.iso", available: true, largeFile: true, url: "https://files.test/s/1" },
          ],
        }),
      );
      await downloadStoredAttachment(authFetch, { type: "work_item", id: "a-1" });
      expect(rawFetch).not.toHaveBeenCalled();
      expect(URL.createObjectURL).not.toHaveBeenCalled();
      expect(clicked?.href).toBe("https://files.test/s/1");
      expect(clicked?.hasAttribute("target")).toBe(false);
      expect(clicked?.hasAttribute("download")).toBe(false);
    });

    it("still fetches a link that is not a large file", async () => {
      authFetch.mockResolvedValueOnce(
        json({ links: [{ id: "a-1", available: true, largeFile: false, url: "https://f/x" }] }),
      );
      rawFetch.mockResolvedValueOnce(new Response("ok", { status: 200 }));
      await downloadStoredAttachment(authFetch, { type: "work_item", id: "a-1" });
      expect(rawFetch).toHaveBeenCalledTimes(1);
      expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
    });
  });
});
