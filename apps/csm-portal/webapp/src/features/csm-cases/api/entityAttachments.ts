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
import { Upload } from "tus-js-client";
import { formatSizeLimit } from "@config/attachmentLimitsConfig";
import { saveBlob } from "@utils/saveBlob";

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
  /**
   * The file is too big to read into memory: hand `url` to the browser's own
   * downloader instead of fetching it.
   */
  largeFile?: boolean;
}

interface EntityAttachmentLinksResponse {
  links: EntityAttachmentLink[];
}

export interface EntityUploadStart {
  attachmentId: string;
  number: string;
  uploadUrl: string;
  fieldName: string;
  expiresAt: string;
  maxBytes: number;
  /** "multipart" (one POST, the default when absent) or "tus" (resumable chunks). */
  uploadMethod?: "multipart" | "tus";
  /** TUS only. `shareId` is a secret: never log it or put it in a message. */
  shareId?: string;
  path?: string;
  chunkSize?: number;
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

/**
 * Hands a URL to the browser's own downloader (a plain link click, no new tab),
 * so a big file streams to disk, can resume, and never sits in memory. The saved
 * name is the one SFTPGo stores the file under.
 */
export function startBrowserDownload(url: string): void {
  const a = document.createElement("a");
  a.href = url;
  a.rel = "noopener noreferrer";
  a.style.display = "none";
  document.body.appendChild(a);
  a.click();
  a.remove();
}

/**
 * Downloads an attachment under its own name (SFTPGo names the file after its
 * stored path, which carries the attachment number). Resolves `false` when
 * entity-service does not hold the file, so the caller can use the older flow.
 *
 * A link flagged `largeFile` is not fetched: the URL goes to the browser's
 * downloader (`navigate`), which streams it to disk under SFTPGo's stored name.
 */
export async function downloadEntityAttachment(
  api: BackendApi,
  args: { type: EntityAttachmentType; id: string },
  fetchImpl?: FetchLike,
  save: (blob: Blob, name: string) => void = saveBlob,
  navigate: (url: string) => void = startBrowserDownload,
): Promise<boolean> {
  const first = await requestDownloadLink(api, args.type, args.id, "download");
  if (first?.available && first.url && first.largeFile) {
    navigate(first.url);
    return true;
  }
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
): Promise<EntityConfirmedAttachment> {
  return api.post<{ type: EntityAttachmentType }, EntityConfirmedAttachment>(
    `/attachments/upload/${encodeURIComponent(attachmentId)}/confirm`,
    { type },
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
  /** Aborts a running upload (a TUS upload stops; a multipart POST is not interrupted). */
  signal?: AbortSignal;
}

/** Shown when SFTPGo refused to create a TUS upload (the link was used or has run out). */
export const TUS_CREATE_FAILED_MESSAGE =
  "The upload link has been used or has expired. Please start the upload again.";

/** Shown when a TUS upload broke off after it had started and retries ran out. */
export const TUS_INTERRUPTED_MESSAGE = "The upload was interrupted. Please try again.";

/** Shown when the user cancels an upload. */
export const UPLOAD_CANCELLED_MESSAGE = "The upload was cancelled.";

/** A failed TUS upload. The message never holds the share id or the upload URL. */
export class EntityTusUploadError extends Error {
  /** HTTP status of the refused request, when there was a response. */
  status?: number;
  /** True when the upload could not even be created (no chunk was sent). */
  duringCreate: boolean;
  constructor(duringCreate: boolean, status?: number) {
    super(duringCreate ? TUS_CREATE_FAILED_MESSAGE : TUS_INTERRUPTED_MESSAGE);
    this.name = "EntityTusUploadError";
    this.duringCreate = duringCreate;
    this.status = status;
  }
}

/** The user (or the app) aborted an upload. */
export class EntityUploadAbortedError extends Error {
  constructor() {
    super(UPLOAD_CANCELLED_MESSAGE);
    this.name = "AbortError";
  }
}

export const TUS_RETRY_DELAYS_MS = [0, 1000, 3000, 5000, 10000];

function tusStatus(err: unknown): number | undefined {
  const response = (err as { originalResponse?: { getStatus?: () => number } } | null)?.originalResponse;
  const status = response?.getStatus?.();
  return typeof status === "number" && status > 0 ? status : undefined;
}

/**
 * Sends `file` to SFTPGo's TUS endpoint in chunks, with no Authorization header
 * (the share id in the metadata is the credential). Resolves when the last chunk
 * is stored; the caller then confirms.
 *
 * Creating the upload is what uses the one-use share, so a failure before the
 * upload exists is never retried (a second create always fails) and reported as
 * {@link TUS_CREATE_FAILED_MESSAGE}. Once it exists, network drops and 5xx
 * answers are retried; when those run out the error is
 * {@link TUS_INTERRUPTED_MESSAGE}.
 */
export function uploadFileViaTus(args: {
  file: Blob;
  endpoint: string;
  shareId: string;
  path: string;
  chunkSize: number;
  onProgress?: (bytesSent: number, bytesTotal: number) => void;
  signal?: AbortSignal;
}): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    if (args.signal?.aborted) {
      reject(new EntityUploadAbortedError());
      return;
    }
    let created = false;
    let settled = false;
    const finish = (fn: () => void): void => {
      if (settled) return;
      settled = true;
      args.signal?.removeEventListener("abort", onAbort);
      fn();
    };
    const upload = new Upload(args.file, {
      endpoint: args.endpoint,
      metadata: { path: args.path, share_id: args.shareId },
      chunkSize: args.chunkSize,
      retryDelays: TUS_RETRY_DELAYS_MS,
      storeFingerprintForResuming: false,
      removeFingerprintOnSuccess: true,
      onUploadUrlAvailable: () => {
        created = true;
      },
      onShouldRetry: (err) => {
        const status = tusStatus(err);
        if (!created) return status === undefined; // only a dropped connection may retry a create
        if (status === undefined) return true;
        return status >= 500 || status === 408 || status === 409 || status === 423 || status === 429;
      },
      onProgress: (sent, total) => args.onProgress?.(sent, total),
      onError: (err) => finish(() => reject(new EntityTusUploadError(!created, tusStatus(err)))),
      onSuccess: () => finish(resolve),
    });
    const onAbort = (): void => {
      void upload.abort();
      finish(() => reject(new EntityUploadAbortedError()));
    };
    args.signal?.addEventListener("abort", onAbort, { once: true });
    upload.start();
  });
}

/**
 * The three steps of an upload: ask entity-service for a one-use upload link,
 * post the file to SFTPGo with it, then confirm. The file's exact size is sent
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

  if (start.uploadMethod === "tus") {
    if (!start.shareId || !start.chunkSize || start.chunkSize <= 0) {
      throw new Error("The upload could not be started. Please try again.");
    }
    // Straight to SFTPGo in resumable chunks; bytes sent map to 0-90 %.
    await uploadFileViaTus({
      file: args.file,
      endpoint: start.uploadUrl,
      shareId: start.shareId,
      path: start.path || "/upload.bin",
      chunkSize: start.chunkSize,
      signal: args.signal,
      onProgress: (sent, total) =>
        args.onProgress?.(total > 0 ? Math.min(90, Math.floor((sent / total) * 90)) : 0),
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
      const confirmed = await confirmEntityAttachment(api, args.type, start.attachmentId);
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
