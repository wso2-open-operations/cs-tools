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
export const DEFAULT_ENTITY_ATTACHMENT_MAX_MB = 10;
export const DEFAULT_INLINE_IMAGE_MAX_MB = 1;

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
 * (`CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB`, default 10). Read at call time.
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

/** A byte limit as the number of MB shown to the user ("25", "2.5"). */
export function formatLimitMb(bytes: number): string {
  const mb = bytes / MIB;
  return Number.isInteger(mb) ? String(mb) : mb.toFixed(1);
}
