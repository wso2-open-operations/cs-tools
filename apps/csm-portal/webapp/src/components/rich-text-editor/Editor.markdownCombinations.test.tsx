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

// Markdown paste combinations: many realistic Markdown documents pasted into the
// reply box (and the normal editor) from every caret context, with both choices
// of the prompt. Block insertion into headings, list items, quotes, table cells and mid-line goes through
// $insertPastedNodes (insertPastedNodes.ts).

import { render, screen, fireEvent, act, cleanup } from "@testing-library/react";
import { describe, expect, it, vi, beforeAll } from "vitest";
import "@testing-library/jest-dom/vitest";
import { $getRoot, $getSelection, $isRangeSelection, type ElementNode, type LexicalEditor, type TextNode } from "lexical";
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

const getEditable = () => screen.getByTestId("case-description-editor");
const getLexical = () =>
  (getEditable() as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;

function paste(data: Record<string, string>): void {
  const event = new ClipboardEvent("paste", {
    bubbles: true,
    cancelable: true,
    clipboardData: { getData: (type: string) => data[type] ?? "", items: [] } as unknown as DataTransfer,
  } as ClipboardEventInit & { clipboardData: DataTransfer });
  act(() => {
    getEditable().dispatchEvent(event);
  });
}

const tick = () => act(async () => { await new Promise((r) => setTimeout(r, 25)); });

// ---- caret contexts -------------------------------------------------------------------------

const firstText = () => $getRoot().getFirstDescendant() as TextNode;
type Ctx = { value?: string; select: () => void };
const CONTEXTS = {
  /** Chrome's click into an empty editor: element point on the root. */
  root: { select: () => { $getRoot().select(0, 0); } },
  mid: { value: "<p>hello world</p>", select: () => { firstText().select(5, 5); } },
  afterBr: { value: "<p>one<br>two<br></p>", select: () => { ($getRoot().getFirstChild() as ElementNode).selectEnd(); } },
  listItem: { value: "<ul><li>item</li></ul>", select: () => { firstText().select(4, 4); } },
  heading: { value: "<h2>Head</h2>", select: () => { firstText().select(4, 4); } },
  tableCell: {
    value: "<table><tbody><tr><td><p>cell</p></td></tr></tbody></table>",
    select: () => { firstText().select(4, 4); },
  },
  quote: { value: "<blockquote>quoted</blockquote>", select: () => { firstText().select(6, 6); } },
  range: { value: "<p>hello world</p>", select: () => { firstText().select(0, 5); } },
  code: { value: "<pre>let a = 1;</pre>", select: () => { firstText().select(3, 3); } },
} satisfies Record<string, Ctx>;
type CtxName = keyof typeof CONTEXTS;

const REPLY = { enterInsertsLineBreak: true, markdownPaste: true, toolbarVariant: "full" } as const;

type Run = { html: string; prompted: boolean };

/** Renders an editor, puts the caret in `ctx`, pastes `data`, answers the prompt with `choice`. */
async function run(
  ctx: CtxName,
  data: string | Record<string, string>,
  choice: "markdown" | "plain" | "none",
  props: Partial<React.ComponentProps<typeof Editor>> = REPLY,
): Promise<Run> {
  cleanup();
  const onChange = vi.fn();
  const c: Ctx = CONTEXTS[ctx];
  render(<Editor autoFocus showToolbar={false} onChange={onChange} value={c.value} {...props} />);
  act(() => {
    getLexical().update(() => c.select(), { discrete: true });
  });
  paste(typeof data === "string" ? { "text/plain": data } : data);
  await tick();
  const name = choice === "markdown" ? "Paste as Markdown" : "Remove Formatting";
  const button = screen.queryByRole("button", { name });
  if (button && choice !== "none") fireEvent.click(button);
  await tick();
  return { html: (onChange.mock.calls.at(-1)?.[0] ?? "") as string, prompted: button !== null };
}

/** The exported HTML without the editor's class/spellcheck/value noise. */
function simple(html: string): string {
  return html
    .replace(/ (?:class|spellcheck|value)="[^"]*"/g, "")
    .replace(/ style="list-style-type: none"/g, "")
    .replace(/<b><strong>(.*?)<\/strong><\/b>/g, "<strong>$1</strong>")
    .replace(/<i><em>(.*?)<\/em><\/i>/g, "<em>$1</em>")
    .replace(/<code><span>(.*?)<\/span><\/code>/g, "<code>$1</code>")
    .replace(/<s><span>(.*?)<\/span><\/s>/g, "<s>$1</s>");
}

function visibleText(html: string): string {
  const body = new DOMParser().parseFromString(
    html.replace(/<br>/g, "\n").replace(/<\/(p|li|h[1-6]|pre|blockquote|tr)>/g, "$&\n"),
    "text/html",
  ).body;
  return (body.textContent ?? "").replace(/\u00a0/g, " ");
}

/** Things no paste may ever produce. */
function expectSafeAndTidy(html: string): void {
  expect(html).not.toMatch(/<(script|iframe|img|svg|style|object|embed)\b/i);
  expect(html).not.toMatch(/<[a-z][^>]*\son[a-z]+\s*=/i);
  expect(html).not.toMatch(/href="\s*(javascript|vbscript|data|file):/i);
  expect(html).not.toMatch(/<p[^>]*><p\b/);
  expect(html).not.toMatch(/<li[^>]*><p\b/);
  expect(html).not.toContain("<br></br>");
}

// ---- documents ------------------------------------------------------------------------------

const DOCS: Record<string, string> = {
  chatgpt:
    "## Steps to fix\n\nHere's what to do:\n\n1. **Stop** the service\n2. Edit `config.yaml`:\n   ```yaml\n   port: 8080\n   ```\n3. Restart\n\n- Note: see [docs](https://example.com/docs)\n- Another *point*\n\nLet me know!",
  githubIssue:
    "### Describe the bug\nThe login fails.\n\n### Steps\n1. Go to **login**\n2. Click `Submit`\n\n- [ ] todo\n- [x] done\n\n> quoted text\n\n```\nTypeError: x is undefined\n    at foo (a.js:1)\n```\n\nSee #123 and https://github.com/x/y/issues/1",
  runbook:
    "# Runbook\n\n## Restart\n\n1. SSH in\n   - run `sudo systemctl restart app`\n   - check logs\n     1. tail -f\n     2. grep ERROR\n2. Verify\n\n| Host | Status |\n|:--|--:|\n| a | **ok** |\n| b | |\n",
  releaseNotes:
    "# v1.2.0\n\n### Features\n* Added `foo`\n* Fixed [#12](https://x.com/12)\n    * nested four\n        * deep eight\n\n### Breaking\n+ plus item\n+ another\n\n---\n\n~~old~~ new",
  tabsNested: "# T\n\n- a\n\t- tab nested\n\t\t- tab deeper\n- b\n  - two\n    - four",
  mixedIndent: "- a\n  - b\n\t- c (tab after two spaces)\n\t\t- d",
  deepLists:
    "1. one\n   - bullet\n     1. num in bullet\n        - bullet in num\n2. two\n\n- top\n  1. num\n     - bul\n       1. deep",
  listMultiParagraph:
    "- item one\n\n  second paragraph\n\n  ```js\n  const a = 1;\n  ```\n\n- item **bold** `code` [l](https://a.com)\n- last",
  quotes:
    "> top\n>\n> second para\n> > nested\n> > deeper\n>\n> - list in quote\n> - two\n\n- item\n  > quote in list\n- next",
  code:
    "# C\n\n```js\nconst x = '<b>**not bold**</b>';\n\n\n\tindented\t\ntrail   \n# not heading\n- not list\n```\n\n~~~\ntilde fence\n~~~\n\n`` a`b `` and `x`",
  codeInLists: "- step\n  ```sh\n  echo hi\n  ```\n- next\n\n1. a\n   ~~~\n   code\n   ~~~\n2. b",
  rawHtml:
    "# H\n\n<script>alert(1)</script>\n<img src=x onerror=alert(1)>\n<b>raw</b> **bold**\n\n[x](javascript:alert(1))\n[y](data:text/html,hi)",
  xssText:
    "# <img src=x onerror=alert(1)>\n\n- <svg onload=alert(1)>\n- `<script>alert(1)</script>`\n\n| <b>a</b> | b |\n|---|---|\n| <iframe src=javascript:1> | x |\n\n> <style>*{}</style>\n\n[<b>x</b>](https://a.com)",
  badUrls:
    "**links**\n[js](JaVaScRiPt:alert(1))\n[ent](&#106;avascript:alert(1))\n[vb](vbscript:x)\n[data](data:text/html;base64,PHNjcmlwdD4=)\n<javascript:alert(1)>\n![j](javascript:alert(1))\n![d](data:image/png;base64,AAAA)\n![v](vbscript:x)",
  images:
    "# Imgs\n\n![alt](https://i.com/a.png)\n![](https://i.com/b.png)\n[![badge](https://img.shields.io/b.svg)](https://ci.com)\n\n- ![in list](https://i.com/c.png)\n- two\n\n![titled](https://i.com/d.png \"Title\") **x**",
  inline:
    "# One\n## Two\n### Three\n#### Four\n##### Five\n###### Six\n\n***bi*** _i_ __bold words__ **b** *i* ~~s~~\n\n<https://auto.com> bare https://bare.com/x?a=1 and [ref][1]\n\n[1]: https://ref.com \"T\"",
  tables: "| a | b `c\\|d` |\n|:---:|---:|\n| 1 \\| 2 | <b>x</b> |\n| | `code` |",
  unicode: "# 日本語 🚀\n\n- 项目一 😀\n- Ünïcode **émoji 🎉**",
  startNumbers: "3. three\n4. four\n\ntext\n\n10. ten\n11. eleven",
  hardBreaks: "# H\n\nline one  \nline two\\\nline three\n\n    indented code\n    line two",
  crlf: "# N\r\nline1\r\nline2\r\n\r\n\r\n\r\n\r\nafter many\r\n",
  leadingTrailing: "\n\n- a\n- b\n\n\n",
  introThenList: "Steps:\n1. x\n2. y",
};

// ---- the detector and the prompt ------------------------------------------------------------

describe("Markdown prompt: documents that must / must not prompt in the reply box", () => {
  it.each(Object.entries(DOCS))("prompts for the %s document", async (_name, md) => {
    const { prompted } = await run("root", md, "none");
    expect(prompted).toBe(true);
  });

  it.each([
    ["one numbered step", "1. only one step"],
    ["lone italic, code span and #123", "*italic* and `code` and #123"],
    ["Windows path, __init__, foo[0](x/y), e-mail, URL", "C:\\my_dir\\__init__\\x\nfoo[0](x/y)\nuser@example.com https://a.com"],
    ["list broken by prose", "- a\nprose\n- b"],
    ["2**3**4 and a * b * c", "2**3**4 and a * b * c"],
    ["a plain sentence with tab", "just a sentence.\nsecond line\n\nthird para\twith tab"],
    ["a bare URL", "https://a.com/b_c_d"],
    ["a glob that is not bold", "**/*.ts"],
    ["a log", "2024-01-01 INFO - started\n2024-01-01 WARN - x"],
    ["hard breaks that interrupt a list run", "line one  \nline two\\\nline three\n\n- a  \n  cont\n- b"],
    ["code indented by four spaces only", "    code line\n    two"],
    ["a reference-style link", "[x][1]\n\n[1]: https://a.com"],
  ])("does not prompt for %s: it pastes straight in as plain text", async (_name, text) => {
    const { prompted, html } = await run("root", text, "none");
    expect(prompted).toBe(false);
    expect(visibleText(html).trim()).toBe(text.replace(/\n{3,}/g, "\n\n").trim());
  });

  it("a glob like **/node_modules/** is not bold and does not interrupt the paste", async () => {
    for (const text of ["exclude: **/node_modules/**", "src/**/test/**"]) {
      const { prompted } = await run("root", text, "none");
      expect(prompted, text).toBe(false);
    }
  });

  it("a Word numbered list (<p class=MsoListParagraph>) is rendered HTML: Keep/Remove Formatting, not the Markdown prompt", async () => {
    // Word's clipboard HTML for a list is paragraphs with mso-list, not <ol>/<li>; its
    // "1.\tOne\r\n2.\tTwo" text/plain looks like Markdown but is rendered text.
    const { prompted } = await run(
      "root",
      {
        "text/html":
          '<p class="MsoListParagraph" style="text-indent:-.25in;mso-list:l0 level1 lfo1"><span style="mso-list:Ignore">1.<span>&nbsp;&nbsp;</span></span>One</p><p class="MsoListParagraph" style="text-indent:-.25in;mso-list:l0 level1 lfo1"><span style="mso-list:Ignore">2.<span>&nbsp;&nbsp;</span></span>Two</p>',
        "text/plain": "1.\tOne\r\n2.\tTwo",
      },
      "none",
    );
    expect(prompted).toBe(true);
    expect(screen.getByText("Paste formatted content?")).toBeInTheDocument();
    expect(screen.queryByText("Paste as Markdown?")).not.toBeInTheDocument();
  });

  it("a lone '> npm output' style line, or '>30 days', does not prompt; two quote lines do", async () => {
    for (const text of ["> app@1.0.0 build", ">30 days", "> app@1.0.0 build\n\n> tsc"]) {
      const { prompted } = await run("root", text, "none");
      expect(prompted, text).toBe(false);
    }
    for (const text of ["> app@1.0.0 build\n> tsc", "> quoted\n- item"]) {
      const { prompted } = await run("root", text, "none");
      expect(prompted, text).toBe(true);
    }
  });
});

// ---- exact output in the reply box from the Chrome empty-editor state ------------------------

describe("Paste as Markdown into an empty reply box (caret on the root): exact result", () => {
  const md = async (doc: string) => simple((await run("root", doc, "markdown")).html);

  it("a ChatGPT answer", async () => {
    expect(await md(DOCS.chatgpt)).toBe(
      '<h2>Steps to fix</h2><p>Here\'s what to do:</p><ol><li><strong>Stop</strong> the service</li><li>Edit <code>config.yaml</code>:<pre data-language="yaml">port: 8080</pre></li><li>Restart</li></ol><ul><li>Note: see <a href="https://example.com/docs">docs</a></li><li>Another <em>point</em></li></ul><p>Let me know!</p>',
    );
  });

  it("a GitHub issue: task lists stay literal, quote, fence, #123, bare URL", async () => {
    expect(await md(DOCS.githubIssue)).toBe(
      '<h3>Describe the bug</h3><p>The login fails.</p><h3>Steps</h3><ol><li>Go to <strong>login</strong></li><li>Click <code>Submit</code></li></ol><ul><li>[ ] todo</li><li>[x] done</li></ul><blockquote>quoted text</blockquote><pre>TypeError: x is undefined<br>    at foo (a.js:1)</pre><p>See #123 and <a href="https://github.com/x/y/issues/1">https://github.com/x/y/issues/1</a></p>',
    );
  });

  it("release notes: 4- and 8-space nesting, + bullets, the rule is dropped, strike", async () => {
    expect(await md(DOCS.releaseNotes)).toBe(
      '<h1>v1.2.0</h1><h3>Features</h3><ul><li>Added <code>foo</code></li><li>Fixed <a href="https://x.com/12">#12</a></li><li><ul><li>nested four</li><li><ul><li>deep eight</li></ul></li></ul></li></ul><h3>Breaking</h3><ul><li>plus item</li><li>another</li></ul><p><s>old</s> new</p>',
    );
  });

  it("tab, 2-space and 4-space bullet nesting all keep their depth", async () => {
    const out = await md(DOCS.tabsNested);
    expect(out).toContain("<li>a</li><li><ul><li>tab nested</li><li><ul><li>tab deeper</li></ul></li></ul></li><li>b</li><li><ul><li>two</li><li><ul><li>four</li></ul></li></ul></li>");
  });

  it("mixed tab/space indentation keeps four levels", async () => {
    const out = await md(DOCS.mixedIndent);
    expect((out.match(/<ul>/g) ?? []).length).toBe(4);
    for (const t of ["a", "b", "c (tab after two spaces)", "d"]) expect(out).toContain(`>${t}<`);
  });

  it("numbers inside bullets inside numbers, three levels, keep their list types", async () => {
    const out = await md(DOCS.deepLists);
    expect(out).toBe(
      "<ol><li>one</li><li><ul><li>bullet</li><li><ol><li>num in bullet</li><li><ul><li>bullet in num</li></ul></li></ol></li></ul></li><li>two</li></ol><ul><li>top</li><li><ol><li>num</li><li><ul><li>bul</li><li><ol><li>deep</li></ol></li></ul></li></ol></li></ul>",
    );
  });

  it("start numbers survive when each ordered list is separated by a paragraph", async () => {
    const out = await md(DOCS.startNumbers);
    expect(out).toBe(
      '<ol start="3"><li>three</li><li>four</li></ol><p>text</p><ol start="10"><li>ten</li><li>eleven</li></ol>',
    );
  });

  it("a paragraph then a list with no blank line between them", async () => {
    expect(await md(DOCS.introThenList)).toBe("<p>Steps:</p><ol><li>x</li><li>y</li></ol>");
  });

  it("fenced code keeps tabs, trailing spaces, blank lines, #/- lines and escapes HTML; inline `` a`b ``", async () => {
    const out = await md(DOCS.code);
    expect(out).toContain(
      '<pre data-language="js">const x = \'&lt;b&gt;**not bold**&lt;/b&gt;\';<br><br><br>\tindented\t<br>trail   <br># not heading<br>- not list</pre>',
    );
    expect(out).toContain("<pre>tilde fence</pre>");
    expect(out).toContain("<code>a`b</code>");
  });

  it("code fences inside list items stay inside their item, in order", async () => {
    expect(await md(DOCS.codeInLists)).toBe(
      '<ul><li>step<pre data-language="sh">echo hi</pre></li><li>next</li></ul><ol><li>a<pre>code</pre></li><li>b</li></ol>',
    );
  });

  it("four-space and tab indented code blocks become one code block with the indent removed", async () => {
    const out = await md(DOCS.hardBreaks);
    expect(out).toBe("<h1>H</h1><p>line one<br>line two<br>line three</p><pre>indented code<br>line two</pre>");
  });

  it("a table: header cells, inline formatting, escaped pipe, raw HTML escaped, empty cell", async () => {
    const out = await md(DOCS.tables);
    expect(out).toContain("<th><p>a</p></th>");
    expect(out).toContain('<p style="text-align: center;">1 | 2</p>');
    expect(out).toContain("&lt;b&gt;x&lt;/b&gt;");
    expect(out).toContain("<code>c|d</code>");
  });

  it("heading, table cell and link containing inline code", async () => {
    const out = await md("# Title with `code`\n\n| `h` | b |\n|---|---|\n| `x` | y |\n\n[`link code`](https://a.com)");
    expect(out).toContain("<h1>Title with <code>code</code></h1>");
    expect(out).toContain("<th><p><code>h</code></p></th>");
    expect(out).toContain('<a href="https://a.com"><code>link code</code></a>');
  });

  it("headings h1-h6, bold-italic, _i_, __b__, strike, autolink, bare URL, reference link with title", async () => {
    const out = await md(DOCS.inline);
    for (const n of [1, 2, 3, 4, 5, 6]) expect(out).toMatch(new RegExp(`<h${n}>`));
    expect(out).toContain("<i><strong>bi</strong></i>");
    expect(out).toContain("<strong>bold words</strong>");
    expect(out).toContain('<a href="https://auto.com">https://auto.com</a>');
    expect(out).toContain('<a href="https://bare.com/x?a=1">https://bare.com/x?a=1</a>');
    expect(out).toContain('<a href="https://ref.com" title="T">ref</a>');
  });

  it("CRLF text: lines join with <br>, 4 blank lines collapse into one paragraph break", async () => {
    expect(await md(DOCS.crlf)).toBe("<h1>N</h1><p>line1<br>line2<br><br>after many</p>");
  });

  it("leading and trailing blank lines add no empty blocks", async () => {
    expect(await md(DOCS.leadingTrailing)).toBe("<ul><li>a</li><li>b</li></ul>");
  });

  it("unicode and emoji survive", async () => {
    expect(await md(DOCS.unicode)).toBe("<h1>日本語 🚀</h1><ul><li>项目一 😀</li><li>Ünïcode <strong>émoji 🎉</strong></li></ul>");
  });

  it("an unclosed fence runs to the end of the text", async () => {
    expect(await md("```js\nconst a = 1;\n- not list\n")).toBe('<pre data-language="js">const a = 1;<br>- not list</pre>');
  });

  it("nested blockquotes at root: depth kept", async () => {
    expect(await md("> a\n> > b\n> > > c")).toBe("<blockquote>a<blockquote>b<blockquote>c</blockquote></blockquote></blockquote>");
  });

  it("images become links (alt or address), a badge keeps one link, unsafe images become inert text", async () => {
    const out = await md(DOCS.images);
    expect(out).toContain('<a href="https://i.com/a.png">alt</a>');
    expect(out).toContain('<a href="https://i.com/b.png">https://i.com/b.png</a>');
    expect(out).toContain('<a href="https://ci.com">badge</a>');
    expect(out).toContain('<li><a href="https://i.com/c.png">in list</a></li>');
    expect(out).toContain('<a href="https://i.com/d.png">titled</a>');
    expect(out).not.toContain("<img");
  });
});

// ---- Remove Formatting -------------------------------------------------------------------------

describe("Remove Formatting: the Markdown source stays literal", () => {
  it("keeps every symbol, tab and multiple space in the reply box", async () => {
    const { html } = await run("root", "# S\n\nmulti   spaces   inside\ntrailing two  \nbackslash\\\n\tTab\n    four", "plain");
    expect(html).toBe("<p># S<br><br>multi   spaces   inside<br>trailing two  <br>backslash\\<br>\tTab<br>    four</p>");
  });

  it("CRLF and 4 blank lines collapse to one blank line; a leading/trailing newline is kept as a break", async () => {
    expect((await run("root", DOCS.crlf, "plain")).html).toBe("<p># N<br>line1<br>line2<br><br>after many<br></p>");
  });

  it("a fenced block with blank lines pastes as lines (3+ newlines collapse to one blank line, as before)", async () => {
    const { html } = await run("root", "```\na\n\n\n\nb\n```\n- x\n- y", "plain");
    expect(html).toBe("<p>```<br>a<br><br>b<br>```<br>- x<br>- y</p>");
  });

  it("in a list item each line is its own item (the old tokens)", async () => {
    const { html } = await run("listItem", "# T\n- a\n- b", "plain");
    expect(simple(html)).toBe("<ul><li>item# T</li><li>- a</li><li>- b</li></ul>");
  });

  it("in a table cell lines are paragraphs", async () => {
    const { html } = await run("tableCell", "# T\n- a", "plain");
    expect(simple(html)).toBe("<table><tbody><tr><td><p>cell# T</p><p>- a</p></td></tr></tbody></table>");
  });
});

// ---- paste contexts -------------------------------------------------------------------------

describe("Paste as Markdown in other caret contexts", () => {
  it("middle of a typed line: block content is split around the caret", async () => {
    const { html } = await run("mid", "- a\n- b", "markdown");
    expect(simple(html)).toBe("<p>hello</p><ul><li>a</li><li>b</li></ul><p> world</p>");
  });

  it("end of a line after <br>s keeps the earlier lines", async () => {
    const { html } = await run("afterBr", "Hi **team**\n\nline", "markdown");
    expect(simple(html)).toBe("<p>one<br>two<br>Hi <strong>team</strong><br><br>line</p>");
  });

  it("replacing a range selection removes it", async () => {
    const { html } = await run("range", "## T\n\n- a", "markdown");
    expect(simple(html)).toBe("<h2>T</h2><ul><li>a</li></ul><p> world</p>");
  });

  it("inside a list item: lists join the list", async () => {
    const { html } = await run("listItem", "intro:\n- x\n- y", "markdown");
    expect(simple(html)).toBe("<ul><li>itemintro:</li><li>x</li><li>y</li></ul>");
  });

  it("inside a heading with only inline Markdown stays one heading", async () => {
    const { html } = await run("heading", "**bold** and `c` and [l](https://a.com)\n- a", "markdown");
    expect(simple(html)).toMatch(/^<h2>Head<strong>bold<\/strong> and <code>c<\/code> and <a href="https:\/\/a.com">l<\/a><\/h2><ul><li>a<\/li><\/ul>/);
  });

  it("inside a table cell the pasted blocks go inside the cell", async () => {
    const { html } = await run("tableCell", "## T\n\n1. a\n2. b", "markdown");
    expect(simple(html)).toContain("<td><p>cellT</p><ol><li>a</li><li>b</li></ol>");
  });

  it("inside a code block: no prompt, the text goes in literally with its lines", async () => {
    const r = await run("code", DOCS.chatgpt, "none");
    expect(r.prompted).toBe(false);
    expect(r.html).toContain("<pre");
    expect(r.html).toContain("## Steps to fix<br>");
    expect(r.html).toContain("   ```yaml<br>");
    expect(r.html).not.toMatch(/<(h2|ol|ul|strong)\b/);
  });

  it("a selection that starts in a paragraph and ends in a code block is not prompted either", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<p>para</p><pre>code</pre>" {...REPLY} />);
    act(() => {
      getLexical().update(
        () => {
          const [p, pre] = $getRoot().getChildren() as ElementNode[];
          const sel = (p.getFirstChild() as TextNode).select(1, 1);
          sel.focus.set((pre.getFirstChild() as TextNode).getKey(), 2, "text");
        },
        { discrete: true },
      );
    });
    paste({ "text/plain": "# T\n- a\n- b" });
    await tick();
    expect(screen.queryByText("Paste as Markdown?")).not.toBeInTheDocument();
  });

  it("inside bold text the pasted Markdown keeps its own formatting (not all bold)", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<p><b>bold</b> tail</p>" {...REPLY} />);
    act(() => { getLexical().update(() => { firstText().select(4, 4); }, { discrete: true }); });
    paste({ "text/plain": "plain and **b**\n- x\n- y" });
    await tick();
    fireEvent.click(screen.getByRole("button", { name: "Paste as Markdown" }));
    await tick();
    expect(simple(onChange.mock.calls.at(-1)?.[0] as string)).toMatch(/^<p><strong>bold<\/strong>plain and <strong>b<\/strong><\/p><ul>/);
  });

  it("the caret lands after the pasted content (typing continues there)", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<p>hello world</p>" {...REPLY} />);
    act(() => { getLexical().update(() => { firstText().select(5, 5); }, { discrete: true }); });
    paste({ "text/plain": "**x** and y" });
    await tick();
    fireEvent.click(screen.getByRole("button", { name: "Paste as Markdown" }));
    await tick();
    act(() => {
      getLexical().update(
        () => {
          const selection = $getSelection();
          if ($isRangeSelection(selection)) selection.insertText("Z");
        },
        { discrete: true },
      );
    });
    expect(simple(onChange.mock.calls.at(-1)?.[0] as string)).toBe("<p>hello<strong>x</strong> and yZ world</p>");
  });

  it("(decision) a heading or list pasted mid-line is glued to the text before the caret and loses its type", async () => {
    // "hello" + "# One" -> "helloOne" as a paragraph; there is no separating break.
    const { html } = await run("mid", "# One\n## Two\n\ntext", "markdown");
    expect(simple(html)).toBe("<p>helloOne</p><h2>Two</h2><p>text world</p>");
  });
});

// ---- invariants over every document x context x choice ------------------------------------------

const ALL_CTX = Object.keys(CONTEXTS) as CtxName[];

describe("invariants: every document, every caret context, both choices", () => {
  const cases = Object.entries(DOCS).flatMap(([name, md]) =>
    ALL_CTX.flatMap((ctx) => (["markdown", "plain"] as const).map((choice) => [name, ctx, choice, md] as const)),
  );

  it.each(cases)("%s | %s | %s: safe, tidy and no text loss", async (name, ctx, choice, md) => {
    const { html, prompted } = await run(ctx, md, choice);
    expect(html).not.toBe("");
    expectSafeAndTidy(html);
    const text = visibleText(html);

    if (choice === "plain" || !prompted) {
      // Nothing introduced, nothing lost: every non-empty source line is still there verbatim.
      for (const line of md.split(/\r?\n/).map((l) => l.trim()).filter(Boolean)) {
        expect(text, `line "${line}"`).toContain(line);
      }
      expect(html).not.toMatch(/<(strong|em|a|ol)\b/);
      return;
    }
    // Paste as Markdown: every word of the source is still visible (syntax characters excepted).
    if (["images", "badUrls", "rawHtml", "xssText"].includes(name)) return;
    const words = md.replace(/https?:\/\/\S+/g, " ").match(/[\p{L}\p{N}]{3,}/gu) ?? [];
    for (const word of words) expect(text, `word "${word}"`).toContain(word);
  });
});

// ---- inert output ---------------------------------------------------------------------------

describe("Paste as Markdown output is inert", () => {
  it("raw HTML in the source is shown as text", async () => {
    const { html } = await run("root", DOCS.rawHtml, "markdown");
    expect(html).toContain("&lt;script&gt;alert(1)&lt;/script&gt;");
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
    expect(html).toContain("&lt;b&gt;raw&lt;/b&gt;");
  });

  it("javascript:, vbscript:, data: links and images never become live", async () => {
    const { html } = await run("root", DOCS.badUrls, "markdown");
    expectSafeAndTidy(html);
    expect(html).not.toMatch(/href="(?!about:blank)[^"]*(javascript|vbscript|data):/i);
  });

  it("tags in headings, list items, table cells, quotes and link text are escaped", async () => {
    const { html } = await run("root", DOCS.xssText, "markdown");
    expectSafeAndTidy(html);
    expect(html).toContain("<h1>&lt;img src=x onerror=alert(1)&gt;</h1>");
    expect(html).toContain("&lt;iframe src=javascript:1&gt;");
  });

  it("(gap) an ftp: link keeps its text but its address is replaced by about:blank", async () => {
    const { html } = await run("root", "**x**\n[ftp](ftp://h/x)", "markdown");
    expect(html).toContain('<a href="about:blank" class="editor-link">ftp</a>');
  });

  it("(decision) a protocol-relative or relative link is kept as a link; the same address as an image is kept only if relative", async () => {
    const { html } = await run("root", "**x**\n[p](//evil.com/x)\n![a](//evil.com/a.png)\n![b](/rel/b.png)", "markdown");
    expect(simple(html)).toBe('<p><strong>x</strong><br><a href="//evil.com/x">p</a><br>a<br><a href="/rel/b.png">b</a></p>');
  });
});

// ---- the normal editor must be untouched ---------------------------------------------------------

describe("the normal editor (no flags): Markdown text pastes as plain text, no prompt", () => {
  const NORMAL: Partial<React.ComponentProps<typeof Editor>> = {};

  it.each(Object.entries(DOCS))("%s: no prompt, plain paragraphs", async (_name, md) => {
    const { html, prompted } = await run("root", md, "none", NORMAL);
    expect(prompted).toBe(false);
    expect(screen.queryByText("Paste as Markdown?")).not.toBeInTheDocument();
    expect(html).not.toMatch(/<(h[1-6]|ul|ol|table|blockquote|pre|strong|em|a)\b/);
    // one paragraph per non-empty line, text exactly as copied
    const lines = md.split(/\r?\n/).filter((l) => l !== "");
    const paragraphs = Array.from(new DOMParser().parseFromString(html, "text/html").body.querySelectorAll("p")).map(
      (p) => p.textContent,
    );
    expect(paragraphs.filter((t) => t !== "")).toEqual(lines);
  });

  it("with only enterInsertsLineBreak (no markdownPaste) Markdown is not prompted either", async () => {
    const { prompted, html } = await run("root", "# T\n\n- a\n- b", "none", { enterInsertsLineBreak: true });
    expect(prompted).toBe(false);
    expect(html).toBe("<p># T<br><br>- a<br>- b</p>");
  });

  it("with only markdownPaste (no line-break mode) paragraphs stay separate", async () => {
    const { html } = await run("root", "Hi **x**,\n\nLine A\nLine B", "markdown", { markdownPaste: true });
    expect(simple(html)).toBe("<p>Hi <strong>x</strong>,</p><p>Line A<br>Line B</p>");
  });

  it("HTML-carrying pastes still get the Keep/Remove prompt in the normal editor", async () => {
    await run("root", { "text/html": "<p>Hello <b>Word</b></p>", "text/plain": "Hello Word" }, "none", NORMAL);
    expect(screen.getByText("Paste formatted content?")).toBeInTheDocument();
  });
});

// ---- Markdown gaps that degrade silently (characterization) -----------------------------------------

describe("known gaps: these degrade silently", () => {
  it("(gap) a run of spaces inside text collapses to one space with Paste as Markdown (Remove Formatting keeps them)", async () => {
    const { html } = await run("root", "# S\n\nmulti   spaces   inside", "markdown");
    expect(simple(html)).toBe("<h1>S</h1><p>multi spaces inside</p>");
  });

  it("(gap) adjacent ordered lists with different delimiters merge and the second start number is lost", async () => {
    const { html } = await run("root", "1) paren\n2) two\n\n5. five\n6. six", "markdown");
    expect(simple(html)).toBe("<ol><li>paren</li><li>two</li><li>five</li><li>six</li></ol>");
  });

  it("(gap) a GFM table whose separator row has fewer than three dashes per column is not detected (pastes as plain text)", async () => {
    const { prompted, html } = await run("root", "| a | b |\n|--|--|\n| 1 | 2 |", "none");
    expect(prompted).toBe(false);
    expect(html).toBe("<p>| a | b |<br>|--|--|<br>| 1 | 2 |</p>");
  });

  it("(gap) a loose list item's paragraphs are joined by one <br>, not a blank line", async () => {
    const { html } = await run("root", DOCS.listMultiParagraph, "markdown");
    expect(simple(html)).toContain("<li>item one<br>second paragraph<pre data-language=\"js\">const a = 1;</pre></li>");
  });

  it("(gap) column alignment is kept on body cells only; header cells lose it", async () => {
    const { html } = await run("root", "| h1 | h2 |\n|:---|---:|\n| a | b |", "markdown");
    expect(html).toContain("<th><p>h1</p></th><th><p>h2</p></th>");
    expect(html).toContain('<td><p style="text-align: left;">a</p></td><td><p style="text-align: right;">b</p></td>');
  });

  it("(gap) a quote holding several blocks becomes a quote with nested blocks; paragraphs inside it join with <br><br>", async () => {
    const { html } = await run("root", DOCS.quotes, "markdown");
    expect(simple(html)).toBe(
      "<blockquote>top<br><br>second para<blockquote>nested<br>deeper</blockquote><ul><li>list in quote</li><li>two</li></ul></blockquote><ul><li>item<blockquote>quote in list</blockquote></li><li>next</li></ul>",
    );
  });
});

// ---- block insertion into an existing block -----------------------------------------------------

const QUOTE_WITH_LIST = "> intro\n> - x\n> - y";

describe("blocks pasted into a heading, list item, table cell or mid-line", () => {
  it("a quote holding a list at the end of a heading goes after the heading, never inside it", async () => {
    const { html } = await run("heading", QUOTE_WITH_LIST, "markdown");
    expect(simple(html)).toBe("<h2>Head</h2><blockquote>intro<ul><li>x</li><li>y</li></ul></blockquote>");
  });

  it("the same paste at the end of a list item goes after the list: no text item marked nested", async () => {
    const { html } = await run("listItem", QUOTE_WITH_LIST, "markdown");
    expect(simple(html)).toBe("<ul><li>item</li></ul><blockquote>intro<ul><li>x</li><li>y</li></ul></blockquote>");
    expect(html).not.toContain("editor-list-item-nested");
  });

  it("a list pasted mid-item splits the list; the text after the caret stays an item of the second half", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<ul><li>one</li><li>abcdef</li><li>three</li></ul>" {...REPLY} />);
    act(() => { getLexical().update(() => { (($getRoot().getFirstChild() as ElementNode).getChildAtIndex(1) as ElementNode).getFirstChild<TextNode>()!.select(3, 3); }, { discrete: true }); });
    paste({ "text/plain": "## T\n\n> q" });
    await tick();
    fireEvent.click(screen.getByRole("button", { name: "Paste as Markdown" }));
    await tick();
    expect(simple(onChange.mock.calls.at(-1)?.[0] as string)).toBe(
      "<ul><li>one</li><li>abcT</li></ul><blockquote>q</blockquote><ul><li>def</li><li>three</li></ul>",
    );
  });

  it("a Markdown table pasted into a table cell becomes one paragraph per row (cells joined by ' | '), not a nested table", async () => {
    const { html } = await run("tableCell", "| a | b |\n|---|---|\n| 1 | 2 |", "markdown");
    const dom = new DOMParser().parseFromString(html, "text/html");
    expect(dom.querySelector("td table, th table"), html).toBeNull();
    expect(dom.querySelectorAll("table")).toHaveLength(1);
    expect(Array.from(dom.querySelectorAll("td p")).map((p) => p.textContent)).toEqual(["cella | b", "1 | 2"]);
  });

  it("a list pasted into a table cell stays in the cell", async () => {
    const { html } = await run("tableCell", "- x\n- y", "markdown");
    expect(simple(html)).toBe("<table><tbody><tr><td><p>cell</p><ul><li>x</li><li>y</li></ul></td></tr></tbody></table>");
  });

  it("text after the caret stays in its own paragraph after a pasted nested blockquote", async () => {
    const { html } = await run("mid", "> a\n> > b\n> > > c", "markdown");
    expect(simple(html)).toBe("<p>hello</p><blockquote>a<blockquote>b<blockquote>c</blockquote></blockquote></blockquote><p> world</p>");
  });

  it("text after the caret stays in its own paragraph after a pasted list", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<p><b>bold</b> tail</p>" {...REPLY} />);
    act(() => { getLexical().update(() => { firstText().select(4, 4); }, { discrete: true }); });
    paste({ "text/plain": QUOTE_WITH_LIST });
    await tick();
    fireEvent.click(screen.getByRole("button", { name: "Paste as Markdown" }));
    await tick();
    expect(simple(onChange.mock.calls.at(-1)?.[0] as string)).toBe(
      "<p><strong>bold</strong></p><blockquote>intro<ul><li>x</li><li>y</li></ul></blockquote><p> tail</p>",
    );
  });

  it("a quote with a list pasted mid-line leaves no empty <p></p>", async () => {
    const { html } = await run("mid", DOCS.quotes, "markdown");
    expect(html).not.toContain("<p></p>");
  });

  it("a paste that ends in a list at the end of a line or heading leaves no empty paragraph below it", async () => {
    for (const ctx of ["afterBr", "heading"] as const) {
      const { html } = await run(ctx, "Steps:\n- x\n- y", "markdown");
      expect(html, ctx).not.toContain("<p><br></p>");
      expect(simple(html), ctx).toMatch(/<\/(p|h2)><ul><li>x<\/li><li>y<\/li><\/ul>$/);
    }
  });

  it("a list pasted at the very start of a paragraph goes before it, the paragraph keeps its text", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<p>hello</p>" {...REPLY} />);
    act(() => { getLexical().update(() => { firstText().select(0, 0); }, { discrete: true }); });
    paste({ "text/plain": "- x\n- y" });
    await tick();
    fireEvent.click(screen.getByRole("button", { name: "Paste as Markdown" }));
    await tick();
    expect(simple(onChange.mock.calls.at(-1)?.[0] as string)).toBe("<ul><li>x</li><li>y</li></ul><p>hello</p>");
  });

  it("Keep Formatting of a rendered list at the end of a heading or list item uses the same insertion", async () => {
    const clip = { "text/html": "<ul><li>x</li><li>y</li></ul>", "text/plain": "x\ny" };
    for (const [ctx, expected] of [
      ["heading", "<h2>Head</h2><ul><li>x</li><li>y</li></ul>"],
      ["listItem", "<ul><li>item</li><li>x</li><li>y</li></ul>"],
    ] as const) {
      cleanup();
      const onChange = vi.fn();
      const c: Ctx = CONTEXTS[ctx];
      render(<Editor autoFocus showToolbar={false} onChange={onChange} value={c.value} {...REPLY} />);
      act(() => { getLexical().update(() => c.select(), { discrete: true }); });
      paste(clip);
      await tick();
      fireEvent.click(screen.getByRole("button", { name: "Keep Formatting" }));
      await tick();
      expect(simple(onChange.mock.calls.at(-1)?.[0] as string), ctx).toBe(expected);
    }
  });
});

// ---- a list pasted into a list item keeps that item's depth ------------------------------------------

describe("a list pasted into a list item becomes siblings of that item, at its depth", () => {
  const NESTED = "<ul><li>one<ul><li>nested<ul><li>deep</li></ul></li></ul></li><li>two</li></ul>";

  /** Caret at the end of the text node whose text is `text`. */
  async function pasteIntoItem(
    text: string,
    clip: Record<string, string>,
    button: string,
    { value = NESTED, offset = text.length }: { value?: string; offset?: number } = {},
  ): Promise<string> {
    cleanup();
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value={value} {...REPLY} />);
    act(() => {
      getLexical().update(
        () => {
          const find = (node: ElementNode): TextNode | null => {
            for (const child of node.getChildren()) {
              if ("getTextContent" in child && (child as TextNode).__text === text) return child as TextNode;
              if ("getChildren" in child) {
                const hit = find(child as ElementNode);
                if (hit) return hit;
              }
            }
            return null;
          };
          find($getRoot())!.select(offset, offset);
        },
        { discrete: true },
      );
    });
    paste(clip);
    await tick();
    fireEvent.click(screen.getByRole("button", { name: button }));
    await tick();
    return simple(onChange.mock.calls.at(-1)?.[0] as string);
  }

  const MD = { "text/plain": "- alpha\n- beta\n  - deeper" };
  const HTML = { "text/html": "<ul><li>alpha</li><li>beta<ul><li>deeper</li></ul></li></ul>", "text/plain": "alpha\nbeta\ndeeper" };
  const PASTED = "<li>alpha</li><li>beta</li><li><ul><li>deeper</li></ul></li>";

  it.each([
    ["Markdown", MD, "Paste as Markdown"],
    ["Keep Formatting", HTML, "Keep Formatting"],
  ] as const)("%s at level 1: siblings of 'one' go after its own sub-list, 'two' stays last", async (_n, clip, button) => {
    expect(await pasteIntoItem("one", clip, button)).toBe(
      `<ul><li>one</li><li><ul><li>nested</li><li><ul><li>deep</li></ul></li></ul></li>${PASTED}<li>two</li></ul>`,
    );
  });

  it.each([
    ["Markdown", MD, "Paste as Markdown"],
    ["Keep Formatting", HTML, "Keep Formatting"],
  ] as const)("%s at level 2: alpha and beta follow 'nested' at level 2, 'deeper' is level 3", async (_n, clip, button) => {
    expect(await pasteIntoItem("nested", clip, button)).toBe(
      `<ul><li>one</li><li><ul><li>nested</li><li><ul><li>deep</li></ul></li>${PASTED}</ul></li><li>two</li></ul>`,
    );
  });

  it.each([
    ["Markdown", MD, "Paste as Markdown"],
    ["Keep Formatting", HTML, "Keep Formatting"],
  ] as const)("%s at level 3: the pasted items are level-3 siblings of 'deep'", async (_n, clip, button) => {
    expect(await pasteIntoItem("deep", clip, button)).toBe(
      `<ul><li>one</li><li><ul><li>nested</li><li><ul><li>deep</li>${PASTED}</ul></li></ul></li><li>two</li></ul>`,
    );
  });

  it("text after the caret becomes an item after the pasted ones, at the same depth", async () => {
    const out = await pasteIntoItem("nested", MD, "Paste as Markdown");
    expect(out).toContain("<li>nested</li>");
    cleanup();
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<ul><li>one<ul><li>abcdef</li></ul></li></ul>" {...REPLY} />);
    act(() => { getLexical().update(() => { const t = $getRoot().getLastDescendant() as TextNode; t.select(3, 3); }, { discrete: true }); });
    paste({ "text/plain": "- x\n- y" });
    await tick();
    fireEvent.click(screen.getByRole("button", { name: "Paste as Markdown" }));
    await tick();
    expect(simple(onChange.mock.calls.at(-1)?.[0] as string)).toBe(
      "<ul><li>one</li><li><ul><li>abc</li><li>x</li><li>y</li><li>def</li></ul></li></ul>",
    );
  });

  // The caret inside a link (an inline element of the item) used to be rejected, so the paste
  // split every ancestor list up to the root instead of staying at the item's depth.
  const LINKED =
    '<ul><li>one <a href="https://e.com/1">link1</a><ul><li>nested <a href="https://e.com/2">link2</a><ul><li>deep <a href="https://e.com/3">link3</a></li></ul></li></ul></li><li>two</li></ul>';
  const A = (n: number, text: string) => `<a href="https://e.com/${n}">${text}</a>`;

  it.each([
    ["Markdown", MD, "Paste as Markdown"],
    ["Keep Formatting", HTML, "Keep Formatting"],
  ] as const)("%s with the caret at the end of a link: level 1, 2 and 3 keep the item's depth", async (_n, clip, button) => {
    expect(await pasteIntoItem("link1", clip, button, { value: LINKED })).toBe(
      `<ul><li>one ${A(1, "link1")}</li><li><ul><li>nested ${A(2, "link2")}</li><li><ul><li>deep ${A(3, "link3")}</li></ul></li></ul></li>${PASTED}<li>two</li></ul>`,
    );
    expect(await pasteIntoItem("link2", clip, button, { value: LINKED })).toBe(
      `<ul><li>one ${A(1, "link1")}</li><li><ul><li>nested ${A(2, "link2")}</li><li><ul><li>deep ${A(3, "link3")}</li></ul></li>${PASTED}</ul></li><li>two</li></ul>`,
    );
    expect(await pasteIntoItem("link3", clip, button, { value: LINKED })).toBe(
      `<ul><li>one ${A(1, "link1")}</li><li><ul><li>nested ${A(2, "link2")}</li><li><ul><li>deep ${A(3, "link3")}</li>${PASTED}</ul></li></ul></li><li>two</li></ul>`,
    );
  });

  it.each([
    ["Markdown", MD, "Paste as Markdown"],
    ["Keep Formatting", HTML, "Keep Formatting"],
  ] as const)("%s with the caret in the middle of a link: the link is split and kept on both halves", async (_n, clip, button) => {
    expect(await pasteIntoItem("link2", clip, button, { value: LINKED, offset: 2 })).toBe(
      `<ul><li>one ${A(1, "link1")}</li><li><ul><li>nested ${A(2, "li")}</li><li><ul><li>deep ${A(3, "link3")}</li></ul></li>${PASTED}<li>${A(2, "nk2")}</li></ul></li><li>two</li></ul>`,
    );
  });
});

// ---- blank lines typed before a pasted block are kept -------------------------------------------------

describe("text typed with Enter, then a block pasted: the typed blank lines survive", () => {
  /** Types `x` and `enters` Enters (real keydowns, so the nodes are the typed ones), then pastes. */
  async function typedThenPaste(enters: number, md: string): Promise<string> {
    cleanup();
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<p>x</p>" {...REPLY} />);
    act(() => { getLexical().update(() => { ($getRoot().getFirstChild() as ElementNode).selectEnd(); }, { discrete: true }); });
    for (let i = 0; i < enters; i += 1) {
      act(() => {
        getEditable().dispatchEvent(
          new KeyboardEvent("keydown", { key: "Enter", code: "Enter", keyCode: 13, bubbles: true, cancelable: true }),
        );
      });
    }
    await tick();
    paste({ "text/plain": md });
    await tick();
    fireEvent.click(screen.getByRole("button", { name: "Paste as Markdown" }));
    await tick();
    return simple(onChange.mock.calls.at(-1)?.[0] as string);
  }

  it("one Enter: the caret line is replaced by the list, no empty paragraph", async () => {
    expect(await typedThenPaste(1, "- a\n- b")).toBe("<p>x</p><ul><li>a</li><li>b</li></ul>");
  });

  it("two Enters: the blank line is kept as one empty paragraph (the toolbar-conversion convention)", async () => {
    expect(await typedThenPaste(2, "- a\n- b")).toBe("<p>x</p><p><br></p><ul><li>a</li><li>b</li></ul>");
  });

  it("three Enters: two blank lines are two empty paragraphs", async () => {
    expect(await typedThenPaste(3, "- a\n- b")).toBe("<p>x</p><p><br></p><p><br></p><ul><li>a</li><li>b</li></ul>");
  });
});
