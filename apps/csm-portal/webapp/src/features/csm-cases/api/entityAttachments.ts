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
import { formatSizeLimit } from "@config/attachmentLimitsConfig";
import { saveBlob } from "@utils/saveBlob";

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

/** Shown when a user has made too many attachment requests in a minute (HTTP 429). */
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

/** Resumes of a download that broke part-way, before giving up. */
const DOWNLOAD_RESUME_ATTEMPTS = 3;

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
 * streams the file under its own name. The bytes are read as they arrive, so a
 * break part-way resumes with a Range request instead of starting again. A
 * refusal (404, a removed file) throws with the backend's message.
 */
export async function downloadEntityAttachment(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string },
  save: (blob: Blob, name: string) => void = saveBlob,
): Promise<void> {
  const path = attachmentContentPath(args);
  const parts: Uint8Array[] = [];
  let received = 0;
  let total: number | undefined;
  let fileName: string | undefined;
  for (let attempt = 0; ; attempt += 1) {
    let response: Response;
    try {
      response = await api.getResponse(path, received > 0 ? { headers: { Range: `bytes=${received}-` } } : undefined);
    } catch (err) {
      if (err instanceof BackendApiError || attempt + 1 >= DOWNLOAD_RESUME_ATTEMPTS) throw err;
      continue; // a dropped connection before any answer: try again
    }
    if (received === 0) {
      fileName = fileNameFromDisposition(response.headers);
      const length = Number(response.headers.get("Content-Length"));
      total = Number.isFinite(length) && length > 0 ? length : undefined;
    } else if (response.status !== 206) {
      // The server ignored the range: start over rather than glue the wrong bytes.
      parts.length = 0;
      received = 0;
    }
    try {
      const reader = response.body?.getReader();
      if (!reader) {
        const bytes = new Uint8Array(await response.arrayBuffer());
        parts.push(bytes);
        received += bytes.length;
      } else {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          parts.push(value);
          received += value.length;
        }
      }
      if (total === undefined || received >= total) break;
      throw new Error("The download ended early.");
    } catch {
      if (attempt + 1 >= DOWNLOAD_RESUME_ATTEMPTS) {
        throw new Error("The download was interrupted. Please try again.");
      }
    }
  }
  save(new Blob(parts as BlobPart[]), fileName || "attachment");
}

/** `POST /attachments/upload/{id}/confirm`. Safe to repeat. */
export async function confirmEntityAttachment(
  api: BackendApi,
  type: EntityAttachmentType,
  attachmentId: string,
  /** Number of chunks sent. */
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
  /** Aborts a running chunked upload. */
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
export const UPLOAD_CHUNK_BYTES = 16 * 1024 * 1024;

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
