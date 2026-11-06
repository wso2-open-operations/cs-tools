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


const MIB = 1024 * 1024;

/** Entity-service's own defaults (`ATTACHMENT_MAX_BYTES`, `INLINE_IMAGE_MAX_BYTES`). */
export const DEFAULT_ENTITY_ATTACHMENT_MAX_MB = 100;
export const DEFAULT_INLINE_IMAGE_MAX_MB = 10;

/**
 * A size in MB from `window.config`. Platform config injection often turns
 * numbers into strings, so both are accepted; anything that is not a positive
 * finite number falls back to the default.
 */
function configuredMb(value: unknown, fallback: number): number {
  const parsed =
    typeof value === "number"
      ? value
      : typeof value === "string" && value.trim() !== ""
        ? Number(value)
        : NaN;
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

/**
 * Largest work item or deployment file the portal offers to upload, in bytes
 * (`CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB`, default 100). Read at call time.
 * Entity-service enforces its own limit; this only refuses early.
 */
export function entityAttachmentMaxBytes(): number {
  return Math.floor(
    configuredMb(
      window.config?.CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB,
      DEFAULT_ENTITY_ATTACHMENT_MAX_MB,
    ) * MIB,
  );
}

/**
 * Largest pasted or inserted comment image, in bytes
 * (`CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB`, default 10). Read at call time.
 */
export function inlineImageMaxBytes(): number {
  return Math.floor(
    configuredMb(
      window.config?.CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB,
      DEFAULT_INLINE_IMAGE_MAX_MB,
    ) * MIB,
  );
}

/** Ranges of one download fetched at once when nothing is configured. */
export const DEFAULT_DOWNLOAD_PARALLELISM = 4;

/** The most ranges of one download that may be fetched at once. */
export const MAX_DOWNLOAD_PARALLELISM = 8;

/**
 * How many 8 MiB ranges of one attachment download are fetched at the same
 * time (`CUSTOMER_PORTAL_DOWNLOAD_PARALLELISM`, default 4; 1 fetches one at a
 * time). A number or numeric string; a whole number is kept between 1 and 8 and
 * anything unparseable gives the default. Read at call time.
 */
export function downloadParallelism(): number {
  const raw: unknown = window.config?.CUSTOMER_PORTAL_DOWNLOAD_PARALLELISM;
  const parsed =
    typeof raw === "number"
      ? raw
      : typeof raw === "string" && raw.trim() !== ""
        ? Number(raw)
        : NaN;
  if (!Number.isFinite(parsed)) return DEFAULT_DOWNLOAD_PARALLELISM;
  return Math.min(MAX_DOWNLOAD_PARALLELISM, Math.max(1, Math.floor(parsed)));
}

/** A byte limit as the number of MB shown to the user ("25", "2.5"). */
export function formatLimitMb(bytes: number): string {
  const mb = bytes / MIB;
  return Number.isInteger(mb) ? String(mb) : mb.toFixed(1);
}

/**
 * A byte limit as text for messages: whole or one-decimal GB from 1024 MB up
 * ("10 GB", "1.5 GB"), otherwise MB ("10 MB").
 */
export function formatLimit(bytes: number): string {
  if (bytes >= 1024 * MIB) {
    const gb = bytes / (1024 * MIB);
    return `${Number.isInteger(gb) ? String(gb) : gb.toFixed(1)} GB`;
  }
  return `${formatLimitMb(bytes)} MB`;
}

/**
 * Whether a download is saved straight to a file the person picks, where the
 * browser can (Chrome, Edge), from `CUSTOMER_PORTAL_DOWNLOAD_TO_DISK` (a boolean
 * or the string "true"/"false"). On unless it is `false`. Read at call time.
 */
export function downloadToDiskEnabled(): boolean {
  const raw = window.config?.CUSTOMER_PORTAL_DOWNLOAD_TO_DISK;
  if (typeof raw === "string") return raw.trim().toLowerCase() !== "false";
  return raw !== false;
}
