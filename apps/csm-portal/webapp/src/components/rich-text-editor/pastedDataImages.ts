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

import { inlineImageMaxBytes } from "@config/attachmentLimitsConfig";
import {
  inlineImageRefusal,
  rememberInlineImageSize,
  type ImagePasteRefusal,
  type InlineImageRoom,
} from "@components/rich-text-editor/inlineImageLimits";
import { ALLOWED_IMAGE_MIME_TYPES } from "@components/rich-text-editor/richTextConstants";
import type { ImageUploadHandler } from "@components/rich-text-editor/richTextEditor";

const DATA_IMAGE = /^data:(image\/[a-z0-9.+-]+);base64,([A-Za-z0-9+/\s]*={0,2})$/i;

/** The decoded size of a `data:...;base64,` image from its length (whitespace ignored), or null. */
function dataImageBytes(src: string): { type: string; bytes: number } | null {
  const match = DATA_IMAGE.exec(src.trim());
  if (!match) return null;
  const payload = match[2].replace(/\s/g, "");
  const padding = payload.endsWith("==") ? 2 : payload.endsWith("=") ? 1 : 0;
  return { type: match[1].toLowerCase(), bytes: Math.floor((payload.length * 3) / 4) - padding };
}

/** The type and size of a picture file the clipboard carries. */
export interface ClipboardImageFile {
  type: string;
  size: number;
}

/**
 * Removes the `data:` images that are the HTML copy of a picture file the clipboard
 * also carries (same type and size, each file taken by one image), so it is not stored
 * twice. Any other `data:` image stays for {@link uploadPastedDataImages}.
 */
export function dropDataImagesMatchingFiles(dom: Document, files: ClipboardImageFile[]): void {
  const unmatched = [...files];
  for (const img of Array.from(dom.querySelectorAll("img"))) {
    const info = dataImageBytes(img.getAttribute("src") ?? "");
    if (!info) continue;
    const at = unmatched.findIndex((f) => f.type.toLowerCase() === info.type && f.size === info.bytes);
    if (at === -1) continue;
    unmatched.splice(at, 1);
    img.remove();
  }
}

/** The image a `data:...;base64,` address holds as a file, or null when it is not one. */
function dataUriToFile(src: string): File | null {
  const match = DATA_IMAGE.exec(src.trim());
  if (!match) return null;
  const binary = atob(match[2].replace(/\s/g, ""));
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
  const type = match[1].toLowerCase();
  return new File([bytes], `pasted-image.${type.split("/")[1].split("+")[0]}`, { type });
}

/**
 * Pasted HTML (a Word/Docs/Gmail page, text beside a screenshot) can carry its
 * pictures as `<img src="data:image/...;base64,...">`. Turning that into editor
 * content would embed the whole image in the comment, so each one is handed to
 * the editor's uploader like a pasted picture file (its `src` becomes the local
 * preview), or removed with its refusal reason when it cannot be taken: no
 * uploader, an unsupported type, over the size limit or the draft's room.
 * Other images (https, stored, blob) are left as they are. Mutates `dom`;
 * returns the room left.
 */
export function uploadPastedDataImages(
  dom: Document,
  room: InlineImageRoom,
  upload: ImageUploadHandler | undefined,
  onRefused: (reason: ImagePasteRefusal) => void,
): InlineImageRoom {
  let left = room;
  for (const img of Array.from(dom.querySelectorAll("img"))) {
    const src = img.getAttribute("src") ?? "";
    if (!/^data:/i.test(src.trim())) continue;
    const refuse = (reason: ImagePasteRefusal) => {
      img.remove();
      onRefused(reason);
    };
    if (!upload) {
      refuse("disabled");
      continue;
    }
    // Reject by decoded length first (whitespace in the payload does not count):
    // a huge image is not worth decoding.
    const estimate = dataImageBytes(src);
    if (estimate && estimate.bytes > inlineImageMaxBytes()) {
      refuse("size");
      continue;
    }
    let file: File | null = null;
    try {
      file = dataUriToFile(src);
    } catch {
      file = null;
    }
    if (!file || !ALLOWED_IMAGE_MIME_TYPES.includes(file.type as (typeof ALLOWED_IMAGE_MIME_TYPES)[number])) {
      refuse("type");
      continue;
    }
    if (file.size > inlineImageMaxBytes()) {
      refuse("size");
      continue;
    }
    const refusal = inlineImageRefusal(left, file.size);
    if (refusal) {
      refuse(refusal);
      continue;
    }
    const preview = upload(file);
    rememberInlineImageSize(preview, file.size);
    img.setAttribute("src", preview);
    left = { count: left.count + 1, bytes: left.bytes + file.size };
  }
  return left;
}
