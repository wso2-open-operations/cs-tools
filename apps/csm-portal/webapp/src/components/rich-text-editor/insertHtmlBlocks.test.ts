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

import { describe, expect, it } from "vitest";
import { $createCodeNode, CodeHighlightNode, CodeNode } from "@lexical/code";
import { $createListItemNode, $createListNode, $isListNode, ListItemNode, ListNode } from "@lexical/list";
import {
  $createParagraphNode,
  $createTextNode,
  $getRoot,
  $getSelection,
  $isRangeSelection,
  $setSelection,
  createEditor,
  type LexicalEditor,
  type RangeSelection,
} from "lexical";
import { $insertHtmlAsBlocks } from "./insertHtmlBlocks";

/** A headless editor holding `paragraphs`, cursor at `cursor` in the first one (or none). */
function editorWith(paragraphs: string[], cursor?: number): LexicalEditor {
  const editor = createEditor({ namespace: "test", onError: (e) => { throw e; } });
  editor.update(
    () => {
      const root = $getRoot();
      paragraphs.forEach((text, i) => {
        const p = $createParagraphNode();
        const t = $createTextNode(text);
        p.append(t);
        root.append(p);
        if (i === 0 && cursor !== undefined) t.select(cursor, cursor);
      });
    },
    { discrete: true },
  );
  return editor;
}

function insert(editor: LexicalEditor, html: string): void {
  editor.update(() => $insertHtmlAsBlocks(editor, html), { discrete: true });
}

function paragraphs(editor: LexicalEditor): string[] {
  return editor.getEditorState().read(() => $getRoot().getChildren().map((n) => n.getTextContent()));
}

describe("$insertHtmlAsBlocks", () => {
  it("keeps the text on both sides of a mid-paragraph cursor apart from the template", () => {
    const editor = editorWith(["BeforeAfter"], "Before".length);
    insert(editor, "<p>Template.</p>");
    expect(paragraphs(editor)).toEqual(["Before", "Template.", "After"]);
  });

  it("adds the template as its own paragraph after a cursor at the end", () => {
    const editor = editorWith(["Hi team."], "Hi team.".length);
    insert(editor, "<p>Template.</p>");
    expect(paragraphs(editor)).toEqual(["Hi team.", "Template."]);
  });

  it("goes to the end when the editor has no selection", () => {
    const editor = editorWith(["First."]);
    insert(editor, "<p>Template.</p>");
    expect(paragraphs(editor)).toEqual(["First.", "Template."]);
  });

  it("fills a blank editor without an extra empty paragraph", () => {
    const editor = editorWith([]);
    insert(editor, "<p>Template.</p><p>Second line.</p>");
    expect(paragraphs(editor)).toEqual(["Template.", "Second line."]);
  });

  it("wraps bare inline text in a paragraph of its own", () => {
    const editor = editorWith(["BeforeAfter"], "Before".length);
    insert(editor, "Just text");
    expect(paragraphs(editor)).toEqual(["Before", "Just text", "After"]);
  });
});

describe("$insertHtmlAsBlocks inside a list", () => {
  /** A paragraph, then a bulleted list of `items`, cursor at `offset` in item `at`. */
  function listEditor(items: string[], at: number, offset: number): LexicalEditor {
    const editor = createEditor({
      namespace: "test",
      nodes: [ListNode, ListItemNode],
      onError: (e) => { throw e; },
    });
    editor.update(
      () => {
        const p = $createParagraphNode();
        p.append($createTextNode("Intro."));
        const list = $createListNode("bullet");
        items.forEach((text, i) => {
          const item = $createListItemNode();
          const t = $createTextNode(text);
          item.append(t);
          list.append(item);
          if (i === at) t.select(offset, offset);
        });
        $getRoot().append(p, list);
      },
      { discrete: true },
    );
    return editor;
  }

  it("puts the template after the whole list, never inside it", () => {
    const editor = listEditor(["One", "TwoThree"], 1, "Two".length);
    insert(editor, "<p>Template.</p>");
    const shape = editor.getEditorState().read(() =>
      $getRoot()
        .getChildren()
        .map((n) => ({
          type: n.getType(),
          text: n.getTextContent(),
          kids: $isListNode(n) ? n.getChildren().map((c) => c.getType()) : [],
        })),
    );
    expect(shape).toEqual([
      { type: "paragraph", text: "Intro.", kids: [] },
      { type: "list", text: "One\n\nTwoThree", kids: ["listitem", "listitem"] },
      { type: "paragraph", text: "Template.", kids: [] },
    ]);
  });
});

describe("$insertHtmlAsBlocks inside a code block", () => {
  it("leaves the code untouched and puts the template after the code block", () => {
    const editor = createEditor({
      namespace: "test",
      nodes: [CodeNode, CodeHighlightNode],
      onError: (e) => { throw e; },
    });
    editor.update(
      () => {
        const p = $createParagraphNode();
        p.append($createTextNode("Intro."));
        const code = $createCodeNode();
        const t = $createTextNode("let x = 1;");
        code.append(t);
        $getRoot().append(p, code);
        t.select(4, 4);
      },
      { discrete: true },
    );
    insert(editor, "<p>Template.</p>");
    const shape = editor.getEditorState().read(() =>
      $getRoot()
        .getChildren()
        .map((n) => ({ type: n.getType(), text: n.getTextContent() })),
    );
    expect(shape).toEqual([
      { type: "paragraph", text: "Intro." },
      { type: "code", text: "let x = 1;" },
      { type: "paragraph", text: "Template." },
    ]);
  });
});

describe("$insertHtmlAsBlocks with a remembered cursor", () => {
  /** The cursor in the first paragraph at `offset`, then dropped, as a menu click does. */
  function blurredAt(text: string, offset: number): { editor: LexicalEditor; last: RangeSelection | null } {
    const editor = editorWith([text], offset);
    let last: RangeSelection | null = null;
    editor.getEditorState().read(() => {
      const s = $getSelection();
      if ($isRangeSelection(s)) last = s.clone();
    });
    editor.update(() => $setSelection(null), { discrete: true });
    return { editor, last };
  }

  it("inserts at the cursor the editor had before it lost focus", () => {
    const { editor, last } = blurredAt("Before After", "Before ".length);
    editor.update(() => $insertHtmlAsBlocks(editor, "<p>Template.</p>", last), { discrete: true });
    expect(paragraphs(editor)).toEqual(["Before ", "Template.", "After"]);
  });

  it("falls back to the end when the remembered text is gone", () => {
    const { editor, last } = blurredAt("Before After", "Before ".length);
    editor.update(
      () => {
        $getRoot().clear();
        const p = $createParagraphNode();
        p.append($createTextNode("New."));
        $getRoot().append(p);
        $setSelection(null);
      },
      { discrete: true },
    );
    editor.update(() => $insertHtmlAsBlocks(editor, "<p>Template.</p>", last), { discrete: true });
    expect(paragraphs(editor)).toEqual(["New.", "Template."]);
  });
});
