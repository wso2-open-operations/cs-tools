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


import { isEntityAttachmentsEnabled } from "@config/entityAttachmentsConfig";

const BYTES_PER_MB = 1024 * 1024;

/** Largest file of the older (ServiceNow) attachment flow, which the backend caps at 10 MB. */
export const LEGACY_ATTACHMENT_MAX_BYTES = 10 * BYTES_PER_MB;

/** Largest stored attachment (entity-service `ATTACHMENT_MAX_BYTES` default, 10 MiB). */
export const DEFAULT_ENTITY_ATTACHMENT_MAX_MB = 10;

/** Largest comment image (entity-service `INLINE_IMAGE_MAX_BYTES` default, 1 MiB). */
export const DEFAULT_INLINE_IMAGE_MAX_MB = 1;

/** A positive number of megabytes from a number or numeric string, else `fallback`. */
function megabytes(raw: unknown, fallback: number): number {
  const value =
    typeof raw === "number" ? raw : typeof raw === "string" && raw.trim() !== "" ? Number(raw) : NaN;
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

/**
 * Largest file, in bytes, the portal offers to upload as a work item attachment
 * through entity-service's stored-attachment flow.
 *
 * Read from the runtime `window.config.CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB`
 * key (a number of megabytes, or a numeric string). A missing, unparseable, zero
 * or negative value gives {@link DEFAULT_ENTITY_ATTACHMENT_MAX_MB}. Keep it equal
 * to the entity-service's `ATTACHMENT_MAX_BYTES`: the server stays the authority,
 * this only refuses an oversized file before the upload starts.
 *
 * Read at call time (not module load) so a test can set `window.config` per case.
 */
export function entityAttachmentMaxBytes(): number {
  return (
    megabytes(window.config?.CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB, DEFAULT_ENTITY_ATTACHMENT_MAX_MB) *
    BYTES_PER_MB
  );
}

/**
 * The size limit a case attachment is checked against before it is uploaded:
 * {@link entityAttachmentMaxBytes} through entity-service's stored flow, the
 * older {@link LEGACY_ATTACHMENT_MAX_BYTES} when that flow is off.
 */
export function getMaxAttachmentSizeBytes(): number {
  return isEntityAttachmentsEnabled() ? entityAttachmentMaxBytes() : LEGACY_ATTACHMENT_MAX_BYTES;
}

/**
 * Largest image, in bytes, that can be pasted or inserted into a comment, from
 * `window.config.CSM_PORTAL_INLINE_IMAGE_MAX_MB` (same rules as above, default
 * {@link DEFAULT_INLINE_IMAGE_MAX_MB}; keep it equal to the entity-service's
 * `INLINE_IMAGE_MAX_BYTES`). Read at call time.
 */
export function inlineImageMaxBytes(): number {
  return (
    megabytes(window.config?.CSM_PORTAL_INLINE_IMAGE_MAX_MB, DEFAULT_INLINE_IMAGE_MAX_MB) *
    BYTES_PER_MB
  );
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
