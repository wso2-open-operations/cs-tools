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

import { formatBytes } from "@utils/formatBytes";

/**
 * The files an announcement may carry. These limits are the entity service's
 * (it checks every one again, and by the file's content, not its name); they are
 * repeated here only so the sender is told straight away instead of after an
 * upload round trip, so keep them in step with it.
 */
export const MAX_ANNOUNCEMENT_ATTACHMENTS = 5;
export const MAX_ANNOUNCEMENT_ATTACHMENT_BYTES = 10 * 1024 * 1024;

/** Extensions an announcement accepts: documents customers open (a PDF above all), plain images and text. */
export const ANNOUNCEMENT_ATTACHMENT_EXTENSIONS = [
  "pdf",
  "png",
  "jpg",
  "jpeg",
  "gif",
  "doc",
  "docx",
  "xls",
  "xlsx",
  "ppt",
  "pptx",
  "txt",
  "csv",
] as const;

/** The `accept` attribute of the file picker. */
export const ANNOUNCEMENT_ATTACHMENT_ACCEPT = ANNOUNCEMENT_ATTACHMENT_EXTENSIONS.map((e) => `.${e}`).join(",");

/** Human-readable form of the accepted types, for the hint under the picker. */
export const ANNOUNCEMENT_ATTACHMENT_TYPES_LABEL = "PDF, images, Word, Excel, PowerPoint, text";

function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot < 0 ? "" : name.slice(dot + 1).toLowerCase();
}

/**
 * Why `file` cannot be added to an announcement that already carries
 * `existingNames` (its files, saved or waiting to be saved), or `null` when it
 * can. Names are compared case-insensitively, like the entity service does.
 */
export function announcementAttachmentProblem(file: File, existingNames: string[]): string | null {
  if (!(ANNOUNCEMENT_ATTACHMENT_EXTENSIONS as readonly string[]).includes(extensionOf(file.name))) {
    return `"${file.name}" can't be sent with an announcement. Use a ${ANNOUNCEMENT_ATTACHMENT_TYPES_LABEL.toLowerCase()} file.`;
  }
  if (file.size === 0) {
    return `"${file.name}" is empty.`;
  }
  if (file.size > MAX_ANNOUNCEMENT_ATTACHMENT_BYTES) {
    return `"${file.name}" is too large. The maximum is ${formatBytes(MAX_ANNOUNCEMENT_ATTACHMENT_BYTES)} per file.`;
  }
  const lower = file.name.toLowerCase();
  if (existingNames.some((n) => n.toLowerCase() === lower)) {
    return `A file named "${file.name}" is already attached.`;
  }
  if (existingNames.length >= MAX_ANNOUNCEMENT_ATTACHMENTS) {
    return `An announcement can carry at most ${MAX_ANNOUNCEMENT_ATTACHMENTS} files. Remove one to add another.`;
  }
  return null;
}

/** Read a File as a base64 data URI (`data:<mime>;base64,...`). */
export function readFileAsDataUrl(file: File): Promise<string> {
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
