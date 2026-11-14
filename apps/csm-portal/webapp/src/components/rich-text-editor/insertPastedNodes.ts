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

import { $isCodeNode } from "@lexical/code";
import { $isHeadingNode, $isQuoteNode } from "@lexical/rich-text";
import { $createListItemNode, $isListItemNode, $isListNode, type ListItemNode, type ListNode } from "@lexical/list";
import { $findMatchingParent, $insertNodeToNearestRootAtCaret } from "@lexical/utils";
import {
  $caretFromPoint,
  $copyNode,
  $createParagraphNode,
  $getSelection,
  $isDecoratorNode,
  $isElementNode,
  $isLineBreakNode,
  $isParagraphNode,
  $isRangeSelection,
  $isRootOrShadowRoot,
  $isTextNode,
  type ElementNode,
  type LexicalNode,
  type PointType,
  type RangeSelection,
} from "lexical";
import { $trailingBreaksToParagraphs } from "@components/rich-text-editor/splitLineBreakLines";

const isBlockNode = (node: LexicalNode): boolean =>
  ($isElementNode(node) || $isDecoratorNode(node)) && !node.isInline();

/** Runs of inline nodes between blocks become paragraphs, like Lexical's own insertion does. */
function wrapInlineRuns(nodes: LexicalNode[]): LexicalNode[] {
  const out: LexicalNode[] = [];
  let run: ElementNode | null = null;
  for (const node of nodes) {
    if (isBlockNode(node)) {
      run = null;
      out.push(node);
    } else {
      if (!run) {
        run = $createParagraphNode();
        out.push(run);
      }
      run.append(node);
    }
  }
  return out;
}

/** A paragraph or heading: a block whose content is only inline nodes and can be merged into another text block. */
const isTextualBlock = (node: LexicalNode): node is ElementNode =>
  $isParagraphNode(node) || $isHeadingNode(node);

/** A block that holds inline content directly (the caret's host in a merge). */
const isTextHost = (node: LexicalNode): boolean =>
  $isParagraphNode(node) || $isHeadingNode(node) || $isQuoteNode(node) || $isListItemNode(node);

/** Whether anything of `block` lies before the collapsed `point`. */
function $hasContentBefore(point: PointType, block: ElementNode): boolean {
  let node: LexicalNode = point.getNode();
  if (point.offset > 0) return true;
  while (!node.is(block)) {
    if (node.getPreviousSibling()) return true;
    const parent: LexicalNode | null = node.getParent();
    if (!parent) return false;
    node = parent;
  }
  return false;
}

/** Whether anything of `block` lies after the collapsed `point`. */
function $hasContentAfter(point: PointType, block: ElementNode): boolean {
  let node: LexicalNode = point.getNode();
  if ($isTextNode(node)) {
    if (point.offset < node.getTextContentSize()) return true;
  } else if ($isElementNode(node) && point.offset < node.getChildrenSize()) {
    return true;
  }
  while (!node.is(block)) {
    if (node.getNextSibling()) return true;
    const parent: LexicalNode | null = node.getParent();
    if (!parent) return false;
    node = parent;
  }
  return false;
}

/**
 * Splits the item's inline content at `point` and returns the nodes after it (now direct children
 * of `host`, still attached after the head), or null when the caret is not in the item's own
 * inline content (nothing is changed then). The caret may sit in text, or inside inline elements
 * of the item (a link, ...): each one is split at the caret, and the tail half is a copy of it
 * (`$copyNode`), so the link stays on both halves.
 */
function $splitInlineTail(host: ListItemNode, point: PointType): LexicalNode[] | null {
  const node = point.getNode();
  // Every ancestor between the caret and the item must be an inline element.
  for (let cur: LexicalNode = node; !cur.is(host); ) {
    const parent: LexicalNode | null = cur.getParent();
    if (!parent) return null;
    if (!parent.is(host) && !($isElementNode(parent) && parent.isInline())) return null;
    cur = parent;
  }
  let container: ElementNode;
  let tail: LexicalNode[];
  if ($isTextNode(node)) {
    container = node.getParentOrThrow();
    if (point.offset > 0 && point.offset < node.getTextContentSize()) {
      const rest = node.splitText(point.offset)[1];
      tail = [rest, ...rest.getNextSiblings()];
    } else if (point.offset === 0) {
      tail = [node, ...node.getNextSiblings()];
    } else {
      tail = node.getNextSiblings();
    }
  } else if ($isElementNode(node) && (node.is(host) || node.isInline())) {
    container = node;
    tail = node.getChildren().slice(point.offset);
  } else {
    return null;
  }
  while (!container.is(host)) {
    const inline: ElementNode = container;
    container = inline.getParentOrThrow();
    if (tail.length > 0) {
      const copy = $copyNode(inline);
      inline.insertAfter(copy);
      copy.append(...tail);
      tail = [copy, ...copy.getNextSiblings()];
    } else {
      tail = inline.getNextSiblings();
    }
    if (inline.isEmpty()) inline.remove();
  }
  return tail;
}

/**
 * A list pasted into a list item: its items become siblings of the caret's item at ITS depth
 * (a nested item gets level-2 siblings, its sub-list items stay level 3), in the same list, so
 * the items after the caret item stay where they were. The text after the caret becomes an
 * item of its own after the pasted ones. Returns false (nothing changed) when the caret is
 * not in the item's own text (or inline elements such as a link), so the caller splits up to
 * the root instead.
 */
function $insertListItemsInPlace(host: ListItemNode, point: PointType, list: ListNode): boolean {
  if (!$isListNode(host.getParent())) return false;
  const tailNodes = $splitInlineTail(host, point);
  if (tailNodes === null) return false;

  // The caret item keeps its own sub-list: the pasted items go after it.
  let cursor: LexicalNode = host;
  for (let next = host.getNextSibling(); next && $isListItemNode(next) && $isListNode(next.getFirstChild()); next = next.getNextSibling()) {
    cursor = next;
  }
  const items = list.getChildren();
  for (const item of items) {
    cursor.insertAfter(item);
    cursor = item;
  }
  if (tailNodes.length > 0) {
    const tailItem = $createListItemNode();
    tailItem.append(...tailNodes);
    cursor.insertAfter(tailItem);
  }
  if (host.isEmpty()) host.remove();
  const lastItem = items[items.length - 1];
  if (lastItem) $selectEnd(lastItem);
  return true;
}

function $selectEnd(node: LexicalNode): void {
  if ($isElementNode(node)) node.selectEnd();
  else if ($isTextNode(node)) node.select();
  else node.selectNext();
}

/**
 * Inserts pasted nodes at the selection. Inline-only content, and anything with no
 * ordinary block around the caret (root, code block, table cell element), goes
 * through `selection.insertNodes` unchanged. When the content holds block nodes,
 * Lexical's own `insertNodes` merges the first block's children into the caret's block
 * (a list's items into a heading or list item, a quote into a paragraph) and appends the
 * text after the caret to the innermost last block (`c world`, `<li>y tail</li>`), and it
 * leaves an empty paragraph behind. So instead:
 *
 * 1. a leading paragraph/heading is merged inline at the caret when the caret has text
 *    before it on its line (text pasted onto a line continues that line, as in a word
 *    processor); at the start of a block it stays a block of its own;
 * 2. the rest is inserted BETWEEN the two halves of the caret's block (split up to the root
 *    or table cell, so a heading, quote or list item is split, never nested into);
 * 3. text after the caret joins a trailing pasted paragraph, otherwise stays in its own block
 *    after the pasted content; an empty half is dropped, never left as `<p></p>`.
 *
 * The caret ends at the end of the pasted content. Call it inside an update.
 */
export function $insertPastedNodes(selection: RangeSelection, nodes: LexicalNode[]): void {
  if (nodes.length === 0) return;
  if (!nodes.some(isBlockNode)) {
    selection.insertNodes(nodes);
    return;
  }
  if (!selection.isCollapsed()) selection.removeText();

  const host = $findMatchingParent(selection.anchor.getNode(), (n) => $isElementNode(n) && !n.isInline());
  if (
    !host ||
    !$isElementNode(host) ||
    $isRootOrShadowRoot(host) ||
    $findMatchingParent(host, $isCodeNode) !== null ||
    $isCodeNode(host)
  ) {
    selection.insertNodes(nodes);
    return;
  }

  let blocks = wrapInlineRuns(nodes);
  const wasEmpty = host.isEmpty();

  // 1. A leading paragraph/heading continues the caret's line.
  if (!wasEmpty && isTextHost(host) && isTextualBlock(blocks[0]) && $hasContentBefore(selection.anchor, host)) {
    const inline = blocks[0].getChildren();
    blocks = blocks.slice(1);
    if (inline.length > 0) selection.insertNodes(inline);
    if (blocks.length === 0) return;
  }

  const current = $getSelection();
  const point = $isRangeSelection(current) ? current.anchor : selection.anchor;
  const hasTail = !wasEmpty && $hasContentAfter(point, host);

  // A list into a list item keeps the item's depth.
  if (blocks.length === 1 && $isListNode(blocks[0]) && $isListItemNode(host)) {
    if ($insertListItemsInPlace(host, point, blocks[0])) return;
  }

  // 2. The rest goes between the halves of the caret's block.
  let caret = $caretFromPoint(point, "next");
  for (const block of blocks) {
    caret = $insertNodeToNearestRootAtCaret(block, caret, { $shouldSplit: () => false });
  }

  const last = blocks[blocks.length - 1];

  // 3. Text after the caret.
  let caretIndex: number | null = null;
  if (hasTail) {
    let tail: LexicalNode | null = last.getLatest().getNextSibling();
    while (tail && ($isListNode(tail) || ($isListItemNode(tail) && $isListNode(tail.getFirstChild())))) {
      tail = (tail as ElementNode).getFirstChild();
    }
    if (tail && isTextualBlock(last) && $isElementNode(tail)) {
      caretIndex = last.getChildrenSize();
      last.append(...tail.getChildren());
      tail.remove();
    }
  }

  // An emptied half is dropped. The head's last `<br>` only ended the caret's own (now replaced)
  // line, so it goes; any break before it is a blank line the user typed (`x<br><br>` + paste is
  // `x`, a blank line, the paste), which a browser would not show as a trailing `<br>`, so in a
  // paragraph each becomes an empty paragraph, as the toolbar conversion does.
  if (host.isAttached()) {
    if (host.isBefore(last.getLatest())) {
      const lastChild = host.getLastChild();
      if ($isLineBreakNode(lastChild)) {
        lastChild.remove();
        if ($isParagraphNode(host)) $trailingBreaksToParagraphs(host);
      }
    }
    if (host.isEmpty()) host.remove();
  }

  if (caretIndex !== null && $isElementNode(last)) last.select(caretIndex, caretIndex);
  else $selectEnd(last.getLatest());
}
