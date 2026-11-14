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

// Paste combinations: plain text, rendered HTML (Keep / Remove Formatting) and
// images, in the reply box (enterInsertsLineBreak + markdownPaste + uploader) and
// in an ordinary editor (no flags), across the places a caret can be, plus how the
// produced HTML survives a draft restore and the comment-body pipeline.
//
// Tests named "..." assert the CORRECT behaviour of a pre-existing bug
// that is not fixed yet; they are it.fails, so the suite is green and turning one into a
// plain test is the signal that it was fixed.

import { render, screen, fireEvent, act, cleanup } from "@testing-library/react";
import { describe, expect, it, vi, beforeAll } from "vitest";
import "@testing-library/jest-dom/vitest";
import {
  $getRoot,
  $isElementNode,
  $isTextNode,
  type LexicalEditor,
  type LexicalNode,
} from "lexical";
import Editor from "./Editor";
import { hasCommentContent, preprocessCommentBodyHtml, trimTrailingLineBreaks } from "../../features/csm-cases/utils/commentContent";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { htmlToPdfPlainText } from "@utils/pdfReportKit";

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

type Cfg = "reply" | "normal";
type Choice = "keep" | "remove";
const CONFIGS: Cfg[] = ["reply", "normal"];

const getEditable = () => screen.getByTestId("case-description-editor");
const getLexical = () =>
  (getEditable() as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;

const png = (name = "shot.png", type = "image/png") => new File([new Uint8Array(4)], name, { type });

/** Caret at the end of the first text node under the first block of `type`. */
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

/** Selects characters [from, to) of the first text node, to be replaced by a paste. */
function selectRange(from: number, to: number): void {
  act(() => {
    getLexical().update(
      () => {
        const text = $getRoot().getFirstDescendant();
        if (!text || !$isTextNode(text)) throw new Error("no text");
        text.select(from, to);
      },
      { discrete: true },
    );
  });
}

interface Mounted {
  onChange: ReturnType<typeof vi.fn>;
  onImageUpload: ReturnType<typeof vi.fn>;
  onPasteError: ReturnType<typeof vi.fn>;
  html: () => string;
}

/**
 * `caret`: "root" is Chrome's state in an empty editor (collapsed element selection
 * on the root, (0, 0)); a node type puts the caret at the end of its text; "none"
 * leaves the editor's own selection alone; "range:a-b" selects characters a..b.
 */
function mount(cfg: Cfg, opts: { value?: string; caret?: string; uploader?: boolean } = {}): Mounted {
  const onChange = vi.fn();
  const onPasteError = vi.fn();
  let n = 0;
  const onImageUpload = vi.fn(() => `blob:https://csm.example.com/${(n += 1)}`);
  const upload = opts.uploader ?? cfg === "reply";
  const flags = cfg === "reply" ? { enterInsertsLineBreak: true, markdownPaste: true } : {};
  render(
    <Editor
      autoFocus
      showToolbar={false}
      toolbarVariant="full"
      value={opts.value}
      onChange={onChange}
      onPasteError={onPasteError}
      onImageUpload={upload ? onImageUpload : undefined}
      {...flags}
    />,
  );
  const caret = opts.caret ?? "root";
  if (caret === "root") {
    act(() => {
      getLexical().update(() => { $getRoot().select(0, 0); }, { discrete: true });
    });
  } else if (caret.startsWith("range:")) {
    const [a, b] = caret.slice(6).split("-").map(Number);
    selectRange(a, b);
  } else if (caret !== "none") {
    caretIn(caret);
  }
  return {
    onChange,
    onImageUpload,
    onPasteError,
    html: () => (onChange.mock.calls.at(-1)?.[0] as string | undefined) ?? "",
  };
}

interface Clip {
  "text/plain"?: string;
  "text/html"?: string;
}

function firePaste(data: Clip, files: File[] = []): ClipboardEvent {
  const types = [
    ...(data["text/html"] ? ["text/html"] : []),
    ...(data["text/plain"] ? ["text/plain"] : []),
  ];
  const clipboardData = {
    getData: (type: string) => (data as Record<string, string>)[type] ?? "",
    items: [
      ...types.map((type) => ({ kind: "string", type, getAsFile: () => null })),
      ...files.map((f) => ({ kind: "file", type: f.type, getAsFile: () => f })),
    ],
  } as unknown as DataTransfer;
  const event = new ClipboardEvent("paste", {
    bubbles: true,
    cancelable: true,
    clipboardData,
  } as ClipboardEventInit & { clipboardData: DataTransfer });
  act(() => {
    getEditable().dispatchEvent(event);
  });
  return event;
}

const settle = () => act(() => new Promise<void>((r) => setTimeout(r, 20)));
const promptButtons = () =>
  screen
    .queryAllByRole("button")
    .map((b) => b.textContent ?? "")
    .filter((t) => /Formatting|Markdown/.test(t));

/**
 * Pastes and, when the format prompt appears, answers it ("keep" = the
 * Keep Formatting / Paste as Markdown button, "remove" = Remove Formatting).
 * Returns the buttons that were offered ([] = no prompt).
 */
async function pasteAnd(m: Mounted, data: Clip, choice: Choice = "keep", files: File[] = []): Promise<string[]> {
  firePaste(data, files);
  const buttons = promptButtons();
  if (buttons.length > 0) {
    const name = choice === "remove" ? "Remove Formatting" : buttons.find((b) => b !== "Remove Formatting")!;
    fireEvent.click(screen.getByRole("button", { name }));
  }
  await settle();
  void m;
  return buttons;
}

const inlineText = (html: string) => html.replace(/<[^>]*>/g, "");

// --- Realistic clipboard fragments -----------------------------------------------------

const WORD_LIST =
  '<html xmlns:o="urn:schemas-microsoft-com:office:office"><body><!--StartFragment-->' +
  '<p class=MsoListParagraph style="text-indent:-.25in;mso-list:l0 level1 lfo1"><![if !supportLists]><span style="mso-list:Ignore">&middot;<span style="font:7.0pt Times">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; </span></span><![endif]>First<o:p></o:p></p>' +
  '<p class=MsoListParagraph style="margin-left:.5in;text-indent:-.25in;mso-list:l0 level2 lfo1"><span style="mso-list:Ignore">o&nbsp;</span>Second<o:p></o:p></p>' +
  "<p class=MsoNormal>Para <b>bold</b><o:p></o:p></p><!--EndFragment--></body></html>";
const WORD_LIST_TEXT = "·       First\no   Second\nPara bold";

const GDOCS =
  '<meta charset="utf-8"><b style="font-weight:normal;" id="docs-internal-guid-abc"><ul style="margin-top:0;margin-bottom:0;">' +
  '<li dir="ltr" style="list-style-type:disc;margin-left:36pt" aria-level="1"><p dir="ltr" role="presentation"><span style="font-size:11pt">One</span></p></li>' +
  '<ul style="margin-top:0;margin-bottom:0;"><li dir="ltr" style="list-style-type:circle" aria-level="2"><p dir="ltr" role="presentation"><span>Two</span></p></li></ul>' +
  '<li dir="ltr" aria-level="1"><p dir="ltr"><span>Three</span></p></li></ul><br>' +
  '<p dir="ltr" style="margin-left:36pt"><span>indented para</span></p></b>';
const GDOCS_TEXT = "One\nTwo\nThree\n\nindented para";

const GMAIL =
  '<div dir="ltr">Hi<div><br></div><div>Line two</div><div><br></div><blockquote class="gmail_quote" style="margin:0 0 0 .8ex"><div>quoted a</div><div>quoted b</div></blockquote></div>';
const GMAIL_TEXT = "Hi\n\nLine two\n\nquoted a\nquoted b";

const WEB_PAGE =
  '<h1>Title</h1><p>Intro with <code>inline</code> and <a href="https://x.com/a">link</a>.</p>' +
  "<ul><li>A<ol><li>A1<ul><li>A1a</li><li>A1b</li></ul></li><li>A2</li></ol></li><li>B</li></ul>" +
  '<pre><code class="language-js">const a = 1;\n  if (a) {\n    b();\n  }\n</code></pre>' +
  "<table><thead><tr><th>H1</th><th>H2</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>";
const WEB_PAGE_TEXT = "Title\nIntro with inline and link.\nA\nA1\nA1a\nA1b\nA2\nB\nconst a = 1;\n  if (a) {\n    b();\n  }\nH1\tH2\n1\t2";

const VSCODE =
  '<meta charset="utf-8"><div style="color:#ccc;background-color:#1f1f1f;font-family:Menlo;font-weight:normal;font-size:12px;line-height:18px;white-space:pre">' +
  '<div><span style="color:#ccc">def f():</span></div><div><span style="color:#ccc">    return 1</span></div></div>';
const VSCODE_TEXT = "def f():\n    return 1";

const EXCEL = "<table><tr><td>Name</td><td>Age</td></tr><tr><td>Bob</td><td>30</td></tr></table>";
const EXCEL_TEXT = "Name\tAge\nBob\t30";

const CHATGPT =
  "<p>Here:</p><ol><li><p><strong>Step</strong>: do it</p></li><li><p>Next</p></li></ol><p>Done.</p>";
const CHATGPT_TEXT = "Here:\n\n1. Step: do it\n\n2. Next\n\nDone.";

describe("plain-text paste: whitespace, newlines and carets", () => {
  it.each(CONFIGS)("[%s] keeps runs of spaces, a leading space run and inline tabs", async (cfg) => {
    const m = mount(cfg);
    await pasteAnd(m, { "text/plain": "  lead  two   spaces\ttab" });
    expect(m.html()).toBe("<p>  lead  two   spaces\ttab</p>");
  });

  it("[reply] a copied spreadsheet row/rows keeps its tabs and has no prompt", async () => {
    const m = mount("reply");
    const offered = await pasteAnd(m, { "text/plain": "Name\tAge\tCity\nBob\t30\tNYC" });
    expect(offered).toEqual([]);
    expect(m.html()).toBe("<p>Name\tAge\tCity<br>Bob\t30\tNYC</p>");
  });

  it("[normal] a copied spreadsheet row/rows is one paragraph per row with tabs", async () => {
    const m = mount("normal");
    await pasteAnd(m, { "text/plain": "Name\tAge\tCity\nBob\t30\tNYC" });
    expect(m.html()).toBe("<p>Name\tAge\tCity</p><p>Bob\t30\tNYC</p>");
  });

  // Pre-existing (identical before bc2bc13c4): a paste that STARTS with a tab, with the
  // caret on the root as in an empty editor, leaves an empty paragraph after it in the
  // normal editor. The reply box is fine because it moves the caret into a block first.
  it("[reply] leading tabs on the very first line are kept and add no stray paragraph", async () => {
    const m = mount("reply");
    await pasteAnd(m, { "text/plain": "\t\tindented\n\tless" });
    expect(m.html().startsWith("<p>\t\tindented")).toBe(true);
    expect(m.html()).not.toContain("<p><br></p>");
  });

  it("[normal] leading tabs on the very first line are kept and add no stray paragraph", async () => {
    const m = mount("normal");
    await pasteAnd(m, { "text/plain": "\t\tindented\n\tless" });
    expect(m.html().startsWith("<p>\t\tindented")).toBe(true);
    expect(m.html()).not.toContain("<p><br></p>");
  });

  it("[reply] CRLF is the same as LF; 3+ newlines are one blank line", async () => {
    const a = mount("reply");
    await pasteAnd(a, { "text/plain": "a\r\nb\r\n\r\n\r\n\r\nc" });
    expect(a.html()).toBe("<p>a<br>b<br><br>c</p>");
  });

  it("[normal] CRLF and blank lines become paragraph breaks", async () => {
    const m = mount("normal");
    await pasteAnd(m, { "text/plain": "a\r\nb\r\n\r\n\r\nc" });
    expect(m.html()).toBe("<p>a</p><p>b</p><p>c</p>");
  });

  it("[reply] a trailing newline leaves a trailing <br> that the sender's trimTrailingLineBreaks removes", async () => {
    const m = mount("reply");
    await pasteAnd(m, { "text/plain": "hello\n\n" });
    expect(m.html()).toBe("<p>hello<br><br></p>");
    expect(trimTrailingLineBreaks(m.html())).toBe("<p>hello</p>");
  });

  it.each(CONFIGS)("[%s] a terminal log / stack trace keeps its indentation and gets no prompt", async (cfg) => {
    const m = mount(cfg);
    const log =
      "2026-01-01 12:00:00 INFO  start\n    at com.x.Y(Y.java:10)\n    at com.x.Z(Z.java:20)\nCaused by: boom";
    const offered = await pasteAnd(m, { "text/plain": log });
    expect(offered).toEqual([]);
    expect(inlineText(m.html().replace(/<br>|<\/p><p>/g, "\n"))).toBe(log);
  });

  it("[reply] an e-mail with quoted lines offers the Markdown prompt (a '>' line counts); Remove keeps '>' literal", async () => {
    const text = "On Mon, Bob wrote:\n> hello\n> there\n\nThanks";
    const m = mount("reply");
    const offered = await pasteAnd(m, { "text/plain": text }, "remove");
    expect(offered).toEqual(["Remove Formatting", "Paste as Markdown"]);
    expect(m.html()).toBe("<p>On Mon, Bob wrote:<br>&gt; hello<br>&gt; there<br><br>Thanks</p>");
    cleanup();
    const n = mount("normal");
    expect(await pasteAnd(n, { "text/plain": text })).toEqual([]);
  });

  it("[reply] a numbered checklist offers the Markdown prompt; Remove keeps it literal", async () => {
    const m = mount("reply");
    const offered = await pasteAnd(m, { "text/plain": "1. one\n2. two\n3. three" }, "remove");
    expect(offered).toEqual(["Remove Formatting", "Paste as Markdown"]);
    expect(m.html()).toBe("<p>1. one<br>2. two<br>3. three</p>");
  });

  it("[normal] a numbered checklist is plain paragraphs and never prompts", async () => {
    const m = mount("normal");
    expect(await pasteAnd(m, { "text/plain": "1. one\n2. two" })).toEqual([]);
    expect(m.html()).toBe("<p>1. one</p><p>2. two</p>");
  });

  describe.each(CONFIGS)("[%s] caret contexts", (cfg) => {
    const NL = cfg === "reply" ? "<br>" : "</p><p>";
    it("middle of a typed line", async () => {
      const m = mount(cfg, { value: "<p>abc def</p>", caret: "paragraph" });
      await pasteAnd(m, { "text/plain": "XY\nZ" });
      expect(m.html()).toBe(`<p>abc defXY${NL}Z</p>`);
    });
    it("after <br> lines", async () => {
      const m = mount(cfg, { value: "<p>a<br>b</p>", caret: "paragraph" });
      await pasteAnd(m, { "text/plain": "X\nY" });
      expect(m.html()).toBe(`<p>a<br>bX${NL}Y</p>`);
    });
    it("replacing a range", async () => {
      const m = mount(cfg, { value: "<p>abc def ghi</p>", caret: "range:4-7" });
      await pasteAnd(m, { "text/plain": "X\nY" });
      expect(m.html()).toBe(`<p>abc X${NL}Y ghi</p>`);
    });
    it("in a bullet item: each line is an item, in both configs", async () => {
      const m = mount(cfg, { value: "<ul><li>one</li></ul>", caret: "listitem" });
      await pasteAnd(m, { "text/plain": "X\nY" });
      expect(m.html()).toContain(">oneX</li>");
      expect((m.html().match(/<li/g) ?? []).length).toBe(2);
    });
    it("in a nested bullet item: stays at its own level", async () => {
      const m = mount(cfg, {
        value: "<ul><li>a<ul><li>b<ul><li>c</li></ul></li></ul></li></ul>",
        caret: "listitem",
      });
      // caretIn picks the FIRST list item, so move to the deepest one.
      act(() => {
        getLexical().update(
          () => {
            const last = $getRoot().getLastDescendant();
            if (last && $isTextNode(last)) last.select();
          },
          { discrete: true },
        );
      });
      await pasteAnd(m, { "text/plain": "X\nY" });
      const html = m.html();
      expect(html).toContain(">cX</li><li");
      expect(html).toContain(">Y</li>");
      expect(html.match(/<ul/g)?.length).toBe(3);
    });
    it("in a heading: the first line joins it, the rest is a paragraph", async () => {
      const m = mount(cfg, { value: "<h2>Title</h2>", caret: "heading" });
      await pasteAnd(m, { "text/plain": "X\nY" });
      expect(m.html()).toBe("<h2>TitleX</h2><p>Y</p>");
    });
    it("in a quote", async () => {
      const m = mount(cfg, { value: "<blockquote>q</blockquote>", caret: "quote" });
      await pasteAnd(m, { "text/plain": "X\nY" });
      expect(m.html()).toBe("<blockquote>qX</blockquote><p>Y</p>");
    });
    it("in a code block: lines and indentation stay inside the block, no prompt even for Markdown-looking text", async () => {
      const m = mount(cfg, { value: "<pre><code>c</code></pre>", caret: "code" });
      const offered = await pasteAnd(m, { "text/plain": "- a\n- b\n  - c\n\tZ" });
      expect(offered).toEqual([]);
      expect(m.html()).toContain("<br>  - c<br>\tZ</pre>");
      expect(m.html().match(/<pre/g)?.length).toBe(1);
    });
    it("an HTML-only table with empty edge cells keeps its tabs (code block, and Remove Formatting)", async () => {
      const clip = { "text/html": "<table><tr><td></td><td>B</td><td></td></tr><tr><td>C</td><td></td><td></td></tr></table>" };
      const code = mount(cfg, { value: "<pre><code>c</code></pre>", caret: "code" });
      await pasteAnd(code, clip);
      expect(code.html()).toContain("</code>\tB\t<br>C\t\t</pre>");
      cleanup();
      const plain = mount(cfg, { value: "<p>x</p>", caret: "paragraph" });
      await pasteAnd(plain, clip, "remove");
      expect(plain.html()).toContain("x\tB\t");
    });
    it("in a table cell: the first line joins the cell paragraph, never a <br>", async () => {
      const m = mount(cfg, {
        value: "<table><tbody><tr><td><p>x</p></td></tr></tbody></table>",
        caret: "tablecell",
      });
      await pasteAnd(m, { "text/plain": "X\nY" });
      expect(m.html()).toBe("<table><tbody><tr><td><p>xX</p><p>Y</p></td></tr></tbody></table>");
    });
  });
});

describe("rendered HTML paste: Keep Formatting / Remove Formatting", () => {
  describe.each(CONFIGS)("[%s]", (cfg) => {
    const NL = cfg === "reply" ? "<br>" : "</p><p>";

    it("Google Docs: nested bullets keep their levels", async () => {
      const m = mount(cfg);
      expect(await pasteAnd(m, { "text/html": GDOCS, "text/plain": GDOCS_TEXT })).toEqual([
        "Remove Formatting",
        "Keep Formatting",
      ]);
      const html = m.html();
      expect(html).toMatch(/^<ul[^>]*><li[^>]*>One<\/li><li[^>]*><ul[^>]*><li[^>]*>Two<\/li><\/ul><\/li><li[^>]*>Three<\/li><\/ul>/);
      expect(inlineText(html)).toContain("indented para");
    });

    it("Google Docs: Remove Formatting is plain lines (blank line preserved in the reply box only)", async () => {
      const m = mount(cfg);
      await pasteAnd(m, { "text/html": GDOCS, "text/plain": GDOCS_TEXT }, "remove");
      expect(m.html()).toBe(
        cfg === "reply"
          ? "<p>One<br>Two<br>Three<br><br>indented para</p>"
          : "<p>One</p><p>Two</p><p>Three</p><p>indented para</p>",
      );
    });

    it("Gmail: Keep keeps the quote as a blockquote and collapses the blank divs", async () => {
      const m = mount(cfg);
      await pasteAnd(m, { "text/html": GMAIL, "text/plain": GMAIL_TEXT });
      expect(m.html()).toBe("<p>Hi</p><p>Line two</p><blockquote>quoted a<br>quoted b</blockquote>");
    });

    it("web page: headings, 3 mixed list levels, inline code, link, code block and table all survive", async () => {
      const m = mount(cfg);
      await pasteAnd(m, { "text/html": WEB_PAGE, "text/plain": WEB_PAGE_TEXT });
      const html = m.html();
      expect(html).toContain("<h1>Title</h1>");
      expect(html).toMatch(/<ul[^>]*><li[^>]*>A<\/li><li[^>]*><ol[^>]*><li[^>]*>A1<\/li><li[^>]*><ul[^>]*><li[^>]*>A1a<\/li><li[^>]*>A1b<\/li><\/ul><\/li><li[^>]*>A2<\/li><\/ol><\/li><li[^>]*>B<\/li><\/ul>/);
      expect(html).toContain('<a href="https://x.com/a"');
      expect(html).toMatch(/<code[^>]*><span[^>]*>inline<\/span><\/code>/);
      // One code block (not nested), indentation inside it kept.
      expect(html.match(/<pre/g)?.length).toBe(1);
      expect(html).toContain("const a = 1;<br>  if (a) {<br>    b();<br>  }");
      expect(html).toMatch(/<th><p>H1<\/p><\/th><th><p>H2<\/p><\/th>/);
    });

    it("web page: Remove Formatting is the plain-text rendering with the tab between table cells", async () => {
      const m = mount(cfg);
      await pasteAnd(m, { "text/html": WEB_PAGE, "text/plain": WEB_PAGE_TEXT }, "remove");
      expect(m.html()).toContain(`  if (a) {${NL}    b();${NL}  }${NL}H1\tH2${NL}1\t2`);
    });

    it("VS Code flat HTML whose text is not Markdown gets the normal Keep/Remove prompt", async () => {
      const m = mount(cfg);
      expect(await pasteAnd(m, { "text/html": VSCODE, "text/plain": VSCODE_TEXT })).toEqual([
        "Remove Formatting",
        "Keep Formatting",
      ]);
      expect(m.html()).toBe("<p>def f():</p><p>    return 1</p>");
    });

    it("VS Code: Remove Formatting keeps the code's indentation", async () => {
      const m = mount(cfg);
      await pasteAnd(m, { "text/html": VSCODE, "text/plain": VSCODE_TEXT }, "remove");
      expect(m.html()).toBe(`<p>def f():${NL}    return 1</p>`);
    });

    it("Excel/Sheets: Keep gives a table, Remove gives tab-separated lines", async () => {
      const k = mount(cfg);
      await pasteAnd(k, { "text/html": EXCEL, "text/plain": EXCEL_TEXT });
      expect(k.html()).toBe(
        "<table><tbody><tr><td><p>Name</p></td><td><p>Age</p></td></tr><tr><td><p>Bob</p></td><td><p>30</p></td></tr></tbody></table>",
      );
    });

    it("ChatGPT rendered answer: Keep gives an ordered list; the prompt is the normal Keep/Remove one", async () => {
      const m = mount(cfg);
      expect(await pasteAnd(m, { "text/html": CHATGPT, "text/plain": CHATGPT_TEXT })).toEqual([
        "Remove Formatting",
        "Keep Formatting",
      ]);
      expect(m.html()).toMatch(/^<p>Here:<\/p><ol[^>]*><li[^>]*><b><strong[^>]*>Step<\/strong><\/b>: do it<\/li><li[^>]*>Next<\/li><\/ol><p>Done\.<\/p>$/);
    });

    it("a <br> inside a list item stays one item with a break; empty paragraphs between paragraphs collapse", async () => {
      const a = mount(cfg);
      await pasteAnd(a, { "text/html": "<ul><li>a<br>b</li><li>c</li></ul>", "text/plain": "a\nb\nc" });
      expect(a.html()).toMatch(/^<ul[^>]*><li[^>]*>a<br>b<\/li><li[^>]*>c<\/li><\/ul>$/);
      cleanup();
      const b = mount(cfg);
      await pasteAnd(b, {
        "text/html": "<p>a</p><p></p><p>&nbsp;</p><p><br></p><p>b</p>",
        "text/plain": "a\n\n\nb",
      });
      expect(b.html()).toBe(cfg === "reply" ? "<p>a<br><br>b</p>" : "<p>a</p><p>b</p>");
    });

    it("&nbsp; runs and white-space:pre blocks keep their spaces", async () => {
      const m = mount(cfg);
      await pasteAnd(m, {
        "text/html": '<div style="white-space:pre">  a\n    b</div><p>x&nbsp;&nbsp;&nbsp;y</p>',
        "text/plain": "  a\n    b\nx   y",
      });
      expect(m.html()).toBe("<p>  a<br>    b</p><p>x&nbsp;&nbsp;&nbsp;y</p>");
    });

    it("Jira/Confluence: heading, link in a span and a <pre> panel", async () => {
      const m = mount(cfg);
      await pasteAnd(m, {
        "text/html":
          '<div class="user-content-block"><h3 id="x">Heading</h3><p>text <span class="nobr"><a href="https://j.com/browse/A-1">A-1</a></span></p><div class="code panel"><div class="codeContent panelContent"><pre class="code-java">line1\n  line2</pre></div></div></div>',
        "text/plain": "Heading\ntext A-1\nline1\n  line2",
      });
      const html = m.html();
      expect(html).toContain("<h3>Heading</h3>");
      expect(html).toContain('<a href="https://j.com/browse/A-1"');
      expect(html).toContain("line1<br>  line2</pre>");
    });

    it("rendered HTML that holds a numbered list keeps the Keep/Remove prompt (never the Markdown one)", async () => {
      const m = mount(cfg);
      const offered = await pasteAnd(m, { "text/html": "<ol><li>a</li><li>b</li></ol>", "text/plain": "1. a\n2. b" });
      expect(offered).toEqual(["Remove Formatting", "Keep Formatting"]);
    });

    it("Keep Formatting in a list item: nested caret level is respected", async () => {
      const m = mount(cfg, { value: "<ul><li>a<ul><li>b</li></ul></li></ul>", caret: "listitem" });
      await pasteAnd(m, { "text/html": "<p>x <b>y</b></p>", "text/plain": "x y" });
      expect(m.html()).toMatch(/<li[^>]*>ax <b><strong[^>]*>y<\/strong><\/b><\/li><li[^>]*><ul[^>]*><li[^>]*>b<\/li><\/ul><\/li>/);
    });

    it("Keep Formatting in a heading and in a table cell stays inside that block", async () => {
      const h = mount(cfg, { value: "<h2>Title</h2>", caret: "heading" });
      await pasteAnd(h, { "text/html": "<p>x <b>y</b></p>", "text/plain": "x y" });
      expect(h.html()).toMatch(/^<h2>Titlex <b><strong[^>]*>y<\/strong><\/b><\/h2>$/);
      cleanup();
      const c = mount(cfg, {
        value: "<table><tbody><tr><td><p>q</p></td></tr></tbody></table>",
        caret: "tablecell",
      });
      await pasteAnd(c, { "text/html": "<p>x</p><p>y</p>", "text/plain": "x\ny" });
      expect(c.html()).toBe("<table><tbody><tr><td><p>qx</p><p>y</p></td></tr></tbody></table>");
    });
  });
});

const imgs = () => Array.from(getEditable().querySelectorAll("img"));

describe("image paste", () => {
  it("[reply] an image file alone is uploaded and inserted once", async () => {
    const m = mount("reply");
    await pasteAnd(m, {}, "keep", [png()]);
    expect(m.onImageUpload).toHaveBeenCalledTimes(1);
    expect(imgs()).toHaveLength(1);
    expect(m.html().match(/<img/g)?.length).toBe(1);
  });

  it("[reply] an image file plus its <img src=https> HTML inserts once, no prompt, no hot-link", async () => {
    const m = mount("reply");
    const offered = await pasteAnd(
      m,
      { "text/html": '<img src="https://example.com/p.png" alt="p">' },
      "keep",
      [png()],
    );
    expect(offered).toEqual([]);
    expect(m.onImageUpload).toHaveBeenCalledTimes(1);
    expect(imgs()).toHaveLength(1);
    expect(m.html()).not.toContain("https://example.com/p.png");
  });

  it("[reply] an image file with caption text HTML: Keep Formatting keeps the caption and adds no link for the picture the file already stores", async () => {
    const m = mount("reply");
    await pasteAnd(
      m,
      { "text/html": '<img src="https://example.com/p.png" alt="p"><p>caption</p>', "text/plain": "caption" },
      "keep",
      [png()],
    );
    expect(m.onImageUpload).toHaveBeenCalledTimes(1);
    expect(m.html()).not.toContain("https://example.com/p.png");
    expect(inlineText(m.html())).toContain("caption");
  });

  it("[reply] an image file with caption text HTML still prompts; Remove keeps only the uploaded copy", async () => {
    // Record only: a copied web page image with a caption has text, so isImageOnlyHtml is false.
    const m = mount("reply");
    const offered = await pasteAnd(
      m,
      { "text/html": '<img src="https://example.com/p.png"><p>caption</p>', "text/plain": "caption" },
      "remove",
      [png()],
    );
    expect(offered).toEqual(["Remove Formatting", "Keep Formatting"]);
    expect(m.onImageUpload).toHaveBeenCalledTimes(1);
    expect(m.html()).not.toContain("https://example.com/p.png");
  });

  it.each(["keep", "remove"] as Choice[])(
    "[reply/%s] copying cells from Excel (png of the range + table html) inserts one representation, not an image AND a table",
    async (choice) => {
      // Excel/Sheets-on-desktop put an image/png rendering of the selected cells next to
      // text/html + text/plain. ClipboardImagePlugin takes the picture unconditionally and
      // the HTML still goes through the prompt, so the same cells arrive twice.
      // Pre-existing (ClipboardImagePlugin is untouched by the two commits).
      const m = mount("reply");
      await pasteAnd(m, { "text/html": EXCEL, "text/plain": EXCEL_TEXT }, choice, [png("cells.png")]);
      const hasImage = imgs().length > 0;
      const hasText = inlineText(m.html()).includes("Bob");
      expect(hasImage && hasText).toBe(false);
    },
  );

  it("[reply] a wrong-type image is refused with 'type', nothing is uploaded or inserted", async () => {
    const m = mount("reply");
    await pasteAnd(m, {}, "keep", [png("a.tiff", "image/tiff")]);
    expect(m.onPasteError).toHaveBeenCalledWith("type");
    expect(m.onImageUpload).not.toHaveBeenCalled();
    expect(imgs()).toHaveLength(0);
  });

  it("[reply] an over-size image whose HTML is only an <img> is refused once and the HTML is not pasted", async () => {
    const big = png("big.png");
    Object.defineProperty(big, "size", { value: 50 * 1024 * 1024 });
    const m = mount("reply");
    const offered = await pasteAnd(m, { "text/html": '<img src="https://example.com/p.png">' }, "keep", [big]);
    expect(offered).toEqual([]);
    expect(m.onPasteError).toHaveBeenCalledTimes(1);
    expect(m.onPasteError).toHaveBeenCalledWith("size");
    expect(imgs()).toHaveLength(0);
  });

  it.each(CONFIGS)("[%s] without an uploader an image file is refused ('disabled') and its <img> HTML is not pasted", async (cfg) => {
    const m = mount(cfg, { uploader: false });
    const offered = await pasteAnd(m, { "text/html": '<img src="https://example.com/p.png">' }, "keep", [png()]);
    expect(offered).toEqual([]);
    expect(m.onPasteError).toHaveBeenCalledWith("disabled");
    expect(imgs()).toHaveLength(0);
  });

  it.each(CONFIGS)("[%s] an image file alone without an uploader inserts nothing", async (cfg) => {
    const m = mount(cfg, { uploader: false });
    await pasteAnd(m, {}, "keep", [png()]);
    expect(m.onPasteError).toHaveBeenCalledWith("disabled");
    expect(imgs()).toHaveLength(0);
  });

  describe.each(CONFIGS)("[%s] a remote <img src=https> inside pasted text HTML", (cfg) => {
    const HTML = '<p>See <img src="https://example.com/a.png" alt="a"> here</p>';
    it("Keep Formatting does not hot-link a new remote image: it becomes a link with the alt text", async () => {
      // CLAUDE.md "Inline image limits": nothing is embedded or hot-linked (same as the Markdown path).
      const m = mount(cfg);
      await pasteAnd(m, { "text/html": HTML, "text/plain": "See  here" }, "keep");
      expect(m.html()).not.toMatch(/<img/);
      expect(m.html()).toMatch(/<p>See <a href="https:\/\/example.com\/a.png"[^>]*>a<\/a> here<\/p>/);
    });
    it("Keep Formatting: an image inside a link keeps the one link; data: and unsafe sources become the alt text", async () => {
      const m = mount(cfg);
      await pasteAnd(
        m,
        {
          "text/html":
            '<p><a href="https://x.com"><img src="https://i.com/b.png" alt="badge"></a> <img src="data:image/png;base64,AAAA" alt="inline"> <img src="javascript:alert(1)" alt="bad"> <img src="https://i.com/noalt.png"></p>',
          "text/plain": "badge inline bad",
        },
        "keep",
      );
      const html = m.html();
      expect(html).not.toMatch(/<img/);
      expect(html).not.toContain("data:image");
      expect(html).not.toContain("javascript:");
      expect(html).toMatch(/<a href="https:\/\/x.com\/?"[^>]*>badge<\/a>/);
      expect(html).toContain("inline");
      expect(html).toMatch(/<a href="https:\/\/i.com\/noalt.png"[^>]*>https:\/\/i.com\/noalt.png<\/a>/);
    });
    it("Remove Formatting drops the image and keeps the text", async () => {
      const m = mount(cfg);
      await pasteAnd(m, { "text/html": HTML, "text/plain": "See  here" }, "remove");
      expect(imgs()).toHaveLength(0);
      expect(inlineText(m.html())).toBe("See  here");
    });
  });
});

// --- Contexts that expose pre-existing nesting / import gaps ------------------------------

describe("pasted HTML structure", () => {
  it.each(CONFIGS)("[%s] Keep Formatting into a code block never puts a <p> inside the <pre>", async (cfg) => {
    // Pre-existing (identical before the two commits): the pasted paragraph becomes a
    // ParagraphNode child of the CodeNode, exported as <pre><code>..</code><p>..</p></pre>.
    const m = mount(cfg, { value: "<pre><code>c</code></pre>", caret: "code" });
    await pasteAnd(m, { "text/html": "<p>one <b>x</b></p>", "text/plain": "one x" }, "keep");
    expect(m.html()).not.toMatch(/<pre[^>]*>[\s\S]*<p[ >]/);
  });

  it.each(CONFIGS)("[%s] a Word bulleted list (MsoListParagraph) pastes as a list, not as '·' paragraphs", async (cfg) => {
    // Pre-existing: mso-list is not understood, so the bullet glyph and its nbsp run are
    // kept as text and the level-2 indent is lost. Word is a main source of pasted lists.
    const m = mount(cfg);
    await pasteAnd(m, { "text/html": WORD_LIST, "text/plain": WORD_LIST_TEXT }, "keep");
    expect(m.html()).toMatch(/<ul[^>]*><li[^>]*>First<\/li>/);
    expect(m.html()).not.toContain("·");
  });

  it.each(CONFIGS)("[%s] Keep Formatting drops margin-left / padding-left / text-indent (no indent node in this editor)", async (cfg) => {
    // Product decision, not asserted as a bug: CSS indentation on a paragraph is lost
    // (Google Docs "indented para", Word first-line indents). Recorded so a change is noticed.
    const m = mount(cfg);
    await pasteAnd(m, {
      "text/html": '<p style="margin-left:40px">m</p><p style="padding-left:30px">p</p><p style="text-indent:2em">t</p>',
      "text/plain": "m\np\nt",
    });
    // Paragraphs with a style stay separate blocks in the reply box too (only unstyled ones merge).
    expect(m.html()).toBe("<p>m</p><p>p</p><p>t</p>");
  });

  it.each(CONFIGS)("[%s] Google Docs' blank <br> between a list and a paragraph does not leave an empty paragraph", async (cfg) => {
    // Pre-existing: collapseEmptyParagraphElements only removes empty <p>/<div>; a bare
    // <br> child of Docs' <b> wrapper becomes <p><br></p> between the list and the text.
    const m = mount(cfg);
    await pasteAnd(m, { "text/html": GDOCS, "text/plain": GDOCS_TEXT }, "keep");
    expect(m.html()).not.toContain("<p><br></p>");
  });

  it("Keep Formatting at the root caret of an empty ordinary editor leaves no trailing empty paragraph (changed by the commits: before, every Keep paste ended in <p><br></p>)", async () => {
    const m = mount("normal");
    await pasteAnd(m, { "text/html": "<p>a</p><p>b</p>", "text/plain": "a\nb" });
    expect(m.html()).toBe("<p>a</p><p>b</p>");
  });

  it("[reply] two Keep-Formatting paragraphs merge into one <p> joined by <br><br>, the same as Remove Formatting and a typed blank line", async () => {
    // Product decision: the same merge the Markdown path does (mergeAdjacentParagraphs).
    const clip = { "text/html": "<p>Hi</p><p>Line two</p>", "text/plain": "Hi\n\nLine two" };
    const k = mount("reply");
    await pasteAnd(k, clip, "keep");
    expect(k.html()).toBe("<p>Hi<br><br>Line two</p>");
    cleanup();
    const r = mount("reply");
    await pasteAnd(r, clip, "remove");
    expect(r.html()).toBe("<p>Hi<br><br>Line two</p>");
  });

  it("[reply] Keep Formatting does not reorder text that sits between two paragraphs", async () => {
    const k = mount("reply");
    await pasteAnd(k, { "text/html": "<p>A</p>middle<p>B</p>", "text/plain": "A\nmiddle\nB" }, "keep");
    const out = k.html();
    expect(out.indexOf("A")).toBeLessThan(out.indexOf("middle"));
    expect(out.indexOf("middle")).toBeLessThan(out.indexOf("B"));
  });

  it("[normal] two Keep-Formatting paragraphs stay two <p>", async () => {
    const k = mount("normal");
    await pasteAnd(k, { "text/html": "<p>Hi</p><p>Line two</p>", "text/plain": "Hi\n\nLine two" }, "keep");
    expect(k.html()).toBe("<p>Hi</p><p>Line two</p>");
  });

  it("[reply] Keep Formatting merges paragraphs only in a plain paragraph: lists, headings and a list item caret stay separate", async () => {
    const clip = { "text/html": "<h2>T</h2><p>a</p><p>b</p><ul><li>x</li></ul><p>c</p>", "text/plain": "T\na\nb\nx\nc" };
    const k = mount("reply");
    await pasteAnd(k, clip, "keep");
    expect(k.html()).toMatch(/^<h2[^>]*>T<\/h2><p>a<br><br>b<\/p><ul[^>]*><li[^>]*>x<\/li><\/ul><p>c<\/p>$/);
    cleanup();
    const inItem = mount("reply", { value: "<ul><li>item</li></ul>", caret: "listitem" });
    await pasteAnd(inItem, { "text/html": "<p>a</p><p>b</p>", "text/plain": "a\nb" }, "keep");
    expect(inItem.html()).toContain("<li");
    expect(inItem.html()).not.toContain("<br><br>");
  });
});

// --- Images copied inside the app stay images -------------------------------------------------

describe.each(CONFIGS)("[%s] Keep Formatting of an image copied out of an editor of this app", (cfg) => {
  const LEXICAL = JSON.stringify({ namespace: "x", nodes: [] });
  const IN_APP = (src: string) => `<p>a <img src="${src}" alt="pic" style="max-width: 100%"> b</p>`;

  /** Pastes `html` (optionally with the Lexical marker) into an empty editor and keeps it. */
  async function keep(html: string, fromApp: boolean, value?: string): Promise<{ m: Mounted }> {
    const m = mount(cfg, { value, caret: value ? "p" : "root", uploader: true });
    firePasteWith(html, fromApp);
    const keepButton = screen.getByRole("button", { name: "Keep Formatting" });
    fireEvent.click(keepButton);
    await settle();
    return { m };
  }

  function firePasteWith(html: string, fromApp: boolean): void {
    const data: Record<string, string> = { "text/html": html, "text/plain": "a pic b", ...(fromApp ? { "application/x-lexical-editor": LEXICAL } : {}) };
    const clipboardData = {
      getData: (type: string) => data[type] ?? "",
      items: [{ kind: "string", type: "text/html", getAsFile: () => null }],
    } as unknown as DataTransfer;
    const event = new ClipboardEvent("paste", { bubbles: true, cancelable: true, clipboardData } as ClipboardEventInit & {
      clipboardData: DataTransfer;
    });
    act(() => {
      getEditable().dispatchEvent(event);
    });
  }

  it("round trip: an ImageNode copied out of the editor pastes back as an image (blob: preview)", async () => {
    const source = mount(cfg, { value: '<p>x <img src="blob:https://csm.example.com/7" alt="shot"></p>', caret: "none" });
    await settle();
    const exported = source.html();
    expect(exported).toContain("<img");
    cleanup();
    const { m } = await keep(exported, true);
    expect(m.html()).toMatch(/<img[^>]*src="blob:https:\/\/csm.example.com\/7"[^>]*alt="shot"/);
  });

  it("a stored /<uuid> image, with or without the editor marker, stays an image", async () => {
    const src = "/0b9f3c1e-aaaa-4bbb-8ccc-1234567890ab";
    expect((await keep(IN_APP(src), true)).m.html()).toContain(`src="${src}"`);
    cleanup();
    expect((await keep(IN_APP(src), false)).m.html()).toContain(`src="${src}"`);
  });

  it("an older draft's data: image stays an image only when the clipboard is from an editor of this app", async () => {
    const src = "data:image/png;base64,AAAA";
    expect((await keep(IN_APP(src), true)).m.html()).toContain(`src="${src}"`);
    cleanup();
    const foreign = (await keep(IN_APP(src), false)).m.html();
    expect(foreign).not.toContain("<img");
    expect(foreign).not.toContain("data:image");
  });

  it("a remote image is a link even when the clipboard came from an editor of this app", async () => {
    const html = (await keep(IN_APP("https://example.com/p.png"), true)).m.html();
    expect(html).not.toContain("<img");
    expect(html).toContain('href="https://example.com/p.png"');
  });

  it.runIf(cfg === "reply")("copied images still respect the 5-image limit: the sixth becomes its alt text and the user is told", async () => {
    const five = Array.from({ length: 5 }, (_, i) => `<img src="blob:https://csm.example.com/${i + 1}" alt="i${i}">`).join("");
    const m = mount(cfg, { value: `<p>${five}</p>`, caret: "none", uploader: true });
    await settle();
    firePasteWith(IN_APP("blob:https://csm.example.com/9"), true);
    fireEvent.click(screen.getByRole("button", { name: "Keep Formatting" }));
    await settle();
    expect(m.html().match(/<img/g)).toHaveLength(5);
    expect(m.html()).not.toContain("csm.example.com/9");
    expect(m.html()).toContain("pic");
    expect(m.onPasteError).toHaveBeenCalledWith("count");
  });
});

// --- Keep Formatting only merges plain paragraphs ---------------------------------------------

describe("[reply] Keep Formatting paragraph merge keeps formatted paragraphs as blocks", () => {
  const keepReply = async (html: string): Promise<string> => {
    cleanup();
    const m = mount("reply");
    await pasteAnd(m, { "text/html": html, "text/plain": inlineText(html) }, "keep");
    return m.html();
  };

  it("an aligned, classed or align= paragraph is not merged into its neighbours", async () => {
    expect(await keepReply('<p>a</p><p style="text-align:center">b</p><p>c</p>')).toBe(
      '<p>a</p><p style="text-align: center;">b</p><p>c</p>',
    );
    expect(await keepReply('<p class="MsoNormal">a</p><p class="MsoNormal">b</p>')).toBe("<p>a</p><p>b</p>");
    expect(await keepReply('<p align="center">a</p><p>b</p><p>c</p>')).toMatch(/^<p[^>]*>a<\/p><p>b<br><br>c<\/p>$/);
  });

  it("plain neighbours around a styled paragraph merge on their own side only", async () => {
    expect(await keepReply('<p>a</p><p>b</p><p style="text-align:right">c</p><p>d</p><p>e</p>')).toBe(
      '<p>a<br><br>b</p><p style="text-align: right;">c</p><p>d<br><br>e</p>',
    );
  });

  it("a surviving empty paragraph is its own block, not three breaks", async () => {
    const out = await keepReply("<p>a</p><p><br></p><p>b</p>");
    expect(out).not.toContain("<br><br><br>");
  });
});

// --- The held paste lands where the caret was ----------------------------------------------

describe.each(CONFIGS)("[%s] the dialog's choice is inserted at the caret the paste was made at", (cfg) => {
  /** Mid-line caret, paste held, selection moved to the end of the line (what Chrome did on refocus), then the choice. */
  async function pasteWithSelectionMovedToEnd(clip: Clip, button: string): Promise<string> {
    const m = mount(cfg, { value: "<p>hello world</p>", caret: "range:5-5" });
    firePaste(clip);
    act(() => {
      getLexical().update(
        () => {
          const text = $getRoot().getFirstDescendant();
          if (!text || !$isTextNode(text)) throw new Error("no text");
          text.select(11, 11);
        },
        { discrete: true },
      );
    });
    fireEvent.click(screen.getByRole("button", { name: button }));
    await settle();
    return m.html();
  }

  const HTML_CLIP = { "text/html": "<b>X</b>", "text/plain": "X" };

  it("Keep Formatting", async () => {
    expect(inlineText(await pasteWithSelectionMovedToEnd(HTML_CLIP, "Keep Formatting"))).toBe("helloX world");
  });

  it("Remove Formatting", async () => {
    expect(inlineText(await pasteWithSelectionMovedToEnd(HTML_CLIP, "Remove Formatting"))).toBe("helloX world");
  });

  it.runIf(cfg === "reply")("Paste as Markdown", async () => {
    const html = await pasteWithSelectionMovedToEnd({ "text/plain": "**X** y" }, "Paste as Markdown");
    expect(inlineText(html)).toBe("helloX y world");
  });

  it("a held paste whose caret node is gone falls back to the end of the document, never dropped", async () => {
    const m = mount(cfg, { value: "<p>hello world</p>", caret: "range:5-5" });
    firePaste(HTML_CLIP);
    act(() => {
      getLexical().update(() => { $getRoot().clear(); }, { discrete: true });
    });
    fireEvent.click(screen.getByRole("button", { name: "Keep Formatting" }));
    await settle();
    expect(inlineText(m.html())).toContain("X");
  });
});

// --- Downstream: draft restore, comment pipeline, PDF ----------------------------------

async function restored(html: string): Promise<string> {
  cleanup();
  const m = mount("reply", { value: html, caret: "none" });
  await settle();
  return m.html();
}

/** HTML the editor produced from real pastes, for the downstream checks. */
async function produce(cfg: Cfg, clip: Clip, choice: Choice = "keep"): Promise<string> {
  cleanup();
  const m = mount(cfg);
  await pasteAnd(m, clip, choice);
  return m.html();
}

const SAMPLES: Array<[string, Cfg, Clip, Choice]> = [
  ["reply plain text with blank line and tabs", "reply", { "text/plain": "Hi team,\n\nA\tB\n    indented\nlast" }, "keep"],
  ["reply log/stack trace", "reply", { "text/plain": "ERR start\n    at a.b(C.java:1)\n    at d.e(F.java:2)" }, "keep"],
  ["reply Remove Formatting of web page", "reply", { "text/html": WEB_PAGE, "text/plain": WEB_PAGE_TEXT }, "remove"],
  ["reply Keep Google Docs nested list", "reply", { "text/html": GDOCS, "text/plain": GDOCS_TEXT }, "keep"],
  ["reply Keep web page", "reply", { "text/html": WEB_PAGE, "text/plain": WEB_PAGE_TEXT }, "keep"],
  ["reply Keep Gmail quote", "reply", { "text/html": GMAIL, "text/plain": GMAIL_TEXT }, "keep"],
  ["normal Keep web page", "normal", { "text/html": WEB_PAGE, "text/plain": WEB_PAGE_TEXT }, "keep"],
];

describe("downstream of the produced HTML", () => {
  it.each(CONFIGS)("[%s] a nested list exports its wrapper item with list-style-type: none, survives sanitizing and re-imports unchanged", async (cfg) => {
    cleanup();
    const m = mount(cfg, { value: "<ul><li>a<ul><li>b</li></ul></li><li>c</li></ul>", caret: "none" });
    await settle();
    const exported = m.html();
    expect(exported).toMatch(/<li[^>]*class="editor-list-item-nested"[^>]*style="list-style-type: none"/);
    expect(sanitizeRichTextHtml(exported)).toContain("list-style-type: none");
    expect(htmlToPdfPlainText(exported).match(/•/g)).toHaveLength(3);
    expect(await restored(exported)).toBe(exported);
  });

  it.each(SAMPLES)("draft restore round-trips: %s", async (_name, cfg, clip, choice) => {
    const html = await produce(cfg, clip, choice);
    expect(html).not.toBe("");
    // Structure survives; horizontal whitespace and the <br> that ends a <pre> are compared
    // loosely here because the next test pins that loss down separately.
    const loose = (h: string) => h.replace(/<br><\/pre>/g, "</pre>").replace(/[ \t\u00a0]+/g, "");
    expect(loose(await restored(html))).toBe(loose(html));
  });

  it.each([
    ["reply", "<p>A\tB<br>    indented<br>\ttabbed</p>"],
    ["normal", "<p>A\tB</p><p>    indented</p>"],
  ] as Array<[Cfg, string]>)(
    "[%s] restoring a draft keeps pasted tabs and leading indentation",
    async (cfg, html) => {
      // Pre-existing (not from the two commits): the editor emits plain spaces/tabs (no
      // white-space style) and relies on the renderers' pre-wrap, but InitialValuePlugin
      // re-parses the draft as HTML, which collapses tabs to a space and drops the spaces
      // after a <br> or at the start of a block. A pasted log or tab-separated row loses its
      // layout when the draft is restored or the HTML-source toggle is flipped back.
      cleanup();
      const m = mount(cfg, { value: html, caret: "none" });
      await settle();
      expect(m.html()).toBe(html);
    },
  );

  it.each(CONFIGS)("[%s] minified / ServiceNow-style HTML with whitespace gaps between blocks loads and re-saves with no new blank lines", async (cfg) => {
    cleanup();
    const html =
      "<div>  <p>a</p></div><ul><li>\t<ul><li>b</li></ul></li></ul><blockquote>  <p>q</p></blockquote>" +
      "<table><tbody><tr><td>  <p>c</p></td></tr></tbody></table>";
    const m = mount(cfg, { value: html, caret: "none" });
    await settle();
    const once = m.html();
    expect(once).not.toContain("<p><br></p>");
    expect(once.match(/<br>/g)).toBeNull();
    expect(await restored(once)).toBe(once);
  });

  it.each(SAMPLES)("comment pipeline keeps the structure: %s", async (_name, cfg, clip, choice) => {
    const html = await produce(cfg, clip, choice);
    const processed = preprocessCommentBodyHtml({ bodyHtml: html } as never);
    const sanitized = sanitizeRichTextHtml(processed);
    const count = (s: string, re: RegExp) => s.match(re)?.length ?? 0;
    for (const re of [/<br\b/g, /<li\b/g, /<ul\b/g, /<ol\b/g, /<p\b/g, /<pre\b/g, /<blockquote\b/g, /<table\b/g]) {
      expect(count(sanitized, re), String(re)).toBe(count(html, re));
    }
    // Spaces/tabs inside text are untouched by the source-whitespace collapsing.
    expect(inlineText(sanitized).replace(/\u00a0/g, " ")).toBe(inlineText(html).replace(/\u00a0/g, " "));
  });

  it("htmlToPdfPlainText keeps the line breaks of a reply with a blank line, indentation and a list", async () => {
    const html = await produce("reply", { "text/plain": "Hi team,\n\nA\tB\n    indented\nlast" });
    expect(htmlToPdfPlainText(html)).toBe("Hi team,\n\nA\tB\n    indented\nlast\n");
    const list = await produce("reply", { "text/html": GDOCS, "text/plain": GDOCS_TEXT });
    const text = htmlToPdfPlainText(list);
    expect(text).toContain("• One");
    expect(text).toContain("• Two");
    expect(text).toContain("• Three");
    expect(text.indexOf("• One")).toBeLessThan(text.indexOf("• Two"));
  });

  it("htmlToPdfPlainText: a code block keeps its line breaks and indentation", async () => {
    const html = await produce("reply", { "text/html": WEB_PAGE, "text/plain": WEB_PAGE_TEXT });
    expect(htmlToPdfPlainText(html)).toContain("const a = 1;\n  if (a) {\n    b();\n  }");
  });

  it("trimTrailingLineBreaks / hasCommentContent on pasted drafts", async () => {
    const trailing = await produce("reply", { "text/plain": "hello\n\n\n" });
    expect(trimTrailingLineBreaks(trailing)).toBe("<p>hello</p>");
    expect(hasCommentContent(trailing)).toBe(true);
    // Only blank lines pasted: no content, and trimming leaves the breaks (never an empty <p></p>).
    const blank = await produce("reply", { "text/plain": "\n\n" });
    expect(hasCommentContent(blank)).toBe(false);
    expect(trimTrailingLineBreaks(blank)).not.toBe("<p></p>");
    // Whitespace-only HTML paste.
    const spaces = await produce("reply", { "text/plain": "   \t  " });
    expect(hasCommentContent(spaces)).toBe(false);
    // An image-only paste counts as content.
    cleanup();
    const m = mount("reply");
    await pasteAnd(m, {}, "keep", [png()]);
    expect(hasCommentContent(m.html())).toBe(true);
  });
});

describe("HTML payloads with little or no body", () => {
  it.each(CONFIGS)("[%s] Keep Formatting of HTML with no body content (only a <meta>) still pastes the text/plain", async (cfg) => {
    // Pre-existing: any non-blank text/html opens the prompt, and Keep inserts nothing when
    // the HTML converts to no nodes, so the user's text is silently dropped.
    const m = mount(cfg);
    await pasteAnd(m, { "text/html": '<meta charset="utf-8">', "text/plain": "hello world" }, "keep");
    expect(inlineText(m.html())).toBe("hello world");
  });

  // Pre-existing: the fallback is htmlToPlainText (textContent), which glues blocks together.
  it.each(CONFIGS)("[%s] Remove Formatting of HTML with no text/plain keeps the blocks apart", async (cfg) => {
    const m = mount(cfg);
    await pasteAnd(m, { "text/html": "<p>one</p><p>two</p>" }, "remove");
    expect(inlineText(m.html().replace(/<br>|<\/p><p>/g, " "))).toBe("one two");
  });
});
