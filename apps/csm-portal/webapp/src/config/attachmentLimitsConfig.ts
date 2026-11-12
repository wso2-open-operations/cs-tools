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


export const BYTES_PER_MB = 1024 * 1024;

/** Largest attachment (10 MiB) used until the server's limit is loaded. */
export const DEFAULT_ENTITY_ATTACHMENT_MAX_MB = 10;

/** Most inline-image bytes in one comment or description (1 MiB) used until the server's limit is loaded. */
export const DEFAULT_INLINE_IMAGE_MAX_MB = 1;

/**
 * Ceiling for one stored attachment, used only to cap the ranges of a download
 * whose size is unknown. The real limits come from the server
 * (`useAttachmentLimits`); the defaults above apply until they are loaded.
 */
export const MAX_ENTITY_ATTACHMENT_MAX_MB = 1024;

/** Ranges of one download fetched at once when nothing is configured. */
export const DEFAULT_DOWNLOAD_PARALLELISM = 4;

/** The most ranges of one download that may be fetched at once. */
export const MAX_DOWNLOAD_PARALLELISM = 8;

/**
 * How many ranges of one attachment download are fetched at the same time, from
 * `window.config.CSM_PORTAL_DOWNLOAD_PARALLELISM` (a number or numeric string).
 * 1 fetches one range at a time. A whole number is kept between 1 and
 * {@link MAX_DOWNLOAD_PARALLELISM}; a missing or unparseable value gives
 * {@link DEFAULT_DOWNLOAD_PARALLELISM}. Read at call time.
 */
export function downloadParallelism(): number {
  const raw = window.config?.CSM_PORTAL_DOWNLOAD_PARALLELISM;
  const value =
    typeof raw === "number" ? raw : typeof raw === "string" && raw.trim() !== "" ? Number(raw) : NaN;
  if (!Number.isFinite(value)) return DEFAULT_DOWNLOAD_PARALLELISM;
  return Math.min(MAX_DOWNLOAD_PARALLELISM, Math.max(1, Math.floor(value)));
}

/**
 * Whether a download is saved straight to a file the person picks, where the
 * browser can (Chrome, Edge), from `window.config.CSM_PORTAL_DOWNLOAD_TO_DISK`
 * (a boolean or the string "true"/"false"). On unless it is `false`. Read at
 * call time.
 */
export function downloadToDiskEnabled(): boolean {
  const raw = window.config?.CSM_PORTAL_DOWNLOAD_TO_DISK;
  if (typeof raw === "string") return raw.trim().toLowerCase() !== "false";
  return raw !== false;
}

/** A size limit for a message: "10 MB", "2.5 MB". */
export function formatMegabytes(bytes: number): string {
  return `${Number((bytes / BYTES_PER_MB).toFixed(2))} MB`;
}

/**
 * A size limit for a message, readable for big values: "10 MB" below 1024 MB,
 * "10 GB" from there ("1.5 GB"). Use it for the attachment limit shown to a user.
 */
export function formatSizeLimit(bytes: number): string {
  if (bytes >= 1024 * BYTES_PER_MB) {
    return `${Number((bytes / (1024 * BYTES_PER_MB)).toFixed(2))} GB`;
  }
  return formatMegabytes(bytes);
}

/** How many attachments a create form (case, service request, security report) takes. */
export const CREATE_FORM_MAX_ATTACHMENTS = 1;

/** How many attachments one comment or reply takes. */
export const COMMENT_MAX_ATTACHMENTS = 1;

/**
 * The message shown when a form, comment or editor already holds the most
 * attachments it takes: "Only one file allowed." for a limit of one,
 * "Only 3 files allowed." above it.
 */
export function attachmentLimitMessage(limit: number): string {
  return limit === 1 ? "Only one file allowed." : `Only ${limit} files allowed.`;
}
