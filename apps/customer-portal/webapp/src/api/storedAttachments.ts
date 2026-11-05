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

import { isBackendAttachmentStorage } from "@config/entityAttachmentsConfig";
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

/** Why a download link is not available, as entity-service reports it. */
export type AttachmentUnavailableReason =
  | "DELETED"
  | "FAILED"
  | "NOT_STORED"
  | "STORAGE_UNAVAILABLE";

/** One entry of POST /attachments/download. */
export interface StoredAttachmentLink {
  id: string;
  number?: string;
  name?: string;
  contentType?: string;
  sizeBytes?: number;
  available: boolean;
  reason?: AttachmentUnavailableReason | string;
  url?: string;
  expiresAt?: string;
}

/**
 * Response of POST /attachments/upload. With the files held by the portal
 * backend (see `isBackendAttachmentStorage`) there is no `uploadUrl`: the file
 * is sent to the backend in chunks instead.
 */
export interface StoredAttachmentUploadStart {
  attachmentId: string;
  number?: string;
  uploadUrl?: string;
  fieldName?: string;
  expiresAt?: string;
  maxBytes: number;
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

const UNAVAILABLE_REASON_MESSAGES: Record<string, string> = {
  DELETED: "This attachment has been deleted.",
  FAILED: "This attachment could not be stored.",
  NOT_STORED: "This attachment is not available yet.",
  STORAGE_UNAVAILABLE:
    "File storage is temporarily unavailable. Please try again in a moment.",
};

/** Raised when a confirmed upload could not be finished; the file is still staged. */
export class UploadConfirmPendingError extends Error {
  public readonly attachmentId: string;
  public readonly type: StoredAttachmentType;

  constructor(
    type: StoredAttachmentType,
    attachmentId: string,
    message: string,
  ) {
    super(message);
    this.name = "UploadConfirmPendingError";
    this.type = type;
    this.attachmentId = attachmentId;
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
): Promise<Response> {
  const response = await authFetch(`${getBackendBaseUrl()}${path}`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  if (!response.ok) throw await toApiError(response);
  return response;
}

/**
 * Asks for the link to one attachment's file. The link is single-use and lasts
 * about a minute, so it must be fetched right after this returns and never kept.
 * Returns undefined when entity-service has no such attachment of this type (a
 * hidden one, or one stored by the older flow).
 */
export async function requestAttachmentLink(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    id: string;
    purpose?: "download" | "preview";
  },
): Promise<StoredAttachmentLink | undefined> {
  const response = await postJson(authFetch, "/attachments/download", {
    type: input.type,
    ids: [input.id],
    purpose: input.purpose ?? "download",
  });
  const data = (await response.json()) as { links?: StoredAttachmentLink[] };
  const wanted = input.id.toLowerCase();
  return (data.links ?? []).find((l) => l.id?.toLowerCase() === wanted);
}

/** The message shown for a link that is not available. */
export function unavailableMessage(reason?: string): string {
  return (
    (reason && UNAVAILABLE_REASON_MESSAGES[reason]) ||
    "This attachment is not available."
  );
}

/** Fetches a link's file straight from SFTPGo: no app credentials are sent. */
async function fetchLinkBlob(url: string): Promise<Blob> {
  const response = await fetch(url, { credentials: "omit" });
  if (!response.ok) {
    throw new ApiError(
      response.status,
      response.statusText,
      `The file could not be downloaded (HTTP ${response.status}).`,
    );
  }
  return response.blob();
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
async function readParts(response: Response): Promise<Uint8Array[]> {
  const reader = response.body?.getReader();
  if (!reader) return [new Uint8Array(await response.arrayBuffer())];
  const parts: Uint8Array[] = [];
  for (;;) {
    const { done, value } = await reader.read();
    if (done) return parts;
    parts.push(value);
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
 */
async function readRange(
  authFetch: AuthFetch,
  url: string,
  start: number,
  end: number,
  isWhole: (read: RangeRead) => boolean,
): Promise<RangeRead> {
  for (let attempt = 1; ; attempt++) {
    try {
      const response = await authFetch(url, {
        method: "GET",
        headers: { Range: `bytes=${start}-${end}` },
      });
      if (!response.ok) throw await toApiError(response);
      const parts = await readParts(response);
      const read = { response, parts };
      if (isWhole(read)) return read;
    } catch (error) {
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
 *
 * Throws an ApiError with status 404 when there is no such stored attachment,
 * which a caller still on the older flow can fall back from. `onSaved` runs once
 * the download ends, saying whether it was written to the picked file (`saved`)
 * or handed to the browser (`started`); a closed save dialog or a failure never
 * calls it.
 */
async function downloadFromBackend(
  authFetch: AuthFetch,
  input: { type: StoredAttachmentType; id: string; name?: string; size?: number; onSaved?: (outcome: DownloadSaveOutcome) => void; onProgress?: DownloadProgress },
): Promise<void> {
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
  input: { type: StoredAttachmentType; id: string; name?: string; onProgress?: DownloadProgress },
  sink: DiskSink | undefined,
): Promise<void> {
  const url = `${getBackendBaseUrl()}/attachments/download/${encodeURIComponent(
    toHyphenatedUuid(input.id),
  )}/content?type=${encodeURIComponent(input.type)}`;
  const first = await readRange(authFetch, url, 0, DOWNLOAD_RANGE_BYTES - 1, ({ response, parts }) => {
    if (response.status !== 206) return true; // the whole file, the range was ignored
    const range = parseContentRange(response.headers);
    return (
      range !== null &&
      range.start === 0 &&
      range.end === Math.min(DOWNLOAD_RANGE_BYTES, range.total) - 1 &&
      partsLength(parts) === range.end + 1
    );
  });
  const fileName = input.name || fileNameFromDisposition(first.response.headers);
  const total =
    first.response.status === 206 ? parseContentRange(first.response.headers)?.total : undefined;
  const ranges: Uint8Array[][] = [];
  if (sink) await sink.write(0, first.parts);
  else ranges[0] = first.parts;

  let received = partsLength(first.parts);
  if (total !== undefined) input.onProgress?.(received, total);
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
          );
          if (sink) await sink.write(start, parts);
          else ranges[index] = parts;
          received += end - start + 1;
          input.onProgress?.(received, total);
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
 * Downloads one attachment: asks for a link, fetches it at once and saves the
 * bytes under the attachment's own name (the stored file's name carries an
 * internal prefix and a cross-origin `<a download>` would be ignored). If
 * fetching the link fails, one fresh link is requested and tried once more.
 *
 * Throws an ApiError with status 404 when entity-service has no such stored
 * attachment, which a caller still on the older flow can fall back from.
 */
export async function downloadStoredAttachment(
  authFetch: AuthFetch,
  input: { type: StoredAttachmentType; id: string; name?: string; size?: number; onSaved?: (outcome: DownloadSaveOutcome) => void; onProgress?: DownloadProgress },
): Promise<void> {
  if (isBackendAttachmentStorage()) {
    await downloadFromBackend(authFetch, input);
    return;
  }
  const link = await requestAttachmentLink(authFetch, {
    type: input.type,
    id: input.id,
    purpose: "download",
  });
  if (!link) {
    throw new ApiError(404, "Not Found", "Attachment not found.");
  }
  if (!link.available || !link.url) {
    throw new ApiError(
      link.reason === "STORAGE_UNAVAILABLE" ? 503 : 410,
      "Unavailable",
      unavailableMessage(link.reason),
    );
  }

  let blob: Blob;
  try {
    blob = await fetchLinkBlob(link.url);
  } catch {
    const fresh = await requestAttachmentLink(authFetch, {
      type: input.type,
      id: input.id,
      purpose: "download",
    });
    if (!fresh?.available || !fresh.url) {
      throw new ApiError(
        410,
        "Unavailable",
        unavailableMessage(fresh?.reason),
      );
    }
    blob = await fetchLinkBlob(fresh.url);
  }
  saveBlobAs(blob, link.name || input.name || "attachment");
  input.onSaved?.("started");
}

/** Step 1 of an upload: records the attachment and returns the one-use upload link. */
export async function startStoredUpload(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    parentId: string;
    name: string;
    contentType?: string;
    sizeBytes: number;
  },
): Promise<StoredAttachmentUploadStart> {
  const response = await postJson(authFetch, "/attachments/upload", {
    type: input.type,
    parentId: input.parentId,
    name: input.name,
    ...(input.contentType ? { contentType: input.contentType } : {}),
    sizeBytes: input.sizeBytes,
  });
  return (await response.json()) as StoredAttachmentUploadStart;
}

/**
 * Step 2: the browser sends the file straight to SFTPGo as multipart form data,
 * one file in `fieldName`. No app credentials or auth headers go with it.
 */
export async function sendFileToUploadLink(
  start: { uploadUrl: string; fieldName?: string },
  file: File,
): Promise<void> {
  const form = new FormData();
  form.append(start.fieldName || "filenames", file, file.name);
  const response = await fetch(start.uploadUrl, {
    method: "POST",
    body: form,
    credentials: "omit",
  });
  if (!response.ok) {
    throw new ApiError(
      response.status,
      response.statusText,
      `The file could not be uploaded (HTTP ${response.status}). Please try again.`,
    );
  }
}

/** Step 3: entity-service checks what arrived and makes the attachment visible. Safe to repeat. */
export async function confirmStoredUpload(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    attachmentId: string;
    /** Number of chunks sent, when the files are held by the portal backend. */
    chunks?: number;
  },
): Promise<StoredAttachmentConfirmed> {
  const response = await postJson(
    authFetch,
    `/attachments/upload/${encodeURIComponent(input.attachmentId)}/confirm`,
    {
      type: input.type,
      ...(input.chunks !== undefined ? { chunks: input.chunks } : {}),
    },
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

/** True for the error thrown when an upload is cancelled. */
export function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === "AbortError";
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
 * Step 2 when the portal backend holds the files: the file is cut into
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
  let next = 0;
  let sent = 0;
  // The first failure, kept so it is the one thrown, and so the other workers
  // stop taking new chunks. A chunk already in flight is left to finish.
  let failure: { error: unknown } | undefined;
  const worker = async () => {
    while (!failure) {
      const index = next++;
      if (index >= count) return;
      const from = index * UPLOAD_CHUNK_BYTES;
      const body = file.slice(from, Math.min(from + UPLOAD_CHUNK_BYTES, file.size));
      try {
        await sendChunk(authFetch, { type, attachmentId, index, body, signal });
      } catch (error) {
        failure ??= { error };
        return;
      }
      sent += body.size;
      onProgress?.(sent, file.size);
    }
  };
  const workers = Array.from(
    { length: Math.min(UPLOAD_CHUNK_CONCURRENCY, count) },
    worker,
  );
  await Promise.all(workers);
  if (failure) throw failure.error;
  return count;
}

export type StoredUploadStep = "preparing" | "uploading" | "finishing";

/**
 * Uploads one file in three steps (start, send, confirm), reporting each step:
 * to SFTPGo as one request, or to the portal backend in chunks. If only the confirm step fails, an UploadConfirmPendingError is
 * thrown: the file is already in place, so retry with confirmStoredUpload and
 * do not upload again.
 */
export async function uploadStoredAttachment(
  authFetch: AuthFetch,
  input: {
    type: StoredAttachmentType;
    parentId: string;
    file: File;
    name: string;
    onStep?: (step: StoredUploadStep) => void;
    /** Cancels a chunked upload. */
    signal?: AbortSignal;
    /** Bytes stored so far, for a chunked upload only. */
    onProgress?: (bytesSent: number, bytesTotal: number) => void;
  },
): Promise<StoredAttachmentConfirmed> {
  const { type, parentId, file, name, onStep, signal, onProgress } = input;
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
  const start = await startStoredUpload(authFetch, {
    type,
    parentId,
    name,
    contentType: file.type || undefined,
    sizeBytes: file.size,
  });
  if (start.maxBytes > 0 && file.size > start.maxBytes) {
    throw new Error(
      `File size exceeds the ${formatLimit(start.maxBytes)} limit.`,
    );
  }
  if (signal?.aborted) throw abortError();
  onStep?.("uploading");
  let chunks: number | undefined;
  if (start.uploadUrl) {
    await sendFileToUploadLink({ ...start, uploadUrl: start.uploadUrl }, file);
  } else {
    chunks = await sendFileInChunks(authFetch, {
      type,
      attachmentId: start.attachmentId,
      file,
      signal,
      onProgress,
    });
  }
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

/** Outcome of loading a comment's inline image through entity-service. */
export type InlineImageResult =
  | { status: "ok"; blob: Blob }
  | { status: "not-found" }
  | { status: "removed" }
  | { status: "error" };

/**
 * Fetches a comment's inline image through entity-service (which checks access and
 * streams it). "not-found" (a 404) lets a caller fall back to the older flow;
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
