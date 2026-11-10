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


import { $getRoot, $isElementNode, type LexicalEditor, type LexicalNode } from "lexical";
import { $isImageNode } from "@components/rich-text-editor/ImageNode";
import { INLINE_IMAGE_MAX_COUNT } from "@constants/common";
import { inlineImagesTotalMaxBytes } from "@config/attachmentLimitsConfig";
import { INSERT_IMAGE_COMMAND } from "@features/support/utils/richTextEditor";

/**
 * Called with an image the user added and the blob: URL it is shown under until
 * it is saved. Resolves once the image is stored; rejecting removes it from the
 * editor again (the caller reports the error).
 */
export type ImageUploadHandler = (file: File, previewUrl: string) => Promise<void>;

function $removeImagesBySrc(node: LexicalNode, src: string): void {
  if ($isImageNode(node) && node.getSrc() === src) {
    node.remove();
    return;
  }
  if ($isElementNode(node)) {
    for (const child of node.getChildren()) $removeImagesBySrc(child, src);
  }
}

/** A preview (blob:) or an already stored ("/<uuid>") image counts toward the cap. */
const COUNTED_IMAGE_SRC = /^(blob:|\/[0-9a-f-]{8,}$)/i;

/** Size of each image added through insertImageWithUpload, by its preview URL. */
const previewSizes = new Map<string, number>();

function $collectImageSrcs(node: LexicalNode, out: string[]): string[] {
  if ($isImageNode(node)) {
    const src = node.getSrc();
    if (COUNTED_IMAGE_SRC.test(src)) out.push(src);
  } else if ($isElementNode(node)) {
    for (const child of node.getChildren()) $collectImageSrcs(child, out);
  }
  return out;
}

function imageSrcsIn(editor: LexicalEditor): string[] {
  return editor.getEditorState().read(() => $collectImageSrcs($getRoot(), []));
}

/** How many more images the draft can take before it reaches INLINE_IMAGE_MAX_COUNT. */
export function remainingImageSlots(editor: LexicalEditor): number {
  return Math.max(0, INLINE_IMAGE_MAX_COUNT - imageSrcsIn(editor).length);
}

/** Total size of the images in the draft that are still local previews, in bytes. */
export function usedImageBytes(editor: LexicalEditor): number {
  return imageSrcsIn(editor).reduce((sum, src) => sum + (previewSizes.get(src) ?? 0), 0);
}

/** How many more bytes of images the draft can take before it reaches the total limit. */
export function remainingImageBytes(editor: LexicalEditor): number {
  return Math.max(0, inlineImagesTotalMaxBytes() - usedImageBytes(editor));
}

/** Why an image of `size` bytes cannot be added to the draft, or null if it can. */
export function imageRefusal(editor: LexicalEditor, size: number): "count" | "size" | null {
  if (remainingImageSlots(editor) <= 0) return "count";
  if (size > remainingImageBytes(editor)) return "size";
  return null;
}

/**
 * Adds an image to the editor without reading it into the page as base64: it is
 * shown from a local blob: URL while `onImageUpload` stores it. If that fails the
 * image is taken out of the editor again and the URL released.
 */
export function insertImageWithUpload(
  editor: LexicalEditor,
  file: File,
  altText: string,
  onImageUpload: ImageUploadHandler,
): void {
  const previewUrl = URL.createObjectURL(file);
  previewSizes.set(previewUrl, file.size);
  editor.dispatchCommand(INSERT_IMAGE_COMMAND, { src: previewUrl, altText });
  onImageUpload(file, previewUrl).catch(() => {
    editor.update(() => $removeImagesBySrc($getRoot(), previewUrl));
    previewSizes.delete(previewUrl);
    URL.revokeObjectURL(previewUrl);
  });
}
