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

import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { describe, expect, it, vi, beforeAll } from "vitest";
import "@testing-library/jest-dom/vitest";
import { $getRoot, $isElementNode, $isTextNode, UNDO_COMMAND, type LexicalEditor, type LexicalNode } from "lexical";
import Editor from "./Editor";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

beforeAll(() => {
  if (typeof globalThis.ClipboardEvent === "undefined") {
    class ClipboardEventPolyfill extends Event {
      clipboardData: DataTransfer | null;
      constructor(type: string, init: EventInit & { clipboardData?: DataTransfer } = {}) {
        super(type, init);
        this.clipboardData = init.clipboardData ?? null;
      }
    }
    // @ts-expect-error -- test-only polyfill; jsdom has no native ClipboardEvent.
    globalThis.ClipboardEvent = ClipboardEventPolyfill;
  }
  if (!Range.prototype.getBoundingClientRect) {
    Range.prototype.getBoundingClientRect = () =>
      ({ x: 0, y: 0, width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0 }) as DOMRect;
  }
});

function paste(target: Element, data: Record<string, string>): void {
  const event = new ClipboardEvent("paste", {
    bubbles: true,
    cancelable: true,
    clipboardData: {
      getData: (type: string) => data[type] ?? "",
      items: [],
    } as unknown as DataTransfer,
  } as ClipboardEventInit & { clipboardData: DataTransfer });
  act(() => {
    target.dispatchEvent(event);
  });
}

const getEditable = () => screen.getByTestId("case-description-editor");
const getLexical = () =>
  (getEditable() as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;

/** Caret at the end of the first block of `type` (its last text node). */
function caretIn(type: string): void {
  act(() => {
    getLexical().update(
      () => {
        const find = (node: LexicalNode): LexicalNode | null => {
          if (node.getType() === type) return node;
          if ($isElementNode(node)) {
            for (const child of node.getChildren()) {
              const hit = find(child);
              if (hit) return hit;
            }
          }
          return null;
        };
        const block = find($getRoot());
        const text = block && $isElementNode(block) ? block.getLastDescendant() : null;
        if (!text || !$isTextNode(text)) throw new Error("no text");
        text.select();
      },
      { discrete: true },
    );
  });
}

const MD_TITLE = "Paste as Markdown?";
const HTML_TITLE = "Paste formatted content?";

function setup(props: Partial<React.ComponentProps<typeof Editor>> = {}, value?: string) {
  const onChange = vi.fn();
  render(<Editor autoFocus showToolbar={false} markdownPaste onChange={onChange} value={value} {...props} />);
  return () => (onChange.mock.calls.at(-1)?.[0] ?? "") as string;
}

/** Pastes `markdown` as Markdown source and returns the editor's HTML. */
async function pasteAsMarkdown(markdown: string, props: Partial<React.ComponentProps<typeof Editor>> = {}) {
  const html = setup(props);
  paste(getEditable(), { "text/plain": markdown });
  fireEvent.click(await screen.findByRole("button", { name: "Paste as Markdown" }));
  await waitFor(() => expect(html()).not.toBe(""));
  return html();
}

describe("Editor markdownPaste: the prompt", () => {
  it("prompts for Markdown that arrives as plain text only", async () => {
    setup();
    paste(getEditable(), { "text/plain": "# Title\n\n- a\n- b" });
    expect(await screen.findByText(MD_TITLE)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove Formatting" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Keep Formatting" })).not.toBeInTheDocument();
  });

  it("prompts the Markdown variant when the clipboard also carries text/html", async () => {
    setup();
    paste(getEditable(), { "text/html": "<p>x</p>", "text/plain": "**bold** text" });
    expect(await screen.findByText(MD_TITLE)).toBeInTheDocument();
  });

  it("prompts the Markdown variant for a flat code-view HTML twin (VS Code style)", async () => {
    setup();
    paste(getEditable(), {
      "text/html": "<meta charset='utf-8'><div><span># Title</span></div><div><span>- a</span></div><div><span>- b</span></div>",
      "text/plain": "# Title\n- a\n- b",
    });
    expect(await screen.findByText(MD_TITLE)).toBeInTheDocument();
  });

  it.each([
    ["a Word-style list", "<ol><li>a</li><li>b</li></ol>", "1. a\n2. b"],
    ["an Outlook quote", "<blockquote>hi</blockquote>", "> hi"],
    ["page text with **x**", "<p>use <b>x</b> and **x**</p>", "use x and **x**"],
    ["a ChatGPT-style rendered answer", "<ul><li><strong>a</strong></li><li>b</li></ul>", "- a\n- b"],
  ])("rendered HTML from %s keeps the Keep/Remove Formatting prompt", async (_name, html, text) => {
    setup();
    paste(getEditable(), { "text/html": html, "text/plain": text });
    expect(await screen.findByText(HTML_TITLE)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Keep Formatting" })).toBeInTheDocument();
    expect(screen.queryByText(MD_TITLE)).not.toBeInTheDocument();
  });

  it("with the flag off, Markdown-looking HTML keeps the html prompt", async () => {
    setup({ markdownPaste: false });
    paste(getEditable(), { "text/html": "<p>x</p>", "text/plain": "# T\n- a\n- b" });
    expect(await screen.findByText(HTML_TITLE)).toBeInTheDocument();
  });

  it("does not prompt for plain text that is not Markdown (pastes straight in)", async () => {
    const html = setup();
    paste(getEditable(), { "text/plain": "just a normal sentence" });
    await waitFor(() => expect(html()).toContain("just a normal sentence"));
    expect(screen.queryByText(MD_TITLE)).not.toBeInTheDocument();
  });

  it("a non-Markdown HTML paste still gets the Keep/Remove Formatting prompt", async () => {
    setup();
    paste(getEditable(), { "text/html": "<p>Hello <b>Word</b></p>", "text/plain": "Hello Word" });
    expect(await screen.findByText(HTML_TITLE)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Keep Formatting" })).toBeInTheDocument();
  });

  it("without the flag nothing changes: Markdown text pastes straight in", async () => {
    const html = setup({ markdownPaste: false });
    paste(getEditable(), { "text/plain": "# Title" });
    await waitFor(() => expect(html()).toContain("# Title"));
    expect(screen.queryByText(MD_TITLE)).not.toBeInTheDocument();
  });

  it("does not prompt when the selection is in a code block", async () => {
    const html = setup({}, "<pre>let a = 1;</pre>");
    caretIn("code");
    paste(getEditable(), { "text/plain": "# Title\n- a\n- b" });
    await waitFor(() => expect(html()).toContain("# Title"));
    expect(screen.queryByText(MD_TITLE)).not.toBeInTheDocument();
  });

  it("a backdrop click cancels too", async () => {
    const html = setup();
    paste(getEditable(), { "text/plain": "# Title" });
    await screen.findByText(MD_TITLE);
    fireEvent.click(document.querySelector(".MuiBackdrop-root") as HTMLElement);
    await waitFor(() => expect(screen.queryByText(MD_TITLE)).not.toBeInTheDocument());
    expect(html()).not.toContain("Title");
  });

  it("Escape cancels and inserts nothing", async () => {
    const html = setup();
    paste(getEditable(), { "text/plain": "# Title" });
    await screen.findByText(MD_TITLE);
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    await waitFor(() => expect(screen.queryByText(MD_TITLE)).not.toBeInTheDocument());
    expect(html()).not.toContain("Title");
  });

  it("Remove Formatting keeps the Markdown symbols as typed", async () => {
    const html = setup();
    paste(getEditable(), { "text/plain": "# Title\n\n**bold** text" });
    fireEvent.click(await screen.findByRole("button", { name: "Remove Formatting" }));
    await waitFor(() => expect(html()).toContain("# Title"));
    expect(html()).toContain("**bold** text");
    expect(html()).not.toMatch(/<h1|<strong|<b>/);
  });

  it("Remove Formatting follows the line-break rules in an enterInsertsLineBreak editor", async () => {
    const html = setup({ enterInsertsLineBreak: true });
    paste(getEditable(), { "text/plain": "# Title\n\n**bold**" });
    fireEvent.click(await screen.findByRole("button", { name: "Remove Formatting" }));
    await waitFor(() => expect(html()).toBe("<p># Title<br><br>**bold**</p>"));
  });
});

describe("Editor markdownPaste: what Paste as Markdown produces", () => {
  it("headings h1-h6", async () => {
    const out = await pasteAsMarkdown("# One\n## Two\n###### Six");
    expect(out).toMatch(/<h1[^>]*>.*One.*<\/h1>/);
    expect(out).toMatch(/<h2[^>]*>.*Two.*<\/h2>/);
    expect(out).toMatch(/<h6[^>]*>.*Six.*<\/h6>/);
  });

  it("inline formatting: bold, italic, strike, inline code", async () => {
    const out = await pasteAsMarkdown("**b** and *i* and ~~s~~ and `c`");
    expect(out).toContain('<strong class="editor-text-bold">b</strong>');
    expect(out).toContain('<em class="editor-text-italic">i</em>');
    expect(out).toContain("editor-text-strikethrough");
    expect(out).toContain('<span class="editor-text-code">c</span>');
  });

  it("links and bare URLs become links", async () => {
    const out = await pasteAsMarkdown("[docs](https://example.com/d) and https://example.org/x");
    expect(out).toContain('<a href="https://example.com/d" class="editor-link">docs</a>');
    expect(out).toContain('<a href="https://example.org/x" class="editor-link">https://example.org/x</a>');
  });

  it("nested and ordered lists, keeping the start number", async () => {
    const out = await pasteAsMarkdown("- a\n  - nested\n- b\n\n3. three\n4. four");
    expect(out).toMatch(/<ul[^>]*><li[^>]*>a<\/li><li[^>]*><ul[^>]*><li[^>]*>nested<\/li><\/ul><\/li><li[^>]*>b<\/li><\/ul>/);
    expect(out).toMatch(/<ol start="3"[^>]*><li[^>]*>three<\/li><li[^>]*>four<\/li><\/ol>/);
  });

  it("a loose list stays a list of plain items", async () => {
    const out = await pasteAsMarkdown("- a\n\n- b");
    expect(out).toMatch(/<ul[^>]*><li[^>]*>a<\/li><li[^>]*>b<\/li><\/ul>/);
    expect(out).not.toContain("<p>");
  });

  it("a fenced code block with a language is one code block", async () => {
    const out = await pasteAsMarkdown("```js\nconst a = 1;\nconst b = 2;\n```");
    expect(out).toMatch(/<pre[^>]*data-language="js"[^>]*>const a = 1;<br>const b = 2;/);
    expect((out.match(/<pre/g) ?? []).length).toBe(1);
  });

  it("an image inside a link gives one link reading the alt text", async () => {
    const out = await pasteAsMarkdown("[![alt](https://i.com/a.png)](https://x.com) **y**");
    expect(out).toContain('<a href="https://x.com" class="editor-link">alt</a>');
    expect(out).not.toContain("i.com/a.png");
  });

  it("a fenced code block has no trailing empty line", async () => {
    const out = await pasteAsMarkdown("```js\nconst a = 1;\n```");
    expect(out).toMatch(/<pre[^>]*>const a = 1;<\/pre>/);
  });

  it("javascript: links and raw HTML in a fence stay inert", async () => {
    const out = await pasteAsMarkdown(
      "[x](javascript:alert(1)) **b**\n\n```html\n<script>alert(1)</script>\n```",
    );
    expect(out).not.toMatch(/href="javascript/i);
    expect(out).not.toContain("<script");
  });

  it("a blockquote holds its text directly (no nested paragraph)", async () => {
    const out = await pasteAsMarkdown("> quoted\n> more");
    expect(out).toBe("<blockquote>quoted<br>more</blockquote>");
  });

  it("a horizontal rule is dropped and an image becomes a link", async () => {
    const out = await pasteAsMarkdown("before\n\n---\n\n![alt text](https://example.com/a.png) **x**");
    expect(out).not.toContain("<hr");
    expect(out).not.toContain("<img");
    expect(out).toContain('<a href="https://example.com/a.png" class="editor-link">alt text</a>');
    expect(out).toContain("before");
  });

  it("a GFM table becomes a table with header cells", async () => {
    const out = await pasteAsMarkdown("| a | b |\n|---|---|\n| 1 | 2 |");
    expect(out).toBe(
      "<table><tbody><tr><th><p>a</p></th><th><p>b</p></th></tr><tr><td><p>1</p></td><td><p>2</p></td></tr></tbody></table>",
    );
  });

  it("task-list syntax stays as literal [ ] / [x] text", async () => {
    const out = await pasteAsMarkdown("- [ ] todo\n- [x] done");
    expect(out).toContain("[ ] todo");
    expect(out).toContain("[x] done");
  });
});

describe("Editor markdownPaste: paragraphs in enterInsertsLineBreak mode", () => {
  it("merges the converted paragraphs into one joined by <br><br>", async () => {
    const out = await pasteAsMarkdown("Hi **team**,\n\nLine A\nLine B", { enterInsertsLineBreak: true });
    expect(out).toBe('<p>Hi <b><strong class="editor-text-bold">team</strong></b>,<br><br>Line A<br>Line B</p>');
  });

  it("keeps lists, headings and code as separate blocks", async () => {
    const out = await pasteAsMarkdown("Intro\n\n## Head\n\n- a\n- b\n\nOutro", { enterInsertsLineBreak: true });
    expect(out).toMatch(/^<p>Intro<\/p><h2[^>]*>.*Head.*<\/h2><ul/);
    expect(out).toMatch(/<\/ul><p>Outro<\/p>$/);
  });

  it("without the flag the paragraphs stay separate", async () => {
    const out = await pasteAsMarkdown("Hi **x**,\n\nLine A");
    expect(out).toBe('<p>Hi <b><strong class="editor-text-bold">x</strong></b>,</p><p>Line A</p>');
  });
});

describe("Editor markdownPaste: undo", () => {
  it("is one undo step", async () => {
    const html = setup({}, "<p>hello world</p>");
    act(() => {
      getLexical().update(
        () => {
          const text = $getRoot().getFirstDescendant();
          if (!text || !$isTextNode(text)) throw new Error("no text");
          text.select(5, 5);
        },
        { discrete: true },
      );
    });
    paste(getEditable(), { "text/plain": "# H\n\n- a\n- b\n\n**x**" });
    fireEvent.click(await screen.findByRole("button", { name: "Paste as Markdown" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    act(() => {
      getLexical().dispatchCommand(UNDO_COMMAND, undefined);
    });
    await waitFor(() => expect(html()).toBe("<p>hello world</p>"));
  });
});
