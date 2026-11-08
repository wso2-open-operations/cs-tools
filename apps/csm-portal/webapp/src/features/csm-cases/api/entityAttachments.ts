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

import { BackendApiError, type BackendApi } from "@api/backend/client";
import { downloadParallelism, formatSizeLimit } from "@config/attachmentLimitsConfig";
import { saveBlob } from "@utils/saveBlob";
import { openDiskTarget, type DiskSink } from "@features/csm-cases/api/downloadToDisk";

/**
 * Entity-service stored attachments, reached through the CSM portal backend,
 * which holds the files in Azure Blob Storage. Plain functions over a
 * {@link BackendApi}, so they are unit-testable without React; the hooks in
 * `useCsmCaseAttachments.ts` bind them to the signed-in session.
 *
 * The browser never talks to the storage itself: an upload goes to the backend
 * in chunks and a download streams back through it, each after the backend has
 * checked access with entity-service.
 */

/**
 * How a finished download ended: `saved` when the whole file was written to the
 * file the person picked (observable), `started` when the file was handed to the
 * browser as a download, which cannot tell whether the person saves it.
 */
export type DownloadSaveOutcome = "saved" | "started";

/** Called as a download arrives: bytes received so far and the file's total size, once the first range is back. */
export type DownloadProgress = (received: number, total: number) => void;

/** Which attachment table a request is about (the request body's `type`). */
export type EntityAttachmentType = "work_item" | "comment" | "deployment";

/**
 * Cases, incidents and change requests are all work items in entity-service, so
 * every attachment on them uses this \`type\`.
 */
export const WORK_ITEM_ATTACHMENT_TYPE: EntityAttachmentType = "work_item";

/**
 * Response of `POST /attachments/upload`. There is no upload address: the file
 * is sent to the portal backend in chunks.
 */
export interface EntityUploadStart {
  attachmentId: string;
  number: string;
  uploadUrl?: string;
  fieldName?: string;
  expiresAt: string;
  maxBytes: number;
}

export interface EntityConfirmedAttachment {
  id: string;
  number: string;
  name: string;
  contentType?: string;
  sizeBytes: number;
  status: string;
}

/** Shown when a user has made too many link requests in a minute (HTTP 429). */
export const ENTITY_ATTACHMENT_RATE_LIMIT_MESSAGE =
  "Too many attachment requests. Please wait a minute and try again.";

/**
 * A readable message for a failed attachment call: the rate limit gets its own
 * wording, anything else is the backend's own message (it is already written
 * for the user, e.g. "This case is closed, so attachments can't be deleted.").
 */
export function entityAttachmentErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof BackendApiError && err.status === 429) {
    return ENTITY_ATTACHMENT_RATE_LIMIT_MESSAGE;
  }
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

/**
 * The file was sent but the final confirm step did not succeed. The chunks are
 * still stored, so `confirmEntityAttachment` can be called again with these ids
 * without sending the file a second time.
 */
export class EntityUploadConfirmError extends Error {
  attachmentId: string;
  type: EntityAttachmentType;
  constructor(attachmentId: string, type: EntityAttachmentType, cause: unknown) {
    super(
      entityAttachmentErrorMessage(
        cause,
        "The file was uploaded but could not be finalised yet. Please try again shortly.",
      ),
    );
    this.name = "EntityUploadConfirmError";
    this.attachmentId = attachmentId;
    this.type = type;
    this.cause = cause;
  }
}

/** Tries to read one range of a download before giving up. */
const DOWNLOAD_RANGE_ATTEMPTS = 3;

/** Bytes asked for in one range of a download. */
export const DOWNLOAD_RANGE_BYTES = 8 * 1024 * 1024;

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

/** The streamed route an attachment's file is read from. */
function attachmentContentPath(args: { type: EntityAttachmentType; id: string }): string {
  return `/attachments/download/${encodeURIComponent(args.id)}/content?type=${encodeURIComponent(args.type)}`;
}

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

const partsLength = (parts: Uint8Array[]): number => parts.reduce((sum, part) => sum + part.length, 0);

/** The `bytes a-b/total` of a `Content-Range` header, or null when it is not that. */
function parseContentRange(headers: Headers): { start: number; end: number; total: number } | null {
  const match = /^bytes\s+(\d+)-(\d+)\/(\d+)$/i.exec((headers.get("Content-Range") ?? "").trim());
  if (!match) return null;
  const [start, end, total] = [Number(match[1]), Number(match[2]), Number(match[3])];
  return end >= start && total > end ? { start, end, total } : null;
}

/**
 * The response headers a browser may hide. Content-Range and Content-Disposition
 * are not readable by page scripts unless the backend (or the gateway in front
 * of it) lists them in Access-Control-Expose-Headers; Content-Length and
 * Content-Type always are. A download must work either way, so what those two
 * headers would say is taken from the attachment itself when they are hidden:
 * the file name from its record, the size from its metadata.
 */
function knownSizeOf(size: number | undefined): number | undefined {
  return size !== undefined && Number.isSafeInteger(size) && size > 0 ? size : undefined;
}

/**
 * Whether the first range is the whole of what it should be when its
 * Content-Range cannot be read. With the size known it must be exactly the
 * first 8 MiB, or the whole file when that is smaller; with no size, any body up
 * to the 8 MiB asked for. A Content-Length that disagrees with the bytes read is
 * a broken connection either way.
 */
function firstRangeIsWhole(response: Response, length: number, size: number | undefined): boolean {
  const declared = response.headers.get("Content-Length");
  if (declared !== null && Number(declared) !== length) return false;
  if (size !== undefined) return length === Math.min(DOWNLOAD_RANGE_BYTES, size);
  // No size: the rest is found by reading on until the file ends ({@link readUntilEnd}).
  // A body cut short by a dropped connection disagrees with its Content-Length, checked above.
  return length > 0 && length <= DOWNLOAD_RANGE_BYTES;
}

interface RangeRead {
  response: Response;
  parts: Uint8Array[];
}

/**
 * Asks for bytes `start` to `end` of the file and reads them in full. A dropped
 * connection or a short body is tried again, up to three times; a refusal from
 * the backend (a {@link BackendApiError}) is thrown at once.
 */
async function readRange(
  api: BackendApi,
  path: string,
  start: number,
  end: number,
  isWhole: (read: RangeRead) => boolean,
): Promise<RangeRead> {
  for (let attempt = 1; ; attempt += 1) {
    try {
      const response = await api.getResponse(path, { headers: { Range: `bytes=${start}-${end}` } });
      const parts = await readParts(response);
      const read = { response, parts };
      if (isWhole(read)) return read;
    } catch (err) {
      if (err instanceof BackendApiError) throw err;
    }
    if (attempt >= DOWNLOAD_RANGE_ATTEMPTS) throw new Error(DOWNLOAD_INTERRUPTED_MESSAGE);
  }
}

/**
 * Fetches an attachment's file as a blob, for an inline preview. The portal
 * backend checks access and streams it; any failure (no such attachment, its
 * file is not stored, a removed file) throws with the backend's message.
 */
export function fetchEntityAttachmentBlob(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string },
): Promise<Blob> {
  return api.getBlob(attachmentContentPath(args));
}

/**
 * Downloads an attachment through the portal backend, which checks access and
 * streams the file under its own name. Resolves `false` on a 404 so the caller can
 * use the older flow. The first 8 MiB are asked for with a
 * `Range` header; the answer says how big the file is, and the rest is fetched
 * as 8 MiB ranges, {@link downloadParallelism} at a time.
 *
 * Where the browser can, the file is written straight to a file the person
 * picked, each range at its own offset as it arrives, so it is never held whole
 * in memory; otherwise the ranges are joined in order and saved as one Blob. A
 * range that breaks is fetched again on its own. A refusal (404, a removed
 * file) throws with the backend's message and stops the other ranges. A closed
 * save dialog resolves `true` with nothing downloaded and never calls `onSaved`,
 * which runs once the download ends, saying whether it was written to the picked
 * file (`saved`) or handed to the browser (`started`).
 */
async function downloadFromBackend(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string; name?: string; size?: number; onSaved?: (outcome: DownloadSaveOutcome) => void; onProgress?: DownloadProgress },
  save: (blob: Blob, name: string) => void,
): Promise<boolean> {
  const target = await openDiskTarget(args.name || "attachment", args.size);
  if (target.kind === "cancelled") return true;
  const sink = target.kind === "sink" ? target.sink : undefined;
  try {
    const done = await readAndStore(api, args, save, sink);
    if (done) args.onSaved?.(sink ? "saved" : "started");
    return done;
  } catch (err) {
    await sink?.abort();
    throw err;
  }
}

/**
 * Reads the rest of a file whose size is not known, because Content-Range is
 * hidden and the caller gave none: ranges after the first are asked for
 * `parallelism` at a time, in order, until one comes back short or is refused as
 * past the end (416). The most that is wasted is the ranges asked for beyond the
 * end, each a cheap refusal. A range's body is checked against its own
 * Content-Length, so a connection that dropped is read again instead of being
 * taken for the end of the file.
 */
async function readUntilEnd(
  api: BackendApi,
  path: string,
  store: (index: number, start: number, parts: Uint8Array[]) => Promise<void>,
): Promise<void> {
  const window = downloadParallelism();
  for (let next = 1; ; next += window) {
    const results = await Promise.all(
      Array.from({ length: window }, async (_, offset) => {
        const index = next + offset;
        const start = index * DOWNLOAD_RANGE_BYTES;
        try {
          const { parts } = await readRange(api, path, start, start + DOWNLOAD_RANGE_BYTES - 1, ({ response, parts }) => {
            const length = partsLength(parts);
            const declared = response.headers.get("Content-Length");
            return response.status === 206 && length > 0 && length <= DOWNLOAD_RANGE_BYTES && (declared === null || Number(declared) === length);
          });
          return { index, start, parts };
        } catch (err) {
          if (err instanceof BackendApiError && err.status === 416) return { index, start, parts: null };
          throw err;
        }
      }),
    );
    for (const { index, start, parts } of results) {
      if (parts === null) return; // past the end of the file
      await store(index, start, parts);
      if (partsLength(parts) < DOWNLOAD_RANGE_BYTES) return; // the last, short range
    }
  }
}

async function readAndStore(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string; name?: string; size?: number; onProgress?: DownloadProgress },
  save: (blob: Blob, name: string) => void,
  sink: DiskSink | undefined,
): Promise<boolean> {
  const path = attachmentContentPath(args);
  const knownSize = knownSizeOf(args.size);
  let first: RangeRead;
  try {
    first = await readRange(api, path, 0, DOWNLOAD_RANGE_BYTES - 1, ({ response, parts }) => {
      if (response.status !== 206) return true; // the whole file, the range was ignored
      const range = parseContentRange(response.headers);
      // Content-Range hidden from page scripts: fall back on the attachment's own size.
      if (range === null) return firstRangeIsWhole(response, partsLength(parts), knownSize);
      return (
        range.start === 0 &&
        range.end === Math.min(DOWNLOAD_RANGE_BYTES, range.total) - 1 &&
        partsLength(parts) === range.end + 1
      );
    });
  } catch (err) {
    if (err instanceof BackendApiError && err.status === 404) {
      await sink?.abort();
      return false;
    }
    throw err;
  }
  // The name the backend suggests, else the attachment's own: Content-Disposition may be hidden.
  const fileName = fileNameFromDisposition(first.response.headers) || args.name || "attachment";
  let total: number | undefined;
  if (first.response.status === 206) {
    total =
      parseContentRange(first.response.headers)?.total ??
      knownSize ??
      (partsLength(first.parts) < DOWNLOAD_RANGE_BYTES ? partsLength(first.parts) : undefined);
  }
  const ranges: Uint8Array[][] = [];
  if (sink) await sink.write(0, first.parts);
  else ranges[0] = first.parts;

  let received = partsLength(first.parts);
  if (total !== undefined) args.onProgress?.(received, total);
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
          const { parts } = await readRange(api, path, start, end, ({ response, parts }) => {
            if (response.status !== 206 || partsLength(parts) !== end - start + 1) return false;
            const range = parseContentRange(response.headers);
            // With Content-Range hidden the length and the 206 are all there is to check.
            return range === null || (range.start === start && range.end === end && range.total === total);
          });
          if (sink) await sink.write(start, parts);
          else ranges[index] = parts;
          received += end - start + 1;
          args.onProgress?.(received, total);
        } catch (error) {
          failure ??= { error };
        }
      }
    };
    await Promise.all(Array.from({ length: Math.min(downloadParallelism(), count - 1) }, worker));
    if (failure) throw failure.error;
  } else if (total === undefined && first.response.status === 206 && partsLength(first.parts) === DOWNLOAD_RANGE_BYTES) {
    // A full first range and no way to learn the size: read on until the file ends.
    await readUntilEnd(api, path, async (index, start, parts) => {
      if (sink) await sink.write(start, parts);
      else ranges[index] = parts;
    });
  }
  if (sink) await sink.close();
  else save(new Blob(ranges.flat() as BlobPart[]), fileName);
  return true;
}

/**
 * Downloads an attachment under its own name through the portal backend, which
 * streams it. Resolves `false` on a 404.
 */
export async function downloadEntityAttachment(
  api: BackendApi,
  /** `name` and `size` (bytes) are the attachment's own, when the caller has them: the name suggests the file name in the save dialog, a small size skips it. */
  args: { type: EntityAttachmentType; id: string; name?: string; size?: number; onSaved?: (outcome: DownloadSaveOutcome) => void; onProgress?: DownloadProgress },
  save: (blob: Blob, name: string) => void = saveBlob,
): Promise<boolean> {
  return downloadFromBackend(api, args, save);
}

/** `POST /attachments/upload/{id}/confirm`. Safe to repeat. */
export async function confirmEntityAttachment(
  api: BackendApi,
  type: EntityAttachmentType,
  attachmentId: string,
  /** Number of chunks sent, when the portal backend holds the files. */
  chunks?: number,
): Promise<EntityConfirmedAttachment> {
  return api.post<{ type: EntityAttachmentType; chunks?: number }, EntityConfirmedAttachment>(
    `/attachments/upload/${encodeURIComponent(attachmentId)}/confirm`,
    chunks !== undefined ? { type, chunks } : { type },
  );
}

const CONFIRM_ATTEMPTS = 3;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * What an upload belongs to: `parentId` is the record it is attached to;
 * `projectId` (only for a `comment` upload) is a draft image added before the
 * record exists, to be linked when the record is created. Exactly one is given.
 */
export type UploadEntityAttachmentTarget =
  | { parentId: string; projectId?: undefined }
  | { projectId: string; parentId?: undefined };

export type UploadEntityAttachmentArgs = UploadEntityAttachmentTarget & {
  type: EntityAttachmentType;
  file: File;
  /** Display name; defaults to the file's own name. */
  name?: string;
  /** Overall progress, 0-100. */
  onProgress?: (percent: number) => void;
  /** Aborts a running chunked upload (a single multipart POST is not interrupted). */
  signal?: AbortSignal;
};

/** Shown when the user cancels an upload. */
export const UPLOAD_CANCELLED_MESSAGE = "The upload was cancelled.";

/** Shown when a chunk could not be stored and the retries ran out. */
export const UPLOAD_INTERRUPTED_MESSAGE = "The upload was interrupted. Please try again.";

/** The user (or the app) aborted an upload. */
export class EntityUploadAbortedError extends Error {
  constructor() {
    super(UPLOAD_CANCELLED_MESSAGE);
    this.name = "AbortError";
  }
}

/** Bytes in every upload chunk but the last. Must match the backend's chunk size. */
export const UPLOAD_CHUNK_BYTES = 8 * 1024 * 1024;

/** Chunks sent at the same time, and tries per chunk. */
const UPLOAD_CHUNK_CONCURRENCY = 3;
const UPLOAD_CHUNK_ATTEMPTS = 4;

/** Waits `ms`, or throws as soon as the upload is cancelled. */
function pause(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new EntityUploadAbortedError());
      return;
    }
    const onAbort = (): void => {
      clearTimeout(timer);
      reject(new EntityUploadAbortedError());
    };
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

/** True for a chunk failure worth trying again (not a refusal of the request). */
function chunkFailureIsTransient(err: unknown): boolean {
  if (err instanceof BackendApiError) {
    return err.status === 408 || err.status === 429 || err.status >= 500;
  }
  return true; // a network error
}

/** Sends one chunk, trying again with a short pause when it fails in passing. */
async function sendChunk(
  api: BackendApi,
  args: { type: EntityAttachmentType; attachmentId: string; index: number; body: Blob; signal?: AbortSignal },
): Promise<void> {
  const path = `/attachments/upload/${encodeURIComponent(args.attachmentId)}/chunks/${args.index}?type=${encodeURIComponent(args.type)}`;
  for (let attempt = 1; ; attempt += 1) {
    if (args.signal?.aborted) throw new EntityUploadAbortedError();
    try {
      await api.putBinary(path, args.body, { signal: args.signal });
      return;
    } catch (err) {
      if (args.signal?.aborted || (err instanceof Error && err.name === "AbortError")) {
        throw new EntityUploadAbortedError();
      }
      if (attempt >= UPLOAD_CHUNK_ATTEMPTS || !chunkFailureIsTransient(err)) {
        throw err instanceof BackendApiError ? err : new Error(UPLOAD_INTERRUPTED_MESSAGE);
      }
      await pause(500 * 2 ** (attempt - 1), args.signal);
    }
  }
}

/**
 * Sends `file` to the portal backend in {@link UPLOAD_CHUNK_BYTES} chunks, a few
 * at a time, each retried on a passing failure. Once one chunk fails for good the
 * others stop taking new chunks (one already in flight finishes) and that first
 * failure is thrown. Returns the number of chunks, which the confirm step needs.
 */
export async function sendFileInChunks(
  api: BackendApi,
  args: {
    type: EntityAttachmentType;
    attachmentId: string;
    file: Blob;
    signal?: AbortSignal;
    onProgress?: (bytesSent: number, bytesTotal: number) => void;
  },
): Promise<number> {
  const { type, attachmentId, file, signal, onProgress } = args;
  const count = Math.max(1, Math.ceil(file.size / UPLOAD_CHUNK_BYTES));
  let next = 0;
  let sent = 0;
  let failure: { error: unknown } | undefined;
  const worker = async (): Promise<void> => {
    while (!failure) {
      const index = next;
      next += 1;
      if (index >= count) return;
      const from = index * UPLOAD_CHUNK_BYTES;
      const body = file.slice(from, Math.min(from + UPLOAD_CHUNK_BYTES, file.size));
      try {
        await sendChunk(api, { type, attachmentId, index, body, signal });
      } catch (error) {
        failure ??= { error };
        return;
      }
      sent += body.size;
      onProgress?.(sent, file.size);
    }
  };
  await Promise.all(Array.from({ length: Math.min(UPLOAD_CHUNK_CONCURRENCY, count) }, worker));
  if (failure) throw failure.error;
  return count;
}

/**
 * The three steps of an upload: ask entity-service to record the attachment,
 * send the file to the portal backend in chunks, then confirm. The file's exact
 * size is sent up front; entity-service checks the stored file against it.
 *
 * A confirm answered 503 (storage busy) is retried a few times without sending
 * the file again; if it still fails an {@link EntityUploadConfirmError} is
 * thrown, and the caller may call {@link confirmEntityAttachment} later.
 */
export async function uploadEntityAttachment(
  api: BackendApi,
  args: UploadEntityAttachmentArgs,
  options: { confirmDelayMs?: number } = {},
): Promise<EntityConfirmedAttachment> {
  const name = args.name?.trim() || args.file.name;

  args.onProgress?.(0);
  if (!!args.parentId === !!args.projectId) {
    throw new Error("An upload needs exactly one of parentId and projectId.");
  }
  const start = await api.post<
    {
      type: EntityAttachmentType;
      parentId?: string;
      projectId?: string;
      name: string;
      contentType: string;
      sizeBytes: number;
    },
    EntityUploadStart
  >("/attachments/upload", {
    type: args.type,
    ...(args.parentId ? { parentId: args.parentId } : { projectId: args.projectId }),
    name,
    contentType: args.file.type || "application/octet-stream",
    sizeBytes: args.file.size,
  });
  if (start.maxBytes > 0 && args.file.size > start.maxBytes) {
    throw new Error(
      `"${name}" is too large. The maximum attachment size is ${formatSizeLimit(start.maxBytes)}.`,
    );
  }
  if (args.signal?.aborted) throw new EntityUploadAbortedError();

  // Chunks go to the portal backend (bytes sent map to 10-90 %).
  const chunks = await sendFileInChunks(api, {
    type: args.type,
    attachmentId: start.attachmentId,
    file: args.file,
    signal: args.signal,
    onProgress: (sent, total) =>
      args.onProgress?.(total > 0 ? 10 + Math.min(80, Math.floor((sent / total) * 80)) : 10),
  });
  args.onProgress?.(90);

  let lastError: unknown;
  for (let attempt = 0; attempt < CONFIRM_ATTEMPTS; attempt += 1) {
    try {
      const confirmed = await confirmEntityAttachment(api, args.type, start.attachmentId, chunks);
      args.onProgress?.(100);
      return confirmed;
    } catch (err) {
      lastError = err;
      // Only "storage unavailable" is worth another try; anything else (closed
      // case, rejected file, ...) will not change.
      if (!(err instanceof BackendApiError && err.status === 503)) break;
      if (attempt < CONFIRM_ATTEMPTS - 1) await sleep((options.confirmDelayMs ?? 1000) * (attempt + 1));
    }
  }
  if (lastError instanceof BackendApiError && lastError.status !== 503) throw lastError;
  throw new EntityUploadConfirmError(start.attachmentId, args.type, lastError);
}

/** `POST /attachments/delete` (only the uploader may; not once the case is closed). */
export async function deleteEntityAttachment(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string },
): Promise<void> {
  await api.post<{ type: EntityAttachmentType; id: string }, null>("/attachments/delete", args);
}

/** The streamed route a comment's inline image is read from. */
export function entityInlineImagePath(id: string): string {
  return `/attachments/inline/${encodeURIComponent(id)}/content`;
}

/** Fetches a comment's inline image as a blob (authenticated, streamed by the backend). */
export function fetchEntityInlineImage(api: BackendApi, id: string): Promise<Blob> {
  return api.getBlob(entityInlineImagePath(id));
}
