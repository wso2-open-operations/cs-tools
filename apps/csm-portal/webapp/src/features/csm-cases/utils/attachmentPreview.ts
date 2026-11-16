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

/** The content-type families {@link AttachmentPreviewDialog} knows how to render inline. */
export type AttachmentPreviewKind = "image" | "pdf";

/**
 * A previewable URL for an attachment: its bytes are fetched as an
 * authenticated `Blob` through the portal backend and turned into a `blob:`
 * object URL. `revoke: true` — the caller must `URL.revokeObjectURL(url)` once
 * it is no longer displayed. (`revoke: false` is for a URL that is not a
 * `blob:` URL and must not be revoked.)
 */
export interface AttachmentPreviewSource {
  url: string;
  revoke: boolean;
}

/**
 * Mirrors the entity-service's `safeAttachmentTypes` allowlist
 * (`case_handler.go`). `GET /attachments/{id}/content` only forwards the
 * stored `Content-Type` as-is for a type in this set — anything else
 * (including every `video/*` type, which has no entry at all) is coerced to
 * `application/octet-stream` by the backend as a deliberate stored-XSS
 * control. The attachment's `contentType` from `/attachments/search` list
 * metadata reflects whatever the uploader claimed, uncoerced, so the FE must
 * re-check it against this same allowlist before trusting it for anything
 * (relabeling a fetched blob, or offering an inline preview) — otherwise a
 * spoofed metadata `contentType` re-enables exactly what the backend control
 * is meant to block. Keep this list in lockstep with the backend's; do not
 * add an entry here that isn't also in `safeAttachmentTypes`.
 */
const SAFE_ATTACHMENT_CONTENT_TYPES: ReadonlySet<string> = new Set([
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
  "application/pdf",
  "text/plain",
  "application/zip",
  "application/x-zip-compressed",
  "application/msword",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.ms-excel",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
]);

/**
 * Normalizes a content type the same way the backend does before its
 * allowlist check: strip any `;`-parameters, trim, lowercase. Order matches
 * `strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))`
 * in `case_handler.go`.
 */
function normalizeContentType(contentType: string): string {
  return contentType.split(";")[0].trim().toLowerCase();
}

/**
 * True when `contentType` is a member of the backend's safe-content-type
 * allowlist (after the same normalization the backend applies). Anything
 * outside this set must be treated as untrusted for rendering/relabeling
 * purposes, even though it is a perfectly valid attachment to store/download.
 */
export function isSafeAttachmentContentType(contentType: string): boolean {
  return SAFE_ATTACHMENT_CONTENT_TYPES.has(normalizeContentType(contentType));
}

/**
 * Classify an attachment's content type into a previewable kind, or `null`
 * when it has no inline preview (docs, archives, video, etc. stay
 * download-only).
 *
 * Gated on {@link isSafeAttachmentContentType}: a type that the backend's
 * `GET /attachments/{id}/content` would coerce to
 * `application/octet-stream` can never be offered a preview, regardless of
 * what the (uploader-controlled) metadata claims. This is why `video/*` is
 * not previewable today — it has no entry in the backend's allowlist at
 * all, so a fetched blob can never be trusted as that video type. Of the
 * allowlisted types, only images and PDFs make sense as *inline* preview;
 * `text/plain` and the archive/office types stay download-only.
 */
export function getAttachmentPreviewKind(
  contentType: string,
): AttachmentPreviewKind | null {
  const type = normalizeContentType(contentType);
  if (!SAFE_ATTACHMENT_CONTENT_TYPES.has(type)) return null;
  if (
    type === "image/png" ||
    type === "image/jpeg" ||
    type === "image/gif" ||
    type === "image/webp"
  ) {
    return "image";
  }
  if (type === "application/pdf") return "pdf";
  return null;
}
