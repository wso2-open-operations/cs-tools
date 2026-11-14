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

import {
  $createParagraphNode,
  $getRoot,
  $getSelection,
  $isRangeSelection,
  $isLineBreakNode,
  $isElementNode,
  $isParagraphNode,
  $isTextNode,
  type LexicalNode,
  type ParagraphNode,
  type PointType,
  type RangeSelection,
} from "lexical";

/** The paragraph (if any) that `node` is, or sits directly inside. */
function owningParagraph(node: LexicalNode): ParagraphNode | null {
  let current: LexicalNode | null = node;
  while (current) {
    if ($isParagraphNode(current)) return current;
    current = current.getParent();
  }
  return null;
}

/** Index in `children` where each line starts (a line ends at a LineBreakNode). */
function lineStarts(children: LexicalNode[]): number[] {
  const starts = [0];
  children.forEach((child, index) => {
    if ($isLineBreakNode(child)) starts.push(index + 1);
  });
  return starts;
}

/** Which line the child offset `offset` (between children) falls on. */
function lineOfOffset(starts: number[], offset: number): number {
  let line = 0;
  for (let i = 0; i < starts.length; i++) if (starts[i] <= offset) line = i;
  return line;
}

/** Child offset of `point` within `paragraph`, or null when the point is elsewhere. */
function childOffsetIn(paragraph: ParagraphNode, point: PointType): number | null {
  const node = point.getNode();
  if (node.is(paragraph)) return point.offset;
  let child: LexicalNode | null = node;
  while (child && !(child.getParent()?.is(paragraph) ?? false)) child = child.getParent();
  return child ? child.getIndexWithinParent() : null;
}

/**
 * Makes the lines the selection touches into paragraphs of their own, for the
 * block conversions (list, heading) of an editor whose Enter key inserts a
 * `<br>` (`enterInsertsLineBreak`): there a "line" is a run of children
 * between LineBreakNodes, so converting the paragraph as it stands would make
 * ONE list item/heading holding several `<br>`-separated lines.
 *
 * Only plain paragraphs are touched, and only the selected lines are split
 * out: the lines before and after stay together in their own paragraph (with
 * their `<br>`s) so unselected text is not re-spaced. The selection is kept
 * on the same text. Run it inside `editor.update`, before `$setBlocksType` /
 * the list command.
 */
export function $splitSelectedLinesIntoParagraphs(
  selection: RangeSelection,
  options: { dropBlankLines?: boolean } = {},
): void {
  // Paragraphs made by the split from a selected line with no text and no inline
  // node (an image counts as content), and how many selected lines have content.
  const blanks: ParagraphNode[] = [];
  let kept = 0;
  const paragraphs: ParagraphNode[] = [];
  for (const node of selection.getNodes()) {
    const paragraph = owningParagraph(node);
    if (paragraph && !paragraphs.some((p) => p.is(paragraph))) paragraphs.push(paragraph);
  }

  const [start, end] = selection.isBackward()
    ? [selection.focus, selection.anchor]
    : [selection.anchor, selection.focus];

  for (const paragraph of paragraphs) {
    const children = paragraph.getChildren();
    if (!children.some($isLineBreakNode)) {
      kept++;
      continue;
    }

    const starts = lineStarts(children);
    const lastLine = starts.length - 1;
    const startOffset = childOffsetIn(paragraph, start);
    const endOffset = childOffsetIn(paragraph, end);
    const firstSelected = startOffset === null ? 0 : lineOfOffset(starts, startOffset);
    const lastSelected = endOffset === null ? lastLine : lineOfOffset(starts, endOffset);

    // Segments in document order: [from, to) child indexes, whether they are
    // split-out selected lines (no inner breaks) or kept-together context.
    const lineEnd = (line: number) =>
      line === lastLine ? children.length : starts[line + 1] - 1;
    const segments: { from: number; to: number; firstLine: number; lastLine: number }[] = [];
    if (firstSelected > 0) {
      segments.push({ from: 0, to: lineEnd(firstSelected - 1), firstLine: 0, lastLine: firstSelected - 1 });
    }
    for (let line = firstSelected; line <= lastSelected; line++) {
      segments.push({ from: starts[line], to: lineEnd(line), firstLine: line, lastLine: line });
    }
    if (lastSelected < lastLine) {
      segments.push({
        from: starts[lastSelected + 1],
        to: children.length,
        firstLine: lastSelected + 1,
        lastLine,
      });
    }
    if (segments.length === 1) {
      kept++;
      continue;
    }

    // Element points directly on the paragraph need re-homing afterwards.
    const rehome = [start, end]
      .filter((p) => p.type === "element" && p.getNode().is(paragraph))
      .map((point) => {
        const line = lineOfOffset(starts, point.offset);
        const index = segments.findIndex((s) => s.firstLine <= line && line <= s.lastLine);
        return { point, index, offset: point.offset - segments[index].from };
      });

    const targets: ParagraphNode[] = [paragraph];
    for (let i = 1; i < segments.length; i++) {
      const created = $createParagraphNode()
        .setFormat(paragraph.getFormatType())
        .setIndent(paragraph.getIndent());
      targets[i - 1].insertAfter(created);
      targets.push(created);
    }
    segments.forEach((segment, i) => {
      if (i > 0) targets[i].append(...children.slice(segment.from, segment.to));
    });
    // The first segment keeps its own children; drop everything after it,
    // including the breaks that used to separate the segments.
    for (let i = segments[0].to; i < children.length; i++) {
      const child = children[i];
      if ($isLineBreakNode(child) && child.getParent()?.is(paragraph)) child.remove();
    }
    for (const { point, index, offset } of rehome) {
      point.set(targets[index].getKey(), offset, "element");
    }
    if (firstSelected > 0) $trailingBreaksToParagraphs(targets[0]);
    segments.forEach((segment, i) => {
      const selectedLine = segment.firstLine === segment.lastLine && segment.firstLine >= firstSelected && segment.firstLine <= lastSelected;
      if (!selectedLine) return;
      const blank = children
        .slice(segment.from, segment.to)
        .every((child) => $isTextNode(child) && child.getTextContent().trim() === "");
      if (blank) blanks.push(targets[i]);
      else kept++;
    });
  }

  if (options.dropBlankLines) $dropBlankParagraphs(blanks, kept > 0, start, end);
}

/**
 * The context paragraph before a split-out line ends in the blank lines the user
 * typed (`a<br>` + `<br>b`), but a trailing `<br>` is no line in HTML (the editor
 * shows it, a browser does not), so the blank line would vanish from the sent
 * reply. Each trailing break becomes an empty paragraph (`<p>a</p><p><br></p>`),
 * which shows as a blank line in the editor and in every renderer.
 */
function $trailingBreaksToParagraphs(before: ParagraphNode): void {
  let last = before.getLastChild();
  let anchor: ParagraphNode = before;
  while ($isLineBreakNode(last)) {
    last.remove();
    const blank = $createParagraphNode()
      .setFormat(before.getFormatType())
      .setIndent(before.getIndent());
    anchor.insertAfter(blank);
    anchor = blank;
    last = before.getLastChild();
  }
}

/**
 * Removes the blank paragraphs the split produced, moving a selection point that
 * sat on one to the nearest paragraph that stays (the start point forward, the
 * end point backward). When every selected line is blank, one is kept so the
 * list conversion still has something to turn into an (empty) item.
 */
function $dropBlankParagraphs(
  blanks: ParagraphNode[],
  hasContentLines: boolean,
  start: PointType,
  end: PointType,
): void {
  const removed = hasContentLines ? blanks : blanks.slice(1);
  if (removed.length === 0) return;
  const removedKeys = new Set(removed.map((node) => node.getKey()));
  const sibling = (node: LexicalNode, next: boolean): ParagraphNode | null => {
    let current = next ? node.getNextSibling() : node.getPreviousSibling();
    while (current && removedKeys.has(current.getKey())) {
      current = next ? current.getNextSibling() : current.getPreviousSibling();
    }
    return $isParagraphNode(current) ? current : null;
  };
  const place = (point: PointType, target: ParagraphNode | null, atEnd: boolean) => {
    if (target) point.set(target.getKey(), atEnd ? target.getChildrenSize() : 0, "element");
  };

  for (const node of removed) {
    for (const [point, isStart] of [[start, true], [end, false]] as const) {
      const owner = owningParagraph(point.getNode());
      if (!owner || !owner.is(node)) continue;
      const forward = sibling(node, true);
      const backward = sibling(node, false);
      if (isStart) place(point, forward ?? backward, forward === null);
      else place(point, backward ?? forward, backward !== null);
    }
  }
  for (const node of removed) {
    if ($isElementNode(node)) node.remove();
  }
}

/**
 * Puts a collapsed caret that sits on the root itself (a truly empty editor in
 * Chrome) inside a block, so a block conversion converts a paragraph instead of
 * leaving the caret on the root, where the next typed text lands in a new
 * paragraph above the converted block. An empty root gets a paragraph. Any other
 * selection is left alone. Run it inside `editor.update`.
 */
export function $caretOnRootIntoBlock(): void {
  const selection = $getSelection();
  if (!$isRangeSelection(selection) || !selection.isCollapsed()) return;
  const root = $getRoot();
  if (selection.anchor.type !== "element" || !selection.anchor.getNode().is(root)) return;
  const offset = selection.anchor.offset;
  const target = root.getChildAtIndex(offset);
  if (target && $isElementNode(target)) {
    target.selectStart();
    return;
  }
  const previous = root.getChildAtIndex(offset - 1);
  if (previous && $isElementNode(previous)) {
    previous.selectEnd();
    return;
  }
  if (root.getChildrenSize() === 0) {
    const paragraph = $createParagraphNode();
    root.append(paragraph);
    paragraph.select();
  }
}
