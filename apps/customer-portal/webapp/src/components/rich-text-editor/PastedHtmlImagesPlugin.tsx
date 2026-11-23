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
  insertImageWithUpload,
  remainingImageBytes,
  remainingImageSlots,
  type ImageUploadHandler,
} from "@components/rich-text-editor/localImageUpload";
import { inlineImageMaxBytes } from "@config/attachmentLimitsConfig";

const ALLOWED_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);
const DATA_IMAGE = /^data:(image\/[a-z0-9.+-]+);base64,([A-Za-z0-9+/\s]*={0,2})$/i;
const HAS_DATA_IMG = /<img[^>]+src\s*=\s*["']?\s*data:/i;

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
 * With an uploader, every such image is taken out of the HTML and uploaded like a
 * pasted picture file instead (when the clipboard also carries the picture file,
 * ClipboardImagePlugin stores that one, so the HTML copy is only dropped). What is left
 * of the HTML pastes as usual. Without an uploader nothing changes.
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
          const dataImages = Array.from(dom.querySelectorAll("img")).filter((img) =>
            /^data:/i.test((img.getAttribute("src") ?? "").trim()),
          );
          if (dataImages.length === 0) return false;

          const carriesFile = Array.from(event.clipboardData.items ?? []).some((item) =>
            item.type.startsWith("image/"),
          );
          const sources = dataImages.map((img) => img.getAttribute("src") ?? "");
          dataImages.forEach((img) => img.remove());
          event.preventDefault();

          if ((dom.body.textContent ?? "").trim() !== "" || dom.body.querySelector("img")) {
            editor.update(() => {
              const selection = $getSelection();
              if ($isRangeSelection(selection)) {
                selection.insertNodes($generateNodesFromDOM(editor, dom));
              }
            });
          }
          if (carriesFile) return true;

          // Read once: an image added in this loop may not be in the editor yet.
          let slots = remainingImageSlots(editor);
          let bytesLeft = remainingImageBytes(editor);
          let reported = false;
          for (const src of sources) {
            const file = dataUriToFile(src);
            if (!file || !ALLOWED_TYPES.has(file.type)) {
              if (!reported) refuse?.("type");
              reported = true;
              continue;
            }
            const refusal =
              file.size > inlineImageMaxBytes()
                ? "size"
                : slots <= 0
                  ? "count"
                  : file.size > bytesLeft
                    ? "size"
                    : null;
            if (refusal) {
              if (!reported) refuse?.(refusal);
              reported = true;
              continue;
            }
            slots -= 1;
            bytesLeft -= file.size;
            insertImageWithUpload(editor, file, "Pasted Image", upload);
          }
          return true;
        },
        COMMAND_PRIORITY_HIGH,
      ),
    [editor],
  );

  return null;
}
