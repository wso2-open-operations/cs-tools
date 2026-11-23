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

import { $generateNodesFromDOM } from "@lexical/html";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { $getSelection, $isRangeSelection, COMMAND_PRIORITY_HIGH, PASTE_COMMAND } from "lexical";
import { useEffect, useRef } from "react";
import {
  remainingImageBytes,
  remainingImageSlots,
  startImageUpload,
  type ImageUploadHandler,
} from "@components/rich-text-editor/localImageUpload";
import { inlineImageMaxBytes } from "@config/attachmentLimitsConfig";

const ALLOWED_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);
const DATA_IMAGE = /^data:(image\/[a-z0-9.+-]+);base64,([A-Za-z0-9+/\s]*={0,2})$/i;
const HAS_DATA_IMG = /<img[^>]+src\s*=\s*["']?\s*data:/i;
/** What the editor counts toward the image limit (see localImageUpload). */
const COUNTED_IMAGE_SRC = /^(blob:|\/[0-9a-f-]{8,}$)/i;

/** The type and decoded size of a `data:...;base64,` image, whitespace in the payload ignored. */
function dataImageInfo(src: string): { type: string; bytes: number } | null {
  const match = DATA_IMAGE.exec(src.trim());
  if (!match) return null;
  const payload = match[2].replace(/\s/g, "");
  const padding = payload.endsWith("==") ? 2 : payload.endsWith("=") ? 1 : 0;
  return { type: match[1].toLowerCase(), bytes: Math.floor((payload.length * 3) / 4) - padding };
}

/** The image a `data:...;base64,` address holds as a file, or null when it is not one. */
function dataUriToFile(src: string): File | null {
  const match = DATA_IMAGE.exec(src.trim());
  if (!match) return null;
  try {
    const binary = atob(match[2].replace(/\s/g, ""));
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
    const type = match[1].toLowerCase();
    return new File([bytes], `pasted-image.${type.split("/")[1].split("+")[0]}`, { type });
  } catch {
    return null;
  }
}

/**
 * Pasted HTML can carry a picture as `<img src="data:image/...;base64,...">`: a
 * web page's "Copy image" puts the picture file AND that HTML on the clipboard, and a
 * Word/Docs/Gmail selection carries its pictures that way. Left to the default paste it
 * is embedded in the comment as base64, which can by itself pass the request body limit.
 *
 * With an uploader, each such image is uploaded like a pasted picture file and its
 * `src` is replaced, in place, by the local preview, so the pasted content keeps its
 * order. One that is the HTML copy of a picture file the clipboard also carries (same
 * type and size) is dropped instead: ClipboardImagePlugin stores that file. Images over
 * the limits are dropped with the refusal message. Without an uploader nothing changes.
 */
export default function PastedHtmlImagesPlugin({
  onPasteError,
  onImageUpload,
}: {
  onPasteError?: (reason: "size" | "type" | "count") => void;
  onImageUpload?: ImageUploadHandler;
}): null {
  const [editor] = useLexicalComposerContext();
  const handlersRef = useRef({ onPasteError, onImageUpload });
  useEffect(() => {
    handlersRef.current = { onPasteError, onImageUpload };
  }, [onPasteError, onImageUpload]);

  useEffect(
    () =>
      editor.registerCommand(
        PASTE_COMMAND,
        (event) => {
          const { onPasteError: refuse, onImageUpload: upload } = handlersRef.current;
          if (!upload || !(event instanceof ClipboardEvent) || !event.clipboardData) return false;
          const html = event.clipboardData.getData("text/html");
          if (!html || !HAS_DATA_IMG.test(html)) return false;

          const dom = new DOMParser().parseFromString(html, "text/html");
          const images = Array.from(dom.querySelectorAll("img"));
          const dataImages = images.filter((img) => /^data:/i.test((img.getAttribute("src") ?? "").trim()));
          if (dataImages.length === 0) return false;

          // The picture files the clipboard carries; ClipboardImagePlugin stores those.
          const files = Array.from(event.clipboardData.items ?? []).flatMap((item) => {
            if (!item.type.startsWith("image/")) return [];
            const file = item.getAsFile();
            return file ? [{ type: file.type.toLowerCase(), size: file.size }] : [];
          });

          // Images the pasted content keeps as they are take room in the draft too.
          const retained = images.filter(
            (img) => !dataImages.includes(img) && COUNTED_IMAGE_SRC.test((img.getAttribute("src") ?? "").trim()),
          ).length;
          // Read once, before anything is inserted.
          let slots = remainingImageSlots(editor) - retained;
          let bytesLeft = remainingImageBytes(editor);
          let reported = false;
          const report = (reason: "size" | "type" | "count") => {
            if (!reported) refuse?.(reason);
            reported = true;
          };

          for (const img of dataImages) {
            const src = img.getAttribute("src") ?? "";
            const info = dataImageInfo(src);
            const at = info ? files.findIndex((f) => f.type === info.type && f.size === info.bytes) : -1;
            if (at !== -1) {
              // The HTML copy of a picture file on the clipboard: that file is stored.
              files.splice(at, 1);
              img.remove();
              continue;
            }
            let file: File | null = null;
            let refusal: "size" | "type" | "count" | null = null;
            if (!info || !ALLOWED_TYPES.has(info.type)) refusal = "type";
            else if (info.bytes > inlineImageMaxBytes()) refusal = "size";
            else {
              file = dataUriToFile(src);
              if (!file) refusal = "type";
              else if (slots <= 0) refusal = "count";
              else if (file.size > bytesLeft) refusal = "size";
            }
            if (refusal || !file) {
              img.remove();
              report(refusal ?? "type");
              continue;
            }
            slots -= 1;
            bytesLeft -= file.size;
            img.setAttribute("src", startImageUpload(editor, file, upload));
          }

          event.preventDefault();
          // One insertion for the whole pasted content, images where they were.
          if ((dom.body.textContent ?? "").trim() !== "" || dom.body.querySelector("img")) {
            editor.update(() => {
              const selection = $getSelection();
              if ($isRangeSelection(selection)) {
                selection.insertNodes($generateNodesFromDOM(editor, dom));
              }
            });
          }
          return true;
        },
        COMMAND_PRIORITY_HIGH,
      ),
    [editor],
  );

  return null;
}
