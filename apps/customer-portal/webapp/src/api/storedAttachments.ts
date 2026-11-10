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

import {
  downloadParallelism,
  entityAttachmentMaxBytes,
  formatLimit,
  inlineImageMaxBytes,
} from "@config/attachmentLimitsConfig";
import { ApiError, parseApiResponseMessage } from "@utils/ApiError";
import { openDiskTarget, type DiskSink } from "@api/downloadToDisk";

/**
 * How a finished download ended: `saved` when the whole file was written to the
 * file the person picked (observable), `started` when the file was handed to the
 * browser as a download, which cannot tell whether the person saves it.
 */
export type DownloadSaveOutcome = "saved" | "started";

/** Called as a download arrives: bytes received so far and the file's total size, once the first range is back. */
export type DownloadProgress = (received: number, total: number) => void;

/** Which attachment table a stored attachment lives in. */
export type StoredAttachmentType = "work_item" | "comment" | "deployment";

/**
 * Response of POST /attachments/upload: the file is then sent to the backend in
 * chunks, or, when `directUpload` is true (a large file, with direct transfers
 * on), to Azure through signed links.
 */
export interface StoredAttachmentUploadStart {
  attachmentId: string;
  number?: string;
  expiresAt?: string;
  maxBytes: number;
  directUpload?: boolean;
}

/** Response of POST /attachments/upload/{id}/confirm. */
export interface StoredAttachmentConfirmed {
  id: string;
  number?: string;
  name: string;
  contentType?: string;
  sizeBytes: number;
  status: string;
}

/** Performs an authenticated request; the shape `useAuthApiClient` returns. */
export type AuthFetch = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export const STORED_ATTACHMENT_RATE_LIMIT_MESSAGE =
  "Too many attachment requests. Please wait a minute and try again.";

/** Raised when a confirmed upload could not be finished; the chunks are still stored. */
export class UploadConfirmPendingError extends Error {
  public readonly attachmentId: string;
  public readonly type: StoredAttachmentType;
  /** How many chunks were stored; the retried confirm must name the same count. */
  public readonly chunks: number;

  constructor(
    type: StoredAttachmentType,
    attachmentId: string,
    message: string,
    chunks: number,
  ) {
    super(message);
    this.name = "UploadConfirmPendingError";
    this.type = type;
    this.attachmentId = attachmentId;
    this.chunks = chunks;
  }
}

/** Bytes in every upload chunk but the last. Must match the backend's chunk size. */
export const UPLOAD_CHUNK_BYTES = 8 * 1024 * 1024;

/** Chunks sent at the same time, and tries per chunk. */
const UPLOAD_CHUNK_CONCURRENCY = 3;
const UPLOAD_CHUNK_ATTEMPTS = 4;

/** Shown when a chunked upload failed after its retries ran out. */
export const UPLOAD_INTERRUPTED_MESSAGE =
  "The upload was interrupted. Please try again.";

function getBackendBaseUrl(): string {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }
  return baseUrl.replace(/\/$/, "");
}

/** Turns a failed response into an ApiError carrying the server's own message. */
async function toApiError(response: Response): Promise<ApiError> {
  const text = await response.text().catch(() => "");
  const message =
    response.status === 429
      ? STORED_ATTACHMENT_RATE_LIMIT_MESSAGE
      : parseApiResponseMessage(text, response.status, response.statusText);
  return new ApiError(
    response.status,
    response.statusText,
    message,
    response.headers?.get?.("X-CSM-Correlation-ID") ?? undefined,
  );
}

async function postJson(
  authFetch: AuthFetch,
  path: string,
  body: unknown,
  signal?: AbortSignal,
): Promise<Response> {
  const response = await authFetch(`${getBackendBaseUrl()}${path}`, {
    method: "POST",
    body: JSON.stringify(body),
    ...(signal ? { signal } : {}),
  });
  if (!response.ok) throw await toApiError(response);
  return response;
}

/** Saves a blob under a chosen file name through a temporary object URL. */
export function saveBlobAs(blob: Blob, fileName: string): void {
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectUrl;
  link.download = fileName || "attachment";
  link.rel = "noopener noreferrer";
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 10_000);
}

/** Tries to read one range of a download before giving up. */
const DOWNLOAD_RANGE_ATTEMPTS = 3;

/** Bytes asked for in one range of a download. */
export const DOWNLOAD_RANGE_BYTES = 8 * 1024 * 1024;

const DOWNLOAD_INTERRUPTED_MESSAGE = "The download was interrupted. Please try again.";

/** Reads a response body to the end, keeping the pieces as they arrive. */
async function readParts(
  response: Response,
  onBytes?: (bytes: number) => void,
): Promise<Uint8Array[]> {
  const reader = response.body?.getReader();
  if (!reader) {
    const whole = new Uint8Array(await response.arrayBuffer());
    onBytes?.(whole.length);
    return [whole];
  }
  const parts: Uint8Array[] = [];
  for (;;) {
    const { done, value } = await reader.read();
    if (done) return parts;
    parts.push(value);
    onBytes?.(value.length);
  }
}

const partsLength = (parts: Uint8Array[]): number =>
  parts.reduce((sum, part) => sum + part.length, 0);

/** The `bytes a-b/total` of a `Content-Range` header, or null when it is not that. */
function parseContentRange(
  headers: Headers,
): { start: number; end: number; total: number } | null {
  const match = /^bytes\s+(\d+)-(\d+)\/(\d+)$/i.exec(
    (headers.get("Content-Range") ?? "").trim(),
  );
  if (!match) return null;
  const [start, end, total] = [Number(match[1]), Number(match[2]), Number(match[3])];
  return end >= start && total > end ? { start, end, total } : null;
}

interface RangeRead {
  response: Response;
  parts: Uint8Array[];
}

/**
 * Asks for bytes `start` to `end` of the file and reads them in full. A dropped
 * connection or a short body is tried again, up to three times; a refusal from
 * the backend is thrown at once, and so is a cancelled request.
 *
 * `onBytes` is told of the bytes of each piece as it arrives, with the response
 * it belongs to. What a failed attempt had reported is taken back (a negative
 * number) before the next one starts, so a retried range is never counted twice.
 */
async function readRange(
  authFetch: AuthFetch,
  url: string,
  start: number,
  end: number,
  isWhole: (read: RangeRead) => boolean,
  onBytes?: (bytes: number, response?: Response) => void,
  signal?: AbortSignal,
): Promise<RangeRead> {
  for (let attempt = 1; ; attempt++) {
    if (signal?.aborted) throw downloadCancelledError();
    let reported = 0;
    const takeBackReported = () => {
      if (reported > 0) onBytes?.(-reported);
      reported = 0;
    };
    try {
      const response = await authFetch(url, {
        method: "GET",
        headers: { Range: `bytes=${start}-${end}` },
        signal,
      });
      if (!response.ok) throw await toApiError(response);
      const parts = await readParts(response, (bytes) => {
        reported += bytes;
        onBytes?.(bytes, response);
      });
      const read = { response, parts };
      if (isWhole(read)) return read;
      takeBackReported();
    } catch (error) {
      takeBackReported();
      if (signal?.aborted) throw downloadCancelledError();
      if (error instanceof ApiError || isAbortError(error)) throw error;
    }
    if (attempt >= DOWNLOAD_RANGE_ATTEMPTS) {
      throw new ApiError(0, "Download failed", DOWNLOAD_INTERRUPTED_MESSAGE);
    }
  }
}

/**
 * Downloads one attachment through the portal backend, which checks access and
 * streams the file. The first 8 MiB are asked for with a `Range` header; the
 * answer says how big the file is, and the rest is fetched as 8 MiB ranges,
 * {@link downloadParallelism} at a time.
 *
 * Where the browser can, the file is written straight to a file the person
 * picked, each range at its own offset as it arrives, so it is never held whole
 * in memory; otherwise the ranges are joined in order and saved as one Blob. A
 * range that breaks is fetched again on its own; a refusal stops the other
 * ranges. A closed save dialog ends the download with nothing saved and no error.
 * `signal` cancels it: the requests in flight are aborted, a half-written picked
 * file is removed, and an AbortError ({@link isAbortError}) is thrown.
 *
 * Throws an ApiError with status 404 when there is no such stored attachment,
 * which a caller still on the older flow can fall back from. `onSaved` runs once
 * the download ends, saying whether it was written to the picked file (`saved`)
 * or handed to the browser (`started`); a closed save dialog or a failure never
 * calls it.
 */
async function downloadFromBackend(
  authFetch: AuthFetch,
  input: { type: StoredAttachmentType; id: string; name?: string; size?: number; onSaved?: (outcome: DownloadSaveOutcome) => void; onProgress?: DownloadProgress; signal?: AbortSignal },
): Promise<void> {
  if (input.signal?.aborted) throw downloadCancelledError();
  const target = await openDiskTarget(input.name || "attachment", input.size);
  if (target.kind === "cancelled") return;
  const sink = target.kind === "sink" ? target.sink : undefined;
  try {
    await readAndStore(authFetch, input, sink);
    input.onSaved?.(sink ? "saved" : "started");
  } catch (error) {
    await sink?.abort();
    throw error;
  }
}

async function readAndStore(
  authFetch: AuthFetch,
  input: { type: StoredAttachmentType; id: string; name?: string; onProgress?: DownloadProgress; signal?: AbortSignal },
  sink: DiskSink | undefined,
): Promise<void> {
  const url = `${getBackendBaseUrl()}/attachments/download/${encodeURIComponent(
    toHyphenatedUuid(input.id),
  )}/content?type=${encodeURIComponent(input.type)}`;
  // Bytes received so far, counted as each piece arrives; the size of the file
  // is known from the first answer's headers on.
  let received = 0;
  let knownTotal: number | undefined;
  const report = (bytes: number, response?: Response): void => {
    received += bytes;
    if (knownTotal === undefined && response?.status === 206) {
      knownTotal = parseContentRange(response.headers)?.total;
    }
    if (knownTotal !== undefined) input.onProgress?.(Math.max(0, received), knownTotal);
  };
  const first = await readRange(
    authFetch,
    url,
    0,
    DOWNLOAD_RANGE_BYTES - 1,
    ({ response, parts }) => {
      if (response.status !== 206) return true; // the whole file, the range was ignored
      const range = parseContentRange(response.headers);
      return (
        range !== null &&
        range.start === 0 &&
        range.end === Math.min(DOWNLOAD_RANGE_BYTES, range.total) - 1 &&
        partsLength(parts) === range.end + 1
      );
    },
    report,
    input.signal,
  );
  const fileName = input.name || fileNameFromDisposition(first.response.headers);
  const total =
    first.response.status === 206 ? parseContentRange(first.response.headers)?.total : undefined;
  const ranges: Uint8Array[][] = [];
  if (sink) await sink.write(0, first.parts);
  else ranges[0] = first.parts;

  if (total !== undefined && partsLength(first.parts) < total) {
    const count = Math.ceil(total / DOWNLOAD_RANGE_BYTES);
    let next = 1;
    let failure: { error: unknown } | undefined;
    const worker = async (): Promise<void> => {
      while (failure === undefined && next < count) {
        const index = next++;
        const start = index * DOWNLOAD_RANGE_BYTES;
        const end = Math.min(start + DOWNLOAD_RANGE_BYTES, total) - 1;
        try {
          const { parts } = await readRange(
            authFetch,
            url,
            start,
            end,
            ({ response, parts }) => {
              const range = parseContentRange(response.headers);
              return (
                response.status === 206 &&
                range !== null &&
                range.start === start &&
                range.end === end &&
                range.total === total &&
                partsLength(parts) === end - start + 1
              );
            },
            report,
            input.signal,
          );
          if (sink) await sink.write(start, parts);
          else ranges[index] = parts;
        } catch (error) {
          failure ??= { error };
        }
      }
    };
    await Promise.all(
      Array.from({ length: Math.min(downloadParallelism(), count - 1) }, worker),
    );
    if (failure) throw failure.error;
  }
  if (sink) await sink.close();
  else saveBlobAs(new Blob(ranges.flat() as BlobPart[]), fileName || "attachment");
}

/** The file name a download response suggests, if it names one. */
function fileNameFromDisposition(headers: Headers): string | undefined {
  const value = headers.get("Content-Disposition") ?? "";
  const match = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(value);
  if (!match) return undefined;
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return match[1];
  }
}

/**
 * Downloads one attachment through the portal backend, saving the bytes under
 * the attachment's own name.
 *
 * Throws an ApiError with status 404 when entity-service has no such stored
 * attachment, which a caller still on the older flow can fall back from.
 */
export async function downloadStoredAttachment(
  authFetch: AuthFetch,
  input: { type: StoredAttachmentType; id: string; name?: string; size?: number; onSaved?: (outcome: DownloadSaveOutcome) => void; onProgress?: DownloadProgress; signal?: AbortSignal },
): Promise<void> {
  if (input.size !== undefined && input.size > DIRECT_TRANSFER_MIN_BYTES) {
    if (input.signal?.aborted) throw downloadCancelledError();
    if (await startDirectDownload(authFetch, input)) {
      input.onSaved?.("started");
      return;
    }
  }
  await downloadFromBackend(authFetch, input);
}

/**
 * Starts a large file's download straight from Azure through a signed link the
 * backend makes after checking access, which the browser then shows and saves
 * itself (outcome `started`, no in-page progress). Resolves `false` when there
 * is no link (direct transfers are off, or the request failed): the caller then
 * streams the file through the backend, which reports any real refusal (a
 * removed file) itself.
 */
async function startDirectDownload(
  authFetch: AuthFetch,
  input: { type: StoredAttachmentType; id: string; name?: string; signal?: AbortSignal },
): Promise<boolean> {
  try {
    const response = await authFetch(
      `${getBackendBaseUrl()}/attachments/download/${encodeURIComponent(input.id)}/link?type=${encodeURIComponent(input.type)}`,
      { method: "GET", ...(input.signal ? { signal: input.signal } : {}) },
    );
    if (!response.ok) return false;
    const link = (await response.json()) as { url?: string; name?: string };
    if (!link.url) return false;
    startBrowserDownload(link.url, input.name || link.name || "attachment");
    return true;
  } catch {
    return false;
  }
}

/** Hands a signed link to the browser as a download; the link itself says it is an attachment. */
function startBrowserDownload(url: string, name: string): void {
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

/** Step 1 of an upload: records the attachment and returns its id and the size limit. */
export async function startStoredUpload(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    /** The record the attachment belongs to. Give this or `projectId`. */
    parentId?: string;
    /**
     * A comment image uploaded before its record exists (a case being created)
     * belongs to the project instead; give this or `parentId`, never both.
     */
    projectId?: string;
    name: string;
    contentType?: string;
    sizeBytes: number;
    /** Cancels the request. */
    signal?: AbortSignal;
  },
): Promise<StoredAttachmentUploadStart> {
  const response = await postJson(authFetch, "/attachments/upload", {
    type: input.type,
    ...(input.parentId ? { parentId: input.parentId } : {}),
    ...(input.projectId && !input.parentId ? { projectId: input.projectId } : {}),
    name: input.name,
    ...(input.contentType ? { contentType: input.contentType } : {}),
    sizeBytes: input.sizeBytes,
  }, input.signal);
  return (await response.json()) as StoredAttachmentUploadStart;
}

/** Step 3: the backend assembles the chunks and entity-service makes the attachment visible. Safe to repeat. */
export async function confirmStoredUpload(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    attachmentId: string;
    /** Number of chunks sent. */
    chunks: number;
  },
): Promise<StoredAttachmentConfirmed> {
  const response = await postJson(
    authFetch,
    `/attachments/upload/${encodeURIComponent(input.attachmentId)}/confirm`,
    { type: input.type, chunks: input.chunks },
  );
  return (await response.json()) as StoredAttachmentConfirmed;
}

/** Options of a chunked upload. */
export interface ChunkUploadOptions {
  signal?: AbortSignal;
  /** Called after each chunk is stored, with the bytes stored so far. */
  onProgress?: (bytesSent: number, bytesTotal: number) => void;
}

function abortError(): Error {
  const error = new Error("The upload was cancelled.");
  error.name = "AbortError";
  return error;
}

/** True for the error thrown when an upload or a download is cancelled. */
export function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === "AbortError";
}

/** What a cancelled download throws: an AbortError, so {@link isAbortError} knows it. */
function downloadCancelledError(): Error {
  const error = new Error("The download was cancelled.");
  error.name = "AbortError";
  return error;
}

/** Waits `ms`, or throws as soon as the upload is cancelled. */
function pause(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(abortError());
    const onAbort = () => {
      window.clearTimeout(timer);
      reject(abortError());
    };
    const timer = window.setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

/** True for a chunk failure worth trying again (not a refusal of the request). */
function chunkFailureIsTransient(error: unknown): boolean {
  if (error instanceof ApiError) {
    return error.status === 0 || error.status >= 500 || error.status === 408 || error.status === 429;
  }
  return true; // a network error
}

/** Sends one chunk, trying again with a short pause when it fails in passing. */
async function sendChunk(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    attachmentId: string;
    index: number;
    body: Blob;
    signal?: AbortSignal;
  },
): Promise<void> {
  const path = `/attachments/upload/${encodeURIComponent(input.attachmentId)}/chunks/${input.index}?type=${encodeURIComponent(input.type)}`;
  for (let attempt = 1; ; attempt++) {
    if (input.signal?.aborted) throw abortError();
    try {
      const response = await authFetch(`${getBackendBaseUrl()}${path}`, {
        method: "PUT",
        headers: { "Content-Type": "application/octet-stream" },
        body: input.body,
        signal: input.signal,
      });
      if (!response.ok) throw await toApiError(response);
      return;
    } catch (error) {
      if (input.signal?.aborted || isAbortError(error)) throw abortError();
      if (attempt >= UPLOAD_CHUNK_ATTEMPTS || !chunkFailureIsTransient(error)) {
        throw error instanceof ApiError && error.status !== 0
          ? error
          : new ApiError(0, "Upload failed", UPLOAD_INTERRUPTED_MESSAGE);
      }
      await pause(500 * 2 ** (attempt - 1), input.signal);
    }
  }
}

/**
 * Cuts `file` into `count` chunks of `chunkBytes` and sends them a few at a time
 * with `send`. The first failure is kept so it is the one thrown, and so the
 * other workers stop taking new chunks; a chunk already in flight is left to finish.
 */
async function sendChunksInParallel(input: {
  file: File;
  chunkBytes: number;
  count: number;
  send: (index: number, body: Blob) => Promise<void>;
  onProgress?: (bytesSent: number, bytesTotal: number) => void;
}): Promise<void> {
  const { file, chunkBytes, count, send, onProgress } = input;
  let next = 0;
  let sent = 0;
  let failure: { error: unknown } | undefined;
  const worker = async () => {
    while (!failure) {
      const index = next++;
      if (index >= count) return;
      const from = index * chunkBytes;
      const body = file.slice(from, Math.min(from + chunkBytes, file.size));
      try {
        await send(index, body);
      } catch (error) {
        failure ??= { error };
        return;
      }
      sent += body.size;
      onProgress?.(sent, file.size);
    }
  };
  await Promise.all(
    Array.from({ length: Math.min(UPLOAD_CHUNK_CONCURRENCY, count) }, worker),
  );
  if (failure) throw failure.error;
}

/**
 * Step 2: the file is cut into
 * {@link UPLOAD_CHUNK_BYTES} chunks, sent a few at a time (each one retried on
 * a passing failure), and its confirm step names how many there were. Returns
 * that count.
 */
export async function sendFileInChunks(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    attachmentId: string;
    file: File;
  } & ChunkUploadOptions,
): Promise<number> {
  const { type, attachmentId, file, signal, onProgress } = input;
  const count = Math.max(1, Math.ceil(file.size / UPLOAD_CHUNK_BYTES));
  await sendChunksInParallel({
    file,
    chunkBytes: UPLOAD_CHUNK_BYTES,
    count,
    onProgress,
    send: (index, body) =>
      sendChunk(authFetch, { type, attachmentId, index, body, signal }),
  });
  return count;
}

/**
 * Files above this size go to and from Azure through signed links (when the
 * backend says they may), so their bytes do not pass through the portal backend.
 * It is one chunk: smaller files, and every inline image, keep the backend path.
 */
export const DIRECT_TRANSFER_MIN_BYTES = UPLOAD_CHUNK_BYTES;

/** The links `POST /attachments/upload/{id}/direct` returns: one per chunk. */
interface DirectUploadLinks {
  links: string[];
  chunkBytes: number;
  expiresAt?: string;
}

/** How many times the links of one upload are replaced when they expire. */
const MAX_LINK_REFRESHES = 3;

/**
 * Sends one chunk to its signed link, trying again with a short pause when it
 * fails in passing. A 403 means the link expired (they last a few minutes): the
 * links are asked for again and the chunk goes to its new one.
 */
async function sendChunkToLink(
  source: () => { url: string; generation: number },
  onExpired: (generation: number) => Promise<void>,
  body: Blob,
  signal?: AbortSignal,
): Promise<void> {
  for (let attempt = 1; ; attempt++) {
    if (signal?.aborted) throw abortError();
    const { url, generation } = source();
    try {
      // No Authorization header: the link itself is the credential.
      const response = await fetch(url, { method: "PUT", body, signal });
      if (response.ok) return;
      if (response.status === 403) {
        await onExpired(generation); // throws once the links were replaced too often
        attempt--; // a new link is not a failed try
        continue;
      }
      // Azure answers 500/503 when it is busy; any other refusal will not change.
      if (
        attempt >= UPLOAD_CHUNK_ATTEMPTS ||
        ![408, 429, 500, 503].includes(response.status)
      ) {
        throw new ApiError(0, "Upload failed", UPLOAD_INTERRUPTED_MESSAGE);
      }
    } catch (error) {
      if (signal?.aborted || isAbortError(error)) throw abortError();
      if (attempt >= UPLOAD_CHUNK_ATTEMPTS || error instanceof ApiError) {
        throw new ApiError(0, "Upload failed", UPLOAD_INTERRUPTED_MESSAGE);
      }
    }
    await pause(500 * 2 ** (attempt - 1), signal);
  }
}

/**
 * Sends `file` straight to Azure through signed links the backend makes (one per
 * chunk), a few at a time. Returns the number of chunks, which the confirm step
 * needs. The links last a few minutes, longer for a bigger file; one that expires
 * part-way is replaced by asking again, and the chunks already sent stay.
 * Throws when the links cannot be had or a chunk cannot be sent (for example a
 * storage account that does not allow this site); the caller then sends the file
 * through the backend instead, which stages the same chunks.
 */
export async function sendFileDirect(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    attachmentId: string;
    file: File;
  } & ChunkUploadOptions,
): Promise<number> {
  const { type, attachmentId, file, signal, onProgress } = input;
  const path = `/attachments/upload/${encodeURIComponent(attachmentId)}/direct?type=${encodeURIComponent(type)}`;
  const requestLinks = async (): Promise<DirectUploadLinks> =>
    (await (await postJson(authFetch, path, {}, signal)).json()) as DirectUploadLinks;
  let direct = await requestLinks();
  const count = Math.max(1, Math.ceil(file.size / direct.chunkBytes));
  if (!Array.isArray(direct.links) || direct.links.length < count) {
    throw new Error("The upload links do not cover the file.");
  }
  // Every chunk in flight may see the expiry at once; one request replaces the links.
  let generation = 0;
  let refreshing: Promise<void> | undefined;
  const onExpired = async (seen: number): Promise<void> => {
    if (seen !== generation) return; // already replaced
    if (generation >= MAX_LINK_REFRESHES) {
      throw new ApiError(0, "Upload failed", UPLOAD_INTERRUPTED_MESSAGE);
    }
    refreshing ??= requestLinks()
      .then((fresh) => {
        if (
          fresh.chunkBytes !== direct.chunkBytes ||
          !Array.isArray(fresh.links) ||
          fresh.links.length < count
        ) {
          throw new Error("The upload links do not cover the file.");
        }
        direct = fresh;
        generation += 1;
      })
      .finally(() => {
        refreshing = undefined;
      });
    await refreshing;
  };
  await sendChunksInParallel({
    file,
    chunkBytes: direct.chunkBytes,
    count,
    onProgress,
    send: (index, body) =>
      sendChunkToLink(() => ({ url: direct.links[index], generation }), onExpired, body, signal),
  });
  return count;
}

export type StoredUploadStep = "preparing" | "uploading" | "finishing";

/**
 * Uploads one file in three steps (start, send in chunks, confirm), reporting
 * each step. If only the confirm step fails, an UploadConfirmPendingError is
 * thrown: the chunks are already stored, so retry with confirmStoredUpload and
 * do not upload again.
 */
export async function uploadStoredAttachment(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    /** The record to attach to. Give this or `projectId` (see startStoredUpload). */
    parentId?: string;
    projectId?: string;
    file: File;
    name: string;
    onStep?: (step: StoredUploadStep) => void;
    /** Cancels the upload. */
    signal?: AbortSignal;
    /** Bytes stored so far. */
    onProgress?: (bytesSent: number, bytesTotal: number) => void;
  },
): Promise<StoredAttachmentConfirmed> {
  const { type, parentId, projectId, file, name, onStep, signal, onProgress } = input;
  // Only an early refusal: entity-service enforces its own limit, and the
  // `maxBytes` of the upload response is checked below too.
  const localMaxBytes =
    type === "comment" ? inlineImageMaxBytes() : entityAttachmentMaxBytes();
  if (file.size > localMaxBytes) {
    throw new Error(
      `File size exceeds ${formatLimit(localMaxBytes)} limit.`,
    );
  }
  onStep?.("preparing");
  let start: StoredAttachmentUploadStart;
  try {
    start = await startStoredUpload(authFetch, {
      type,
      parentId,
      projectId,
      name,
      contentType: file.type || undefined,
      sizeBytes: file.size,
      signal,
    });
  } catch (error) {
    if (signal?.aborted || isAbortError(error)) throw abortError();
    throw error;
  }
  if (start.maxBytes > 0 && file.size > start.maxBytes) {
    throw new Error(
      `File size exceeds the ${formatLimit(start.maxBytes)} limit.`,
    );
  }
  if (signal?.aborted) throw abortError();
  onStep?.("uploading");
  const send = { type, attachmentId: start.attachmentId, file, signal, onProgress };
  let chunks: number | undefined;
  if (start.directUpload && file.size > DIRECT_TRANSFER_MIN_BYTES) {
    // A large file goes straight to Azure. If that does not work for any reason,
    // the same chunks go through the backend; only a cancel stops the upload.
    try {
      chunks = await sendFileDirect(authFetch, send);
    } catch (error) {
      if (signal?.aborted || isAbortError(error)) throw abortError();
      onProgress?.(0, file.size);
    }
  }
  chunks ??= await sendFileInChunks(authFetch, send);
  onStep?.("finishing");
  try {
    return await confirmStoredUpload(authFetch, {
      type,
      attachmentId: start.attachmentId,
      chunks,
    });
  } catch (error) {
    throw new UploadConfirmPendingError(
      type,
      start.attachmentId,
      error instanceof Error ? error.message : "Could not finish the upload.",
      chunks,
    );
  }
}

/** Deletes a stored attachment (soft delete; only its uploader may). */
export async function deleteStoredAttachment(
  authFetch: AuthFetch,
  input: { type: StoredAttachmentType; id: string },
): Promise<void> {
  await postJson(authFetch, "/attachments/delete", {
    type: input.type,
    id: input.id,
  });
}

/**
 * Entity-service and the backend accept only the hyphenated form of an id, while
 * the comment renderer normalises a bare `/<uuid>` reference to 32 hex
 * characters. Any other value is returned unchanged.
 */
export function toHyphenatedUuid(id: string): string {
  const hex = id.trim().toLowerCase();
  return /^[a-f0-9]{32}$/.test(hex)
    ? `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
    : id;
}

/** Outcome of loading a comment's inline image through the backend. */
export type InlineImageResult =
  | { status: "ok"; blob: Blob }
  | { status: "not-found" }
  | { status: "removed" }
  | { status: "error" };

/**
 * Fetches a comment's inline image through the portal backend (entity-service
 * checks access, the backend streams it). "not-found" (a 404) lets a caller fall back to the older flow;
 * "removed" (a 410, the image was deleted) does not, since there is nothing
 * older to try; any other failure is "error".
 */
export async function fetchInlineImageBlob(
  authFetch: AuthFetch,
  id: string,
): Promise<InlineImageResult> {
  const response = await authFetch(
    `${getBackendBaseUrl()}/attachments/inline/${encodeURIComponent(toHyphenatedUuid(id))}/content`,
    { method: "GET" },
  );
  if (response.status === 404) return { status: "not-found" };
  if (response.status === 410) return { status: "removed" };
  if (!response.ok) return { status: "error" };
  return { status: "ok", blob: await response.blob() };
}
