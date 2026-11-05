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
import { isBackendAttachmentStorage } from "@config/entityAttachmentsConfig";
import { downloadParallelism, formatSizeLimit } from "@config/attachmentLimitsConfig";
import { saveBlob } from "@utils/saveBlob";
import { openDiskTarget, type DiskSink } from "@features/csm-cases/api/downloadToDisk";

/**
 * Entity-service stored attachments, reached through the CSM portal backend's
 * pass-through routes. Plain functions over a {@link BackendApi} (and an
 * injectable `fetch` for the calls that go straight to SFTPGo), so they are
 * unit-testable without React; the hooks in `useCsmCaseAttachments.ts` bind
 * them to the signed-in session.
 *
 * Why a share link is never kept: it serves one request and lasts about a
 * minute, and whoever holds the URL can open the file. So a link is requested
 * only when the user acts, used straight away, and dropped — never cached,
 * prefetched or put in React state.
 */

/** Which attachment table a request is about (the request body's `type`). */
export type EntityAttachmentType = "work_item" | "comment" | "deployment";

/**
 * Cases, incidents and change requests are all work items in entity-service, so
 * every attachment on them uses this \`type\`.
 */
export const WORK_ITEM_ATTACHMENT_TYPE: EntityAttachmentType = "work_item";

/** What a download link is for: a click on Download, or an inline preview. */
export type EntityLinkPurpose = "download" | "preview";

export interface EntityAttachmentLink {
  id: string;
  number: string;
  name: string;
  contentType?: string;
  sizeBytes?: number;
  available: boolean;
  /** DELETED, FAILED, NOT_STORED or STORAGE_UNAVAILABLE when not available. */
  reason?: string;
  url?: string;
  expiresAt?: string;
}

interface EntityAttachmentLinksResponse {
  links: EntityAttachmentLink[];
}

/**
 * Response of `POST /attachments/upload`. With the files held by the portal
 * backend (see `isBackendAttachmentStorage`) there is no `uploadUrl`: the file
 * is sent to the backend in chunks instead.
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

/** A link came back but the file cannot be fetched (`reason` says why). */
export class EntityAttachmentUnavailableError extends Error {
  reason: string;
  constructor(reason: string) {
    super(unavailableMessage(reason));
    this.name = "EntityAttachmentUnavailableError";
    this.reason = reason;
  }
}

function unavailableMessage(reason: string): string {
  switch (reason) {
    case "DELETED":
      return "This attachment was deleted.";
    case "FAILED":
      return "This attachment failed to upload and has no file.";
    case "STORAGE_UNAVAILABLE":
      return "File storage is unavailable right now. Please try again.";
    default:
      return "This attachment's file is not available.";
  }
}

/**
 * The upload reached SFTPGo but the final confirm step did not succeed. The
 * file is still staged, so `confirmEntityAttachment` can be called again with
 * these ids without uploading it a second time.
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

/** `POST /attachments/download` for one attachment. `undefined` = no such row. */
export async function requestDownloadLink(
  api: BackendApi,
  type: EntityAttachmentType,
  id: string,
  purpose: EntityLinkPurpose,
): Promise<EntityAttachmentLink | undefined> {
  const response = await api.post<
    { type: EntityAttachmentType; ids: string[]; purpose: EntityLinkPurpose },
    EntityAttachmentLinksResponse
  >("/attachments/download", { type, ids: [id], purpose });
  return response?.links?.find((l) => l.id.toLowerCase() === id.toLowerCase());
}

export type FetchLike = (url: string, init?: RequestInit) => Promise<Response>;

/**
 * Asks for a link and fetches its bytes straight away. If SFTPGo refuses the
 * link (it may have gone, or been used), asks for a fresh one and tries once
 * more. Returns `undefined` when entity-service has no such attachment or its
 * file is not stored there (`NOT_STORED`), so a caller can fall back to the
 * older flow; any other unavailable reason throws.
 *
 * `firstLink` is a link the caller already requested (and found usable); it is
 * used for the first attempt instead of asking again.
 */
export async function fetchEntityAttachmentBlob(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string; purpose: EntityLinkPurpose },
  fetchImpl: FetchLike = (url, init) => fetch(url, init),
  firstLink?: EntityAttachmentLink,
): Promise<{ blob: Blob; link: EntityAttachmentLink } | undefined> {
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const link =
      attempt === 0 && firstLink ? firstLink : await requestDownloadLink(api, args.type, args.id, args.purpose);
    if (!link) return undefined;
    if (!link.available || !link.url) {
      if (link.reason === "NOT_STORED") return undefined;
      throw new EntityAttachmentUnavailableError(link.reason ?? "");
    }
    // Plain cross-origin request to SFTPGo: no cookies, no app credentials.
    const response = await fetchImpl(link.url, { method: "GET", credentials: "omit" });
    if (response.ok) return { blob: await response.blob(), link };
    if (attempt === 1) {
      throw new Error(`Could not download ${link.name} (HTTP ${response.status}).`);
    }
  }
  return undefined;
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
 * which runs once the whole file is saved.
 */
async function downloadFromBackend(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string; name?: string; size?: number; onSaved?: () => void },
  save: (blob: Blob, name: string) => void,
): Promise<boolean> {
  const target = await openDiskTarget(args.name || "attachment", args.size);
  if (target.kind === "cancelled") return true;
  const sink = target.kind === "sink" ? target.sink : undefined;
  try {
    const done = await readAndStore(api, args, save, sink);
    if (done) args.onSaved?.();
    return done;
  } catch (err) {
    await sink?.abort();
    throw err;
  }
}

async function readAndStore(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string },
  save: (blob: Blob, name: string) => void,
  sink: DiskSink | undefined,
): Promise<boolean> {
  const path = attachmentContentPath(args);
  let first: RangeRead;
  try {
    first = await readRange(api, path, 0, DOWNLOAD_RANGE_BYTES - 1, ({ response, parts }) => {
      if (response.status !== 206) return true; // the whole file, the range was ignored
      const range = parseContentRange(response.headers);
      return (
        range !== null &&
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
  const fileName = fileNameFromDisposition(first.response.headers) || "attachment";
  const total = first.response.status === 206 ? parseContentRange(first.response.headers)?.total : undefined;
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
          const { parts } = await readRange(api, path, start, end, ({ response, parts }) => {
            const range = parseContentRange(response.headers);
            return (
              response.status === 206 &&
              range !== null &&
              range.start === start &&
              range.end === end &&
              range.total === total &&
              partsLength(parts) === end - start + 1
            );
          });
          if (sink) await sink.write(start, parts);
          else ranges[index] = parts;
        } catch (error) {
          failure ??= { error };
        }
      }
    };
    await Promise.all(Array.from({ length: Math.min(downloadParallelism(), count - 1) }, worker));
    if (failure) throw failure.error;
  }
  if (sink) await sink.close();
  else save(new Blob(ranges.flat() as BlobPart[]), fileName);
  return true;
}

/**
 * Downloads an attachment under its own name (SFTPGo names the file after its
 * stored path, which carries the attachment number). Resolves `false` when
 * entity-service does not hold the file, so the caller can use the older flow.
 * With the backend holding the files it streams through the backend instead of
 * using a link.
 */
export async function downloadEntityAttachment(
  api: BackendApi,
  /** `name` and `size` (bytes) are the attachment's own, when the caller has them: the name suggests the file name in the save dialog, a small size skips it. */
  args: { type: EntityAttachmentType; id: string; name?: string; size?: number; onSaved?: () => void },
  fetchImpl?: FetchLike,
  save: (blob: Blob, name: string) => void = saveBlob,
): Promise<boolean> {
  if (isBackendAttachmentStorage()) return downloadFromBackend(api, args, save);
  const first = await requestDownloadLink(api, args.type, args.id, "download");
  const got = await fetchEntityAttachmentBlob(api, { ...args, purpose: "download" }, fetchImpl, first);
  if (!got) return false;
  save(got.blob, got.link.name);
  return true;
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

export interface UploadEntityAttachmentArgs {
  type: EntityAttachmentType;
  parentId: string;
  file: File;
  /** Display name; defaults to the file's own name. */
  name?: string;
  /** Overall progress, 0-100. */
  onProgress?: (percent: number) => void;
  /** Aborts a running chunked upload (a single multipart POST is not interrupted). */
  signal?: AbortSignal;
}

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
 * send the file (to SFTPGo with a one-use link, or to the portal backend in
 * chunks), then confirm. The file's exact size is sent
 * up front; entity-service checks the stored file against it.
 *
 * A confirm answered 503 (storage busy) is retried a few times without sending
 * the file again; if it still fails an {@link EntityUploadConfirmError} is
 * thrown, and the caller may call {@link confirmEntityAttachment} later.
 */
export async function uploadEntityAttachment(
  api: BackendApi,
  args: UploadEntityAttachmentArgs,
  options: { fetchImpl?: FetchLike; confirmDelayMs?: number } = {},
): Promise<EntityConfirmedAttachment> {
  const fetchImpl: FetchLike = options.fetchImpl ?? ((url, init) => fetch(url, init));
  const name = args.name?.trim() || args.file.name;

  args.onProgress?.(0);
  const start = await api.post<
    {
      type: EntityAttachmentType;
      parentId: string;
      name: string;
      contentType: string;
      sizeBytes: number;
    },
    EntityUploadStart
  >("/attachments/upload", {
    type: args.type,
    parentId: args.parentId,
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

  let chunks: number | undefined;
  if (!start.uploadUrl) {
    // The portal backend holds the files: chunks go to it (bytes sent map to 10-90 %).
    chunks = await sendFileInChunks(api, {
      type: args.type,
      attachmentId: start.attachmentId,
      file: args.file,
      signal: args.signal,
      onProgress: (sent, total) =>
        args.onProgress?.(total > 0 ? 10 + Math.min(80, Math.floor((sent / total) * 80)) : 10),
    });
    args.onProgress?.(90);
  } else {
    // Straight to SFTPGo: one file in the form field it names, no app credentials.
    const form = new FormData();
    form.append(start.fieldName || "filenames", args.file, name);
    args.onProgress?.(10);
    const uploaded = await fetchImpl(start.uploadUrl, {
      method: "POST",
      body: form,
      credentials: "omit",
    });
    if (!uploaded.ok) {
      throw new Error(`Could not upload ${name} (HTTP ${uploaded.status}). Please try again.`);
    }
    args.onProgress?.(90);
  }

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
