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

import { formatBytes } from "@features/project-details/utils/projectDetails";

export type PayloadSizeResult =
  | { ok: true }
  | { ok: false; totalBytes: number };

/**
 * Checks a serialized request body against a whole-request cap.
 *
 * @param {string} serializedBody - JSON string that will be sent.
 * @param {number} maxBodyBytes - Whole-request cap in bytes.
 * @returns {PayloadSizeResult} Ok, or the size that exceeded the cap.
 */
export function checkPayloadSize(
  serializedBody: string,
  maxBodyBytes: number,
): PayloadSizeResult {
  const totalBytes = new TextEncoder().encode(serializedBody).length;
  if (totalBytes <= maxBodyBytes) return { ok: true };
  return { ok: false, totalBytes };
}

/**
 * Builds the user-facing message for a failed size check.
 *
 * @param {Exclude<PayloadSizeResult, { ok: true }>} _result - Failed result.
 * @param {string} subject - What the body is, e.g. "case description".
 * @param {number} maxBodyBytes - Cap that was applied.
 * @returns {string} Plain-language error message.
 */
export function payloadSizeErrorMessage(
  _result: Exclude<PayloadSizeResult, { ok: true }>,
  subject: string,
  maxBodyBytes: number,
): string {
  return `The ${subject} exceeds the ${formatBytes(maxBodyBytes)} limit. Please reduce the size or the number of inline images and try again.`;
}
