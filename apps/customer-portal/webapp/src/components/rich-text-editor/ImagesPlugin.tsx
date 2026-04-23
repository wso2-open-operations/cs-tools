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

import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import {
  COMMAND_PRIORITY_EDITOR,
  $insertNodes,
  $getRoot,
  $getSelection,
  $isRangeSelection,
  $createParagraphNode,
  $isRootOrShadowRoot,
} from "lexical";
import { useEffect } from "react";
import { $createImageNode } from "@components/rich-text-editor/ImageNode";
import {
  INSERT_IMAGE_COMMAND,
  deriveAltFromFilename,
  type InsertImagePayload,
} from "@features/support/utils/richTextEditor";

function getPayloadSrc(payload: string | InsertImagePayload): string {
  return typeof payload === "string" ? payload : payload.src;
}

function getPayloadAltText(payload: string | InsertImagePayload): string {
  if (typeof payload === "string") {
    return deriveAltFromFilename(payload) || "Uploaded Image";
  }
  return (
    payload.altText || deriveAltFromFilename(payload.src) || "Uploaded Image"
  );
}

export default function ImagesPlugin(): null {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    return editor.registerCommand(
      INSERT_IMAGE_COMMAND,
      (payload) => {
        const src = getPayloadSrc(payload);
        const altText = getPayloadAltText(payload);
        const imageNode = $createImageNode(src, altText);

        // If the editor is empty or has no range selection, move cursor to end first
        const selection = $getSelection();
        if (!$isRangeSelection(selection)) {
          const root = $getRoot();
          const lastChild = root.getLastChild();
          if (lastChild) {
            lastChild.selectEnd();
          } else {
            root.selectEnd();
          }
        }

        $insertNodes([imageNode]);

        // After inserting a DecoratorNode the cursor is "at" the image and
        // not in a text-editable position. Ensure a paragraph exists after
        // the image and move the cursor there so the user can keep typing.
        const parent = imageNode.getParentOrThrow();
        if ($isRootOrShadowRoot(parent)) {
          const nextSibling = imageNode.getNextSibling();
          if (!nextSibling) {
            const paragraph = $createParagraphNode();
            imageNode.insertAfter(paragraph);
            paragraph.select();
          } else {
            nextSibling.selectStart();
          }
        }

        return true;
      },
      COMMAND_PRIORITY_EDITOR,
    );
  }, [editor]);

  return null;
}
