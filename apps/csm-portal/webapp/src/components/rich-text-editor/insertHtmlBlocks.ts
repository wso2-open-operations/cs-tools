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
import {
  $createParagraphNode,
  $getNodeByKey,
  $getRoot,
  $getSelection,
  $insertNodes,
  $isDecoratorNode,
  $isElementNode,
  $isRangeSelection,
  $isTextNode,
  $setSelection,
  type LexicalEditor,
  type LexicalNode,
  type ParagraphNode,
  type PointType,
  type RangeSelection,
} from "lexical";

/** True when a remembered point's node is still in the editor and its offset still fits. */
function pointStillFits(point: PointType): boolean {
  const node = $getNodeByKey(point.key);
  if (!node || !node.isAttached()) return false;
  const size = $isTextNode(node) ? node.getTextContentSize() : $isElementNode(node) ? node.getChildrenSize() : 0;
  return point.offset <= size;
}

/**
 * Inserts `html` as block(s) of its own at the selection (at the end when
 * there is none). Call inside `editor.update`.
 *
 * In an empty spot (blank editor, empty paragraph) Lexical's own insert fills
 * it. Otherwise the cursor's paragraph is split and the HTML's blocks go
 * between the two halves, so neither the text before nor the text after the
 * cursor is merged into it ($insertNodes alone would glue the text after the
 * cursor onto the inserted last line). The HTML goes through Lexical's DOM
 * import, so only node types the editor knows survive; nothing is innerHTML.
 *
 * `lastSelection` is where the cursor was before the editor lost focus: picking
 * from a menu blurs the editor and Lexical then has no selection, so without
 * it every insert would land at the end. Ignored when its nodes are gone.
 */
export function $insertHtmlAsBlocks(
  editor: LexicalEditor,
  html: string,
  lastSelection?: RangeSelection | null,
): void {
  const dom = new DOMParser().parseFromString(html, "text/html");
  const nodes = $generateNodesFromDOM(editor, dom);
  if (!$isRangeSelection($getSelection()) && lastSelection) {
    const restored = lastSelection.clone();
    if (pointStillFits(restored.anchor) && pointStillFits(restored.focus)) {
      $setSelection(restored);
    }
  }
  if (!$isRangeSelection($getSelection())) {
    $getRoot().selectEnd();
  }
  const selection = $getSelection();
  const block = $isRangeSelection(selection) ? selection.anchor.getNode().getTopLevelElement() : null;
  if (!$isRangeSelection(selection) || !block || block.getTextContent().trim() === "") {
    $insertNodes(nodes);
    return;
  }
  const after = selection.insertParagraph();
  if (!after) {
    $insertNodes(nodes);
    return;
  }
  const blocks: LexicalNode[] = [];
  let loose: ParagraphNode | null = null;
  for (const node of nodes) {
    if (($isElementNode(node) || $isDecoratorNode(node)) && !node.isInline()) {
      loose = null;
      blocks.push(node);
    } else {
      // Inline content (bare text, a link) needs a paragraph of its own.
      if (!loose) {
        loose = $createParagraphNode();
        blocks.push(loose);
      }
      loose.append(node);
    }
  }
  blocks.forEach((b) => after.insertBefore(b));
  // The cursor was at the end of its paragraph: the empty second half goes.
  if (after.getTextContent() === "" && after.getChildrenSize() === 0) {
    after.remove();
  }
  const last = blocks[blocks.length - 1];
  if ($isElementNode(last)) last.selectEnd();
}
