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

import { MAX_ATTACHMENT_SIZE_BYTES } from "@features/support/constants/supportConstants";
import { formatBytes } from "@features/project-details/utils/projectDetails";

export type PayloadSizeResult =
  | { ok: true }
  | { ok: false; reason: "attachment_too_large"; fileName: string }
  | { ok: false; reason: "attachments_dominant"; attachmentBytes: number }
  | { ok: false; reason: "body_too_large"; totalBytes: number };

type EncodedAttachment = { name: string; file: string };

/**
 * Raw (decoded) size in bytes of a base64 string without decoding it.
 *
 * @param {string} base64 - Base64 content (no data URI prefix).
 * @returns {number} Decoded size in bytes.
 */
export function base64DecodedBytes(base64: string): number {
  const padding = base64.endsWith("==") ? 2 : base64.endsWith("=") ? 1 : 0;
  return Math.max(0, Math.floor((base64.length * 3) / 4) - padding);
}

/**
 * Checks a serialized request body against a whole-request cap and, when
 * attachments are present, against the per-file raw size limit. When the body
 * is too large, reports whether attachments or the rest (description with
 * inline images) make up the larger part.
 *
 * @param {string} serializedBody - JSON string that will be sent.
 * @param {number} maxBodyBytes - Whole-request cap in bytes.
 * @param {EncodedAttachment[]} [attachments] - Base64 attachments in the body.
 * @returns {PayloadSizeResult} Ok, or a typed reason.
 */
export function checkPayloadSize(
  serializedBody: string,
  maxBodyBytes: number,
  attachments: EncodedAttachment[] = [],
): PayloadSizeResult {
  for (const attachment of attachments) {
    if (base64DecodedBytes(attachment.file) > MAX_ATTACHMENT_SIZE_BYTES) {
      return {
        ok: false,
        reason: "attachment_too_large",
        fileName: attachment.name,
      };
    }
  }

  const totalBytes = new TextEncoder().encode(serializedBody).length;
  if (totalBytes <= maxBodyBytes) return { ok: true };

  const attachmentBytes = attachments.reduce(
    (sum, attachment) => sum + attachment.file.length,
    0,
  );
  if (attachmentBytes > totalBytes - attachmentBytes) {
    return { ok: false, reason: "attachments_dominant", attachmentBytes };
  }
  return { ok: false, reason: "body_too_large", totalBytes };
}

/**
 * Builds the user-facing message for a failed size check.
 *
 * @param {Exclude<PayloadSizeResult, { ok: true }>} result - Failed result.
 * @param {string} subject - What the body is, e.g. "case description".
 * @param {number} maxBodyBytes - Cap that was applied.
 * @returns {string} Plain-language error message.
 */
export function payloadSizeErrorMessage(
  result: Exclude<PayloadSizeResult, { ok: true }>,
  subject: string,
  maxBodyBytes: number,
): string {
  const perFile = formatBytes(MAX_ATTACHMENT_SIZE_BYTES);
  switch (result.reason) {
    case "attachment_too_large":
      return `The file "${result.fileName}" exceeds the ${perFile} per-file limit. Please remove it or attach a smaller file and try again.`;
    case "attachments_dominant":
      return `The attachments are too large to submit together (about ${formatBytes(result.attachmentBytes)} after encoding). Each file can be up to ${perFile}. Please remove or reduce some files and try again.`;
    default:
      return `The ${subject} exceeds the ${formatBytes(maxBodyBytes)} limit. Please reduce the size or the number of inline images and try again.`;
  }
}
