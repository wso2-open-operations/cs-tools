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

import { ApiError, parseApiResponseMessage } from "@utils/ApiError";

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

/** Response of POST /attachments/upload. */
export interface StoredAttachmentUploadStart {
  attachmentId: string;
  number?: string;
  uploadUrl: string;
  fieldName: string;
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

/**
 * Largest file the portal offers to upload. Entity-service enforces its own
 * (authoritative) limit and `maxBytes` of the upload response is checked too;
 * this only avoids starting an upload that is certain to be refused.
 */
export const STORED_ATTACHMENT_MAX_BYTES = 100 * 1024 * 1024;

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
  input: { type: StoredAttachmentType; id: string; name?: string },
): Promise<void> {
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
  start: Pick<StoredAttachmentUploadStart, "uploadUrl" | "fieldName">,
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
  input: { type: StoredAttachmentType; attachmentId: string },
): Promise<StoredAttachmentConfirmed> {
  const response = await postJson(
    authFetch,
    `/attachments/upload/${encodeURIComponent(input.attachmentId)}/confirm`,
    { type: input.type },
  );
  return (await response.json()) as StoredAttachmentConfirmed;
}

export type StoredUploadStep = "preparing" | "uploading" | "finishing";

/**
 * Uploads one file in three steps (start, send to SFTPGo, confirm), reporting
 * each step. If only the confirm step fails, an UploadConfirmPendingError is
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
  },
): Promise<StoredAttachmentConfirmed> {
  const { type, parentId, file, name, onStep } = input;
  if (file.size > STORED_ATTACHMENT_MAX_BYTES) {
    throw new Error("File size exceeds 100 MB limit.");
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
      `File size exceeds the ${Math.floor(start.maxBytes / (1024 * 1024))} MB limit.`,
    );
  }
  onStep?.("uploading");
  await sendFileToUploadLink(start, file);
  onStep?.("finishing");
  try {
    return await confirmStoredUpload(authFetch, {
      type,
      attachmentId: start.attachmentId,
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

/** Outcome of loading a comment's inline image through entity-service. */
export type InlineImageResult =
  | { status: "ok"; blob: Blob }
  | { status: "not-found" }
  | { status: "error" };

/**
 * Fetches a comment's inline image through entity-service (which checks access and
 * streams it). "not-found" (a 404) lets a caller fall back to the older flow;
 * any other failure is "error".
 */
export async function fetchInlineImageBlob(
  authFetch: AuthFetch,
  id: string,
): Promise<InlineImageResult> {
  const response = await authFetch(
    `${getBackendBaseUrl()}/attachments/inline/${encodeURIComponent(id)}/content`,
    { method: "GET" },
  );
  if (response.status === 404) return { status: "not-found" };
  if (!response.ok) return { status: "error" };
  return { status: "ok", blob: await response.blob() };
}
