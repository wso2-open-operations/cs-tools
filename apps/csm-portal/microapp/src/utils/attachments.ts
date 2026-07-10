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

// The BE caps the decoded file at 10 MB; validate client-side for a clear message instead of a
// 413. Mirrors apps/csm-portal/webapp's useCsmCaseAttachments.ts MAX_ATTACHMENT_SIZE_BYTES.
export const MAX_ATTACHMENT_SIZE_BYTES = 10 * 1024 * 1024;

export interface PendingAttachment {
  id: string;
  name: string;
  type: string;
  size: number;
  /** Base64 data URI ("data:<mime>;base64,..."), the shape POST /attachments wants. */
  file: string;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") resolve(reader.result);
      else reject(new Error("Failed to read the file."));
    };
    reader.onerror = () => reject(reader.error ?? new Error("Failed to read the file."));
    reader.readAsDataURL(file);
  });
}

export async function toPendingAttachment(file: File): Promise<PendingAttachment> {
  const dataUrl = await readFileAsDataUrl(file);
  return {
    id: `${file.name}-${file.size}-${file.lastModified}`,
    name: file.name,
    type: file.type || "application/octet-stream",
    size: file.size,
    file: dataUrl,
  };
}
