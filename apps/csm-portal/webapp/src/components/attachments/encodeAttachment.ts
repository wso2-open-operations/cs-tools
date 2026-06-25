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

/** A file encoded for a case-create payload: raw base64 + display metadata. */
export interface EncodedAttachment {
  name: string;
  /** Raw base64 (no `data:` prefix) — the shape the SRA create payload wants. */
  file: string;
  /** Decoded size in bytes, for display. */
  size: number;
  /**
   * The original File. Used by the post-create upload path
   * (`POST /cases/{id}/attachments`, which standard cases and service requests
   * use because the create endpoint only honors create-payload attachments for
   * security reports).
   */
  raw: File;
}

/**
 * Aggregate-size ceiling for attachments uploaded after case creation (standard
 * cases / service requests). Each upload is a separate request capped per-file
 * at 10 MB, so this is a sanity bound on how much the browser base64-encodes and
 * holds in memory at once — not a single-request body limit.
 */
export const POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES = 100 * 1024 * 1024;

/** Combined base64 size of all attachments (≈ bytes, base64 is ASCII). */
export function totalEncodedBytes(attachments: EncodedAttachment[]): number {
  return attachments.reduce((sum, a) => sum + a.file.length, 0);
}

/** Read a File as raw base64, stripping the `data:<mime>;base64,` prefix. */
export function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = typeof reader.result === "string" ? reader.result : "";
      const comma = result.indexOf(",");
      resolve(comma >= 0 ? result.slice(comma + 1) : result);
    };
    reader.onerror = () =>
      reject(reader.error ?? new Error("Failed to read the file."));
    reader.readAsDataURL(file);
  });
}
