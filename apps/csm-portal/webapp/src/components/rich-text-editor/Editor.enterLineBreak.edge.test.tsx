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

import { render, screen, act, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import {
  $getRoot,
  $isTextNode,
  $isElementNode,
  type LexicalEditor,
  type LexicalNode,
  $getSelection,
  $isRangeSelection,
  $isLineBreakNode,
  type ElementNode,
  UNDO_COMMAND,
  INSERT_PARAGRAPH_COMMAND,
} from "lexical";
import Editor from "./Editor";
import EditorWithSourceToggle from "./EditorWithSourceToggle";
import { tokenizePlainTextPaste } from "./richTextEditor";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn(), clearError: vi.fn() }),
}));

const getEditable = () => screen.getByTestId("case-description-editor");
const getLexical = () =>
  (getEditable() as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;

function enter(init: KeyboardEventInit = {}) {
  const event = new KeyboardEvent("keydown", {
    key: "Enter", code: "Enter", keyCode: 13, bubbles: true, cancelable: true, ...init,
  } as KeyboardEventInit);
  act(() => { getEditable().dispatchEvent(event); });
  return event;
}

/** Select text-node `index` (nth text descendant) from offset a to b; optionally end in another text node. */
function selectText(a: { n: number; off: number }, b: { n: number; off: number } = a) {
  act(() => {
    getLexical().update(() => {
      const texts: LexicalNode[] = [];
      const walk = (node: LexicalNode) => {
        if ($isTextNode(node)) texts.push(node);
        else if ($isElementNode(node)) node.getChildren().forEach(walk);
      };
      walk($getRoot());
      const first = texts[a.n];
      const second = texts[b.n];
      if (!$isTextNode(first) || !$isTextNode(second)) throw new Error("no text");
      const sel = first.select(a.off, a.off);
      sel.anchor.set(first.getKey(), a.off, "text");
      sel.focus.set(second.getKey(), b.off, "text");
    }, { discrete: true });
  });
}

function setup(value: string, props: Partial<React.ComponentProps<typeof Editor>> = {}) {
  const onChange = vi.fn();
  render(<Editor showToolbar={false} value={value} onChange={onChange} enterInsertsLineBreak {...props} />);
  return () => (onChange.mock.calls.at(-1)?.[0] ?? "") as string;
}

describe("Enter edge cases (enterInsertsLineBreak)", () => {
  it("in the middle of text splits the line with a <br> at the caret", async () => {
    const html = setup("<p>HelloWorld</p>");
    selectText({ n: 0, off: 5 });
    enter();
    await waitFor(() => expect(html()).toBe("<p>Hello<br>World</p>"));
  });

  it("in the middle of bold text keeps the bold on both sides", async () => {
    const html = setup("<p><b>HelloWorld</b></p>");
    selectText({ n: 0, off: 5 });
    enter();
    await waitFor(() => expect(html()).toContain("<br>"));
    expect(html().match(/<p/g)?.length).toBe(1);
    expect(html()).toMatch(/Hello.*<br>.*World/);
  });

  it("inside a link stays one paragraph with a <br>", async () => {
    const html = setup('<p>a <a href="https://example.com">linktext</a> b</p>');
    selectText({ n: 1, off: 4 });
    enter();
    await waitFor(() => expect(html()).toContain("<br>"));
    expect(html().match(/<p/g)?.length).toBe(1);
  });

  it("with a selection spanning two plain paragraphs replaces the range with one <br>", async () => {
    const html = setup("<p>Hello</p><p>World</p>");
    selectText({ n: 0, off: 2 }, { n: 1, off: 3 });
    enter();
    await waitFor(() => expect(html()).toContain("<br>"));
    expect(html()).toMatch(/^<p[^>]*>(<span[^>]*>)?He(<\/span>)?<br>(<span[^>]*>)?ld(<\/span>)?<\/p>$/);
  });

  it("with a selection spanning a paragraph AND a list item falls back to Lexical's default", async () => {
    const html = setup("<p>Hello</p><ul><li>item</li></ul>");
    selectText({ n: 0, off: 2 }, { n: 1, off: 2 });
    const event = enter();
    await waitFor(() => expect(event.defaultPrevented).toBe(true));
    // Our plugin must not have turned this into a <br> in a merged paragraph.
    expect(html()).not.toMatch(/<p[^>]*>[^]*<br>[^]*<\/p>/);
  });

  it("on an empty editor inserts a <br> in the single paragraph", async () => {
    const html = setup("");
    act(() => { getEditable().focus(); });
    act(() => { getLexical().update(() => { $getRoot().selectEnd(); }, { discrete: true }); });
    enter();
    await waitFor(() => expect(html()).toContain("<br>"));
    expect(html().match(/<p/g)?.length ?? 0).toBeLessThanOrEqual(1);
  });

  it("Enter on an empty list item exits the list; the next Enter is a <br> in the new paragraph", async () => {
    const html = setup("<ul><li>one</li></ul>");
    selectText({ n: 0, off: 3 });
    enter(); // new empty item
    await waitFor(() => expect((html().match(/<li/g) ?? []).length).toBe(2));
    enter(); // exit list
    await waitFor(() => expect(html()).toMatch(/<\/ul><p/));
    enter(); // now in a plain paragraph
    await waitFor(() => expect(html()).toContain("<br>"));
    expect(html().match(/<p/g)?.length).toBe(1);
  });

  it("INSERT_PARAGRAPH_COMMAND (soft keyboard) also inserts a <br>", async () => {
    const html = setup("<p>Hi</p>");
    selectText({ n: 0, off: 2 });
    act(() => { getLexical().dispatchCommand(INSERT_PARAGRAPH_COMMAND, undefined); });
    await waitFor(() => expect(html()).toBe("<p>Hi<br></p>"));
  });

  it("after Enter the caret sits right after exactly one LineBreakNode (so one Backspace undoes one Enter)", async () => {
    // jsdom has no Selection#modify, so Lexical's backward delete cannot run here; this pins the
    // structure a real Backspace acts on. Real-browser behaviour is NOT covered by this test.
    const html = setup("<p>Hi</p>");
    selectText({ n: 0, off: 2 });
    enter();
    await waitFor(() => expect(html()).toBe("<p>Hi<br></p>"));
    const info = getLexical().getEditorState().read(() => {
      const sel = $getSelection();
      if (!$isRangeSelection(sel)) return null;
      const para = $getRoot().getFirstChild() as ElementNode;
      const prev = sel.anchor.type === "element"
        ? para.getChildAtIndex(sel.anchor.offset - 1)
        : sel.anchor.getNode();
      return { collapsed: sel.isCollapsed(), prevIsBreak: prev !== null && $isLineBreakNode(prev), kids: para.getChildrenSize() };
    });
    expect(info).toEqual({ collapsed: true, prevIsBreak: true, kids: 2 });
  });

  it("undo after Enter removes the <br>", async () => {
    const html = setup("<p>Hi</p>");
    selectText({ n: 0, off: 2 });
    enter();
    await waitFor(() => expect(html()).toBe("<p>Hi<br></p>"));
    act(() => { getLexical().dispatchCommand(UNDO_COMMAND, undefined); });
    await waitFor(() => expect(html()).toBe("<p>Hi</p>"));
  });

  it("an initial value with <br>s keeps them (draft round trip)", async () => {
    const html = setup("<p>a<br><br>b</p>");
    await waitFor(() => expect(getLexical().getEditorState().read(() => $getRoot().getTextContent())).toContain("a"));
    selectText({ n: 1, off: 1 });
    enter();
    await waitFor(() => expect(html()).toBe("<p>a<br><br>b<br></p>"));
  });
});

describe("other consumers are unchanged", () => {
  it("a plain <Editor> still makes a paragraph on Enter", async () => {
    const html = setup("<p>Hi</p>", { enterInsertsLineBreak: false });
    selectText({ n: 0, off: 2 });
    enter();
    await waitFor(() => expect(html().match(/<p/g)?.length).toBe(2));
    expect(html()).toBe("<p>Hi</p><p><br></p>"); // new empty paragraph, not "Hi<br>"
  });

  it("EditorWithSourceToggle still makes a paragraph on Enter", async () => {
    const onChange = vi.fn();
    render(<EditorWithSourceToggle value="<p>Hi</p>" onChange={onChange} />);
    selectText({ n: 0, off: 2 });
    enter();
    await waitFor(() => expect(((onChange.mock.calls.at(-1)?.[0] ?? "") as string).match(/<p/g)?.length).toBe(2));
  });
});

describe("tokenizePlainTextPaste lineBreaks mode", () => {
  const kinds = (t: string) =>
    tokenizePlainTextPaste(t, { lineBreaks: true }).map((x) => (x.type === "text" ? x.value : x.type));
  it("normalises \\r\\n", () => expect(kinds("a\r\nb")).toEqual(["a", "linebreak", "b"]));
  it("trailing newline becomes a trailing break (trimmed on send)", () =>
    expect(kinds("a\n")).toEqual(["a", "linebreak"]));
  it("leading newlines are kept as breaks", () => expect(kinds("\n\na")).toEqual(["linebreak", "linebreak", "a"]));
  it("only newlines", () => expect(kinds("\n\n\n\n")).toEqual(["linebreak", "linebreak"]));
  it("tabs mixed with newlines", () =>
    expect(kinds("a\tb\n\tc")).toEqual(["a", "tab", "b", "linebreak", "tab", "c"]));
  it("lone \\r is a newline", () => expect(kinds("a\rb")).toEqual(["a", "linebreak", "b"]));
  it("3+ newlines collapse to one blank line", () =>
    expect(kinds("a\n\n\n\nb")).toEqual(["a", "linebreak", "linebreak", "b"]));
});
