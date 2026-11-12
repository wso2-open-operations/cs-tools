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

import { $nodesOfType, type LexicalEditor } from "lexical";
import { formatMegabytes } from "@config/attachmentLimitsConfig";
import { ImageNode } from "@components/rich-text-editor/ImageNode";
import {
  ALLOWED_IMAGE_TYPES_LABEL,
  MAX_COMMENT_IMAGES,
  MAX_COMMENT_IMAGES_MESSAGE,
} from "@components/rich-text-editor/richTextConstants";

/**
 * What one comment or description may hold in inline images: at most
 * {@link MAX_COMMENT_IMAGES} of them and the server's inline-image limit
 * (`useAttachmentLimits`) in total. The
 * editor refuses the image that would go over, before anything is uploaded.
 */

/** Size of each local `blob:` preview the editor handed out, by its URL. */
const previewSizes = new Map<string, number>();

/** Remembers the size of the file behind a preview URL, so the editor can total it. */
export function rememberInlineImageSize(previewUrl: string, bytes: number): void {
  previewSizes.set(previewUrl, bytes);
}

/** Why an image was not taken: too many images, or too many bytes in all. */
export type InlineImageRefusal = "count" | "total";

export interface InlineImageRoom {
  count: number;
  bytes: number;
}

/**
 * The images the editor's content holds now: `blob:` previews (sizes known from
 * {@link rememberInlineImageSize}) and stored `/<uuid>` images (size unknown,
 * counted as zero bytes).
 */
export function inlineImagesInEditor(editor: LexicalEditor): InlineImageRoom {
  return editor.getEditorState().read(() => {
    const room: InlineImageRoom = { count: 0, bytes: 0 };
    for (const node of $nodesOfType(ImageNode)) {
      const src = node.__src;
      if (src.startsWith("blob:")) {
        room.count += 1;
        room.bytes += previewSizes.get(src) ?? 0;
      } else if (/^\/[0-9a-f-]{36}$/i.test(src)) {
        room.count += 1;
      }
    }
    return room;
  });
}

/** Whether one more image of `bytes` fits after `room`; the reason if not. */
export function inlineImageRefusal(
  room: InlineImageRoom,
  bytes: number,
  maxBytes: number,
): InlineImageRefusal | null {
  if (room.count >= MAX_COMMENT_IMAGES) return "count";
  if (room.bytes + bytes > maxBytes) return "total";
  return null;
}

export function inlineImagesTotalMessage(maxBytes: number): string {
  return `Images in one comment or description can't exceed ${formatMegabytes(maxBytes)} in total.`;
}

/** Why a pasted image was refused. */
export type ImagePasteRefusal = "size" | "type" | "count" | "total" | "disabled";

/** The message for a refused pasted image. */
export function imagePasteRefusalMessage(reason: ImagePasteRefusal, maxBytes: number): string {
  switch (reason) {
    case "count":
      return MAX_COMMENT_IMAGES_MESSAGE;
    case "total":
      return inlineImagesTotalMessage(maxBytes);
    case "disabled":
      return "Images can't be added to this field.";
    case "type":
      return `That image format isn't supported. Allowed formats: ${ALLOWED_IMAGE_TYPES_LABEL}.`;
    default:
      return `Pasted image exceeds the maximum allowed size of ${formatMegabytes(maxBytes)}.`;
  }
}
