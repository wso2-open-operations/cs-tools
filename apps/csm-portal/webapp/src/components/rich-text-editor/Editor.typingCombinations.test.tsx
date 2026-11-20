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


// Typing and editing, no clipboard: every scenario runs in both editor configurations (the
// reply box: `enterInsertsLineBreak` + `markdownPaste`; the normal editor: neither) and the
// exported HTML is compared with a frozen expectation per configuration (see EXPECT at the
// bottom). Where the two differ it is the intended Enter
// change. Known bugs live in Editor.typingCombinations.bugs.test.tsx.

import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { CONFIGS, cleanup, mount, type Config, type Ctx } from "./typingHarness";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn(), clearError: vi.fn() }),
}));

afterEach(cleanup);

type Expected = { reply: string; normal: string };
const EXPECT: Record<string, Expected> = {};

/** One scenario in both configurations. `value` seeds the editor; otherwise it starts empty. */
function scenario(name: string, run: (c: Ctx) => void | Promise<void>, value = ""): void {
  describe(name, () => {
    for (const cfg of CONFIGS) {
      it(cfg, async () => {
        const c = mount(cfg as Config, value);
        await run(c);
        await new Promise((r) => setTimeout(r, 0));
        expect(c.html()).toBe(EXPECT[name]?.[cfg]);
      });
    }
  });
}

const H = "<p>Hello world</p>";
const three = (c: Ctx) => { c.root(); c.type("a"); c.enter(); c.type("b"); c.enter(); c.type("c"); };

// ---- spaces and tabs
scenario("typing basic", (c) => { c.root(); c.type("Hello"); });
scenario("multiple spaces", (c) => { c.root(); c.type("a   b"); });
scenario("leading and trailing spaces", (c) => { c.root(); c.type("  a  "); });
scenario("space-only paragraph", (c) => { c.root(); c.type("   "); });
scenario("non-breaking spaces typed", (c) => { c.root(); c.type("a  b"); });
scenario("spaces around a line break", (c) => { c.root(); c.type("a  "); c.enter(); c.type("  b"); });
scenario("space-only line between lines", (c) => { c.root(); c.type("a"); c.enter(); c.type("   "); c.enter(); c.type("b"); });
scenario("space after bold stays plain", (c) => { c.root(); c.fmt("Bold"); c.type("bo"); c.fmt("Bold"); c.type(" x"); });
scenario("tab node exported", (c) => { c.root(); c.type("a"); c.tabChar(); c.type("b"); });
scenario("Tab key in a paragraph (nothing inserted)", (c) => { c.root(); c.type("a"); c.tab(); c.type("b"); });
scenario("Shift+Tab in a paragraph", (c) => { c.root(); c.type("a"); c.shiftTab(); });
scenario("Tab at the start of an empty editor", (c) => { c.root(); c.tab(); c.type("a"); });
scenario("Tab in the 2nd bullet nests it", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); c.tab(); c.type("c"); });
scenario("Tab in a code block", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); c.tab(); c.type("b"); });
scenario("code block keeps spaces", async (c) => { c.end(); await c.click("Code Block"); c.type("  a   b  "); }, "<p><br></p>");

// ---- Enter / line breaks
scenario("Enter once", (c) => { c.root(); c.type("a"); c.enter(); c.type("b"); });
scenario("Enter twice", (c) => { c.root(); c.type("a"); c.enter(2); c.type("b"); });
scenario("Enter at start of a line", (c) => { c.start(); c.enter(); }, H);
scenario("Enter in the middle of a word", (c) => { c.after("Hel"); c.enter(); }, H);
scenario("Enter after a space", (c) => { c.after("Hello "); c.enter(); }, H);
scenario("Enter at end", (c) => { c.end(); c.enter(); }, H);
scenario("Enter x3 at end then type", (c) => { c.end(); c.enter(3); c.type("z"); }, H);
scenario("Shift+Enter", (c) => { c.end(); c.shiftEnter(); c.type("z"); }, H);
scenario("Shift+Enter x2", (c) => { c.end(); c.shiftEnter(); c.shiftEnter(); c.type("z"); }, H);
scenario("Enter then Shift+Enter", (c) => { c.end(); c.enter(); c.shiftEnter(); c.type("z"); }, H);
scenario("Enter with a range selected", (c) => { c.select("world"); c.enter(); }, H);
scenario("Enter replacing a range across two paragraphs", (c) => { c.selectBetween("a", "b"); c.enter(); }, "<p>a</p><p>b</p>");
scenario("Enter at end of 1st of two legacy paragraphs", (c) => { c.after("a"); c.enter(); }, "<p>a</p><p>b</p>");
scenario("Enter in the middle of a seeded br line", (c) => { c.after("o"); c.enter(); }, "<p>one<br>two</p>");
scenario("Alt+Enter", (c) => { c.end(); c.key({ key: "Enter", keyCode: 13, altKey: true }); c.type("z"); }, H);
scenario("Enter after bold", (c) => { c.root(); c.fmt("Bold"); c.type("bb"); c.enter(); c.type("z"); });
scenario("Enter after bold toggled off", (c) => { c.root(); c.fmt("Bold"); c.type("bb"); c.fmt("Bold"); c.enter(); c.type("z"); });
scenario("Enter in the middle of italic", (c) => { c.root(); c.fmt("Italic"); c.type("abcd"); c.after("ab"); c.enter(); });
scenario("Enter after a link", (c) => { c.end(); c.enter(); c.type("z"); }, '<p>see <a href="https://example.com">link</a></p>');
scenario("Enter after inline code", (c) => { c.root(); c.fmt("Inline Code"); c.type("cc"); c.enter(); c.type("z"); });
scenario("Enter in the middle of inline code", (c) => { c.after("c"); c.enter(); }, "<p><code>cc</code></p>");
scenario("Enter on an empty root, then type", (c) => { c.root(); c.enter(); c.type("x"); });
scenario("Enter x3 on an empty root, then type", (c) => { c.root(); c.enter(3); c.type("z"); });
scenario("Enter only, on an empty root", (c) => { c.root(); c.enter(); });
scenario("Backspace over a trailing break", (c) => { c.end(); c.enter(); c.backspace(); }, "<p>Hello</p>");

// ---- lists
scenario("bullet single", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); });
scenario("number single", async (c) => { c.root(); c.type("a"); await c.click("Numbered List"); });
scenario("bullet from an empty root", async (c) => { c.root(); await c.click("Bullet List"); c.type("a"); c.enter(); c.type("b"); });
scenario("number from an empty root", async (c) => { c.root(); await c.click("Numbered List"); c.type("a"); c.enter(); c.type("b"); });
scenario("bullet: Enter on an empty item exits", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); c.enter(2); c.type("z"); });
scenario("number: Enter on an empty item exits", async (c) => { c.root(); c.type("a"); await c.click("Numbered List"); c.enter(2); c.type("z"); });
scenario("bullet 3 items", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); c.enter(); c.type("c"); });
scenario("bullet toggled off", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.click("Bullet List"); });
scenario("number toggled off", async (c) => { c.root(); c.type("a"); await c.click("Numbered List"); await c.click("Numbered List"); });
scenario("bullet to number", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.click("Numbered List"); });
scenario("number to bullet", async (c) => { c.root(); c.type("a"); await c.click("Numbered List"); await c.click("Bullet List"); });
scenario("bullet two levels (Indent button)", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); await c.click("Indent");
  c.enter(); c.type("c"); await c.click("Indent"); c.enter(); c.type("d");
});
scenario("Enter on an empty nested item outdents", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); await c.click("Indent"); c.enter(2); c.type("z");
});
scenario("Enter x3 on a nested item exits the list", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); await c.click("Indent"); c.enter(3); c.type("z");
});
scenario("Outdent button on a nested item", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); await c.click("Indent"); await c.click("Outdent");
});
scenario("Indent then Outdent on the first item", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.click("Indent"); await c.click("Outdent"); });
scenario("Outdent on a top-level item", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.click("Outdent"); });
scenario("number multi-level, back out", async (c) => {
  c.root(); c.type("a"); await c.click("Numbered List"); c.enter(); c.type("b"); await c.click("Indent");
  c.enter(); c.type("c"); c.enter(); c.type("d"); await c.click("Outdent"); c.enter(); c.type("e");
});
scenario("bullet inside number", async (c) => {
  c.root(); c.type("a"); await c.click("Numbered List"); c.enter(); c.type("b"); await c.click("Indent");
  await c.click("Bullet List"); c.enter(); c.type("c");
});
scenario("number inside bullet", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); await c.click("Indent");
  await c.click("Numbered List"); c.enter(); c.type("c");
});
scenario("toggle list off on a nested item", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.type("b"); await c.click("Indent"); await c.click("Bullet List");
});
scenario("br lines to bullets (all)", async (c) => { three(c); c.selectAll(); await c.click("Bullet List"); });
scenario("br lines to bullets (middle line only)", async (c) => { three(c); c.select("b"); await c.click("Bullet List"); });
scenario("br lines to numbers, indent the middle", async (c) => { three(c); c.selectAll(); await c.click("Numbered List"); c.select("b"); await c.click("Indent"); });
scenario("br lines with a blank to bullets", async (c) => { c.root(); c.type("a"); c.enter(2); c.type("c"); c.selectAll(); await c.click("Bullet List"); });
scenario("br lines with a trailing blank to bullets", async (c) => { c.root(); c.type("a"); c.enter(2); c.selectAll(); await c.click("Bullet List"); });
scenario("list, exit, then Enter makes a break", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); c.enter(2); c.type("z"); c.enter(); c.type("y"); });
scenario("bold item, Enter keeps bold", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); c.enter(); c.fmt("Bold"); c.type("b"); c.enter(); c.type("c"); });
scenario("bullet list then br lines to numbers (two lists)", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(2); c.type("x"); c.enter(); c.type("y"); c.select("x");
  c.selectBetween("x", "y"); await c.click("Numbered List");
});
scenario("Enter in a list item with a range selected", async (c) => {
  c.root(); c.type("abcd"); await c.click("Bullet List"); c.select("bc"); c.enter();
});

// ---- indentation on paragraphs
scenario("indent paragraph", async (c) => { c.root(); c.type("a"); await c.click("Indent"); });
scenario("indent x2", async (c) => { c.root(); c.type("a"); await c.click("Indent"); await c.click("Indent"); });
scenario("indent then outdent", async (c) => { c.root(); c.type("a"); await c.click("Indent"); await c.click("Outdent"); });
scenario("outdent plain paragraph", async (c) => { c.root(); c.type("a"); await c.click("Outdent"); });
scenario("indent ONE br line (that line only in the reply box)", async (c) => { three(c); c.select("b"); await c.click("Indent"); });
scenario("indent then Enter", async (c) => { c.root(); c.type("a"); await c.click("Indent"); c.enter(); c.type("b"); });
scenario("indent the first of two legacy paragraphs", async (c) => { c.select("a"); await c.click("Indent"); }, "<p>a</p><p>b</p>");
scenario("indent a heading", async (c) => { c.root(); c.type("a"); await c.block("h2"); await c.click("Indent"); });
scenario("indent a quote", async (c) => { c.root(); c.type("a"); await c.block("quote"); await c.click("Indent"); });
scenario("indent then list then outdent", async (c) => { c.root(); c.type("a"); await c.click("Indent"); await c.click("Bullet List"); await c.click("Outdent"); });
scenario("indent a code block (no-op)", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); await c.click("Indent"); });
scenario("indent from an empty root", async (c) => { c.root(); await c.click("Indent"); c.type("x"); });
scenario("indented paragraph then br line to bullet", async (c) => { c.root(); c.type("a"); await c.click("Indent"); c.enter(); c.type("b"); c.select("b"); await c.click("Bullet List"); });

// ---- code
scenario("code block: Enter x2 stays in the block", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); c.enter(2); c.type("b"); });
scenario("code block: Enter x3 exits", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); c.enter(3); c.type("b"); });
scenario("code block toggled off", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); await c.click("Code Block"); });
scenario("code block: Shift+Enter", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); c.shiftEnter(); c.type("b"); });
scenario("code block from br lines (one block in the reply box)", async (c) => { c.root(); c.type("a"); c.enter(); c.type("b"); c.selectAll(); await c.click("Code Block"); });
scenario("code block on the middle br line", async (c) => { three(c); c.select("b"); await c.click("Code Block"); });
scenario("code block between lists", async (c) => {
  c.root(); c.type("a"); await c.click("Bullet List"); c.enter(2); c.type("code"); await c.click("Code Block"); c.enter(3); c.type("b"); await c.click("Bullet List");
});
scenario("code block to heading", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); await c.block("h2"); });
scenario("inline code on a word", async (c) => { c.root(); c.type("run foo now"); c.select("foo"); await c.click("Inline Code"); });
scenario("inline code toggled off", async (c) => { c.root(); c.type("run foo now"); c.select("foo"); await c.click("Inline Code"); await c.click("Inline Code"); });
scenario("inline code across a br", async (c) => { c.root(); c.type("aa"); c.enter(); c.type("bb"); c.selectAll(); await c.click("Inline Code"); });
scenario("inline code, toggle off, break, text", (c) => { c.root(); c.type("x "); c.fmt("Inline Code"); c.type("cc"); c.fmt("Inline Code"); c.enter(); c.type("after"); });
scenario("inline code in a list item", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); c.select("a"); await c.click("Inline Code"); });
scenario("inline code and bold on one word", async (c) => { c.root(); c.type("a foo b"); c.select("foo"); await c.click("Inline Code"); await c.click("Bold"); });

// ---- images
scenario("image alone", async (c) => { c.root(); await c.upload(); });
scenario("image then type", async (c) => { c.root(); await c.upload(); c.type("after"); });
scenario("Enter after an image", async (c) => { c.root(); c.type("a"); await c.upload(); c.enter(); c.type("z"); });
scenario("Enter before an image", async (c) => { c.root(); await c.upload(); c.start(); c.enter(); });
scenario("image in a list item", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.upload(); });
scenario("image in a list item, then Enter", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.upload(); c.enter(); c.type("b"); });
scenario("image between br lines", async (c) => { c.root(); c.type("a"); c.enter(); await c.upload(); c.enter(); c.type("b"); });
scenario("br lines with an image to bullets", async (c) => { c.root(); c.type("a"); c.enter(); await c.upload(); c.enter(); c.type("b"); c.selectAll(); await c.click("Bullet List"); });
scenario("image then Align Center", async (c) => { c.root(); c.type("a"); await c.upload(); await c.click("Align Center"); });
scenario("image in a code block", async (c) => { c.root(); c.type("a"); await c.click("Code Block"); await c.upload(); });
scenario("image then undo", async (c) => { c.root(); c.type("a"); await c.upload(); await c.undo(); });

// ---- other formatting
scenario("bold underline strikethrough", async (c) => { c.root(); c.type("abc"); c.selectAll(); await c.click("Bold"); await c.click("Underline"); await c.click("Strikethrough"); });
scenario("underline only", async (c) => { c.root(); c.type("abc"); c.selectAll(); await c.click("Underline"); });
scenario("strikethrough only", async (c) => { c.root(); c.type("abc"); c.selectAll(); await c.click("Strikethrough"); });
scenario("italic on a word", async (c) => { c.root(); c.type("a bc d"); c.select("bc"); await c.click("Italic"); });
scenario("bold across a br", async (c) => { c.root(); c.type("aa"); c.enter(); c.type("bb"); c.selectAll(); await c.click("Bold"); });
scenario("bold across part of two lines", async (c) => { c.root(); c.type("aa"); c.enter(); c.type("bb"); c.selectBetween("a", "b"); await c.click("Bold"); });
scenario("bold toggled off again", async (c) => { c.root(); c.type("abc"); c.selectAll(); await c.click("Bold"); await c.click("Bold"); });
scenario("heading h2", async (c) => { c.root(); c.type("a"); await c.block("h2"); });
scenario("heading: Enter at end starts a paragraph", async (c) => { c.root(); c.type("a"); await c.block("h1"); c.enter(); c.type("b"); });
scenario("heading: Enter in the middle splits it", async (c) => { c.root(); c.type("abcd"); await c.block("h2"); c.after("ab"); c.enter(); });
scenario("heading: Enter at start", async (c) => { c.root(); c.type("abcd"); await c.block("h2"); c.start(); c.enter(); });
scenario("heading: Shift+Enter", async (c) => { c.root(); c.type("ab"); await c.block("h2"); c.shiftEnter(); c.type("c"); });
scenario("heading on one br line", async (c) => { three(c); c.select("b"); await c.block("h2"); });
scenario("heading on all br lines", async (c) => { c.root(); c.type("a"); c.enter(); c.type("b"); c.selectAll(); await c.block("h3"); });
scenario("heading on a line with a blank before it", async (c) => { c.root(); c.type("a"); c.enter(2); c.type("b"); c.select("b"); await c.block("h2"); });
scenario("heading back to paragraph", async (c) => { c.root(); c.type("a"); await c.block("h2"); await c.block("paragraph"); });
scenario("heading, Enter x2, type", async (c) => { c.root(); c.type("a"); await c.block("h2"); c.enter(2); c.type("z"); });
scenario("quote", async (c) => { c.root(); c.type("a"); await c.block("quote"); });
scenario("quote: Enter x2 then type", async (c) => { c.root(); c.type("a"); await c.block("quote"); c.enter(2); c.type("b"); });
scenario("quote on one br line", async (c) => { three(c); c.select("b"); await c.block("quote"); });
scenario("quote on all br lines", async (c) => { c.root(); c.type("a"); c.enter(); c.type("b"); c.selectAll(); await c.block("quote"); });
scenario("quote back to paragraph", async (c) => { c.root(); c.type("a"); await c.block("quote"); await c.block("paragraph"); });
scenario("quote then bullet", async (c) => { c.root(); c.type("a"); await c.block("quote"); await c.click("Bullet List"); });
for (const a of ["Align Left", "Align Center", "Align Right", "Justify"]) {
  scenario(`alignment: ${a}`, async (c) => { c.root(); c.type("a"); await c.click(a); });
}
scenario("center ONE br line (that line only in the reply box)", async (c) => { three(c); c.select("b"); await c.click("Align Center"); });
scenario("center then Enter then type", async (c) => { c.root(); c.type("a"); await c.click("Align Center"); c.enter(); c.type("b"); });
scenario("center then Enter x2 then type", async (c) => { c.root(); c.type("a"); await c.click("Align Center"); c.enter(2); c.type("b"); });
scenario("center a list item", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.click("Align Center"); });
scenario("center from an empty root", async (c) => { c.root(); await c.click("Align Center"); c.type("a"); });
scenario("center then left", async (c) => { c.root(); c.type("a"); await c.click("Align Center"); await c.click("Align Left"); });
scenario("link on a selection", async (c) => { c.root(); c.type("go here now"); c.select("here"); await c.link("https://example.com"); });
scenario("link toggled off", async (c) => { c.root(); c.type("go here now"); c.select("here"); await c.link("https://example.com"); c.select("here"); await c.click("Link"); });
scenario("link with a javascript: URL is refused", async (c) => { c.root(); c.type("go here"); c.select("here"); await c.link("javascript:alert(1)"); });
scenario("link on a br line", async (c) => { three(c); c.select("b"); await c.link("https://example.com"); });
scenario("link, caret after, Enter", async (c) => { c.root(); c.type("go here now"); c.select("here"); await c.link("https://example.com"); c.after("now"); c.enter(); c.type("z"); });
scenario("undo a list conversion of br lines", async (c) => { c.root(); c.type("a"); c.enter(); c.type("b"); c.selectAll(); await c.click("Bullet List"); await c.undo(); });
scenario("undo then redo a list", async (c) => { c.root(); c.type("a"); await c.click("Bullet List"); await c.undo(); await c.redo(); });
scenario("undo a heading on a br line", async (c) => { three(c); c.select("b"); await c.block("h2"); await c.undo(); });

// Frozen expectations, reviewed by hand.
Object.assign(EXPECT, {
  "typing basic": {
    reply: "<p>Hello</p>",
    normal: "<p>Hello</p>",
  },
  "multiple spaces": {
    reply: "<p>a   b</p>",
    normal: "<p>a   b</p>",
  },
  "leading and trailing spaces": {
    reply: "<p>  a  </p>",
    normal: "<p>  a  </p>",
  },
  "space-only paragraph": {
    reply: "<p>   </p>",
    normal: "<p>   </p>",
  },
  "non-breaking spaces typed": {
    reply: "<p>a&nbsp;&nbsp;b</p>",
    normal: "<p>a&nbsp;&nbsp;b</p>",
  },
  "spaces around a line break": {
    reply: "<p>a  <br>  b</p>",
    normal: "<p>a  </p><p>  b</p>",
  },
  "space-only line between lines": {
    reply: "<p>a<br>   <br>b</p>",
    normal: "<p>a</p><p>   </p><p>b</p>",
  },
  "space after bold stays plain": {
    reply: "<p><b><strong class=\"editor-text-bold\">bo</strong></b> x</p>",
    normal: "<p><b><strong class=\"editor-text-bold\">bo</strong></b> x</p>",
  },
  "tab node exported": {
    reply: "<p>a\tb</p>",
    normal: "<p>a\tb</p>",
  },
  "Tab key in a paragraph (nothing inserted)": {
    reply: "<p>ab</p>",
    normal: "<p>ab</p>",
  },
  "Shift+Tab in a paragraph": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "Tab at the start of an empty editor": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "Tab in the 2nd bullet nests it": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">bc</li></ul></li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">bc</li></ul></li></ul>",
  },
  "Tab in a code block": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">ab</pre>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">ab</pre>",
  },
  "code block keeps spaces": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">  a   b  </pre>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">  a   b  </pre>",
  },
  "Enter once": {
    reply: "<p>a<br>b</p>",
    normal: "<p>a</p><p>b</p>",
  },
  "Enter twice": {
    reply: "<p>a<br><br>b</p>",
    normal: "<p>a</p><p><br></p><p>b</p>",
  },
  "Enter at start of a line": {
    reply: "<p><br>Hello world</p>",
    normal: "<p><br></p><p>Hello world</p>",
  },
  "Enter in the middle of a word": {
    reply: "<p>Hel<br>lo world</p>",
    normal: "<p>Hel</p><p>lo world</p>",
  },
  "Enter after a space": {
    reply: "<p>Hello <br>world</p>",
    normal: "<p>Hello </p><p>world</p>",
  },
  "Enter at end": {
    reply: "<p>Hello world<br></p>",
    normal: "<p>Hello world</p><p><br></p>",
  },
  "Enter x3 at end then type": {
    reply: "<p>Hello world<br><br><br>z</p>",
    normal: "<p>Hello world</p><p><br></p><p><br></p><p>z</p>",
  },
  "Shift+Enter": {
    reply: "<p>Hello world<br>z</p>",
    normal: "<p>Hello world<br>z</p>",
  },
  "Shift+Enter x2": {
    reply: "<p>Hello world<br><br>z</p>",
    normal: "<p>Hello world<br><br>z</p>",
  },
  "Enter then Shift+Enter": {
    reply: "<p>Hello world<br><br>z</p>",
    normal: "<p>Hello world</p><p><br>z</p>",
  },
  "Enter with a range selected": {
    reply: "<p>Hello <br></p>",
    normal: "<p>Hello </p><p><br></p>",
  },
  "Enter replacing a range across two paragraphs": {
    reply: "<p><br></p>",
    normal: "<p><br></p><p><br></p>",
  },
  "Enter at end of 1st of two legacy paragraphs": {
    reply: "<p>a<br></p><p>b</p>",
    normal: "<p>a</p><p><br></p><p>b</p>",
  },
  "Enter in the middle of a seeded br line": {
    reply: "<p>o<br>ne<br>two</p>",
    normal: "<p>o</p><p>ne<br>two</p>",
  },
  "Alt+Enter": {
    reply: "<p>Hello world<br>z</p>",
    normal: "<p>Hello world</p><p>z</p>",
  },
  "Enter after bold": {
    reply: "<p><b><strong class=\"editor-text-bold\">bb</strong></b><br><b><strong class=\"editor-text-bold\">z</strong></b></p>",
    normal: "<p><b><strong class=\"editor-text-bold\">bb</strong></b></p><p><b><strong class=\"editor-text-bold\">z</strong></b></p>",
  },
  "Enter after bold toggled off": {
    reply: "<p><b><strong class=\"editor-text-bold\">bb</strong></b><br>z</p>",
    normal: "<p><b><strong class=\"editor-text-bold\">bb</strong></b></p><p>z</p>",
  },
  "Enter in the middle of italic": {
    reply: "<p><i><em class=\"editor-text-italic\">ab</em></i><br><i><em class=\"editor-text-italic\">cd</em></i></p>",
    normal: "<p><i><em class=\"editor-text-italic\">ab</em></i></p><p><i><em class=\"editor-text-italic\">cd</em></i></p>",
  },
  "Enter after a link": {
    reply: "<p>see <a href=\"https://example.com\" class=\"editor-link\">link</a><br>z</p>",
    normal: "<p>see <a href=\"https://example.com\" class=\"editor-link\">link</a></p><p>z</p>",
  },
  "Enter after inline code": {
    reply: "<p><code spellcheck=\"false\"><span class=\"editor-text-code\">cc</span></code><br><code spellcheck=\"false\"><span class=\"editor-text-code\">z</span></code></p>",
    normal: "<p><code spellcheck=\"false\"><span class=\"editor-text-code\">cc</span></code></p><p><code spellcheck=\"false\"><span class=\"editor-text-code\">z</span></code></p>",
  },
  "Enter in the middle of inline code": {
    reply: "<p><code spellcheck=\"false\"><span class=\"editor-text-code\">c</span></code><br><code spellcheck=\"false\"><span class=\"editor-text-code\">c</span></code></p>",
    normal: "<p><code spellcheck=\"false\"><span class=\"editor-text-code\">c</span></code></p><p><code spellcheck=\"false\"><span class=\"editor-text-code\">c</span></code></p>",
  },
  "Enter on an empty root, then type": {
    reply: "<p><br>x</p>",
    normal: "<p><br></p><p>x</p>",
  },
  "Enter x3 on an empty root, then type": {
    reply: "<p><br><br><br>z</p>",
    normal: "<p><br></p><p><br></p><p><br></p><p>z</p>",
  },
  "Enter only, on an empty root": {
    reply: "<p><br></p>",
    normal: "<p><br></p><p><br></p>",
  },
  "Backspace over a trailing break": {
    reply: "<p>Hello</p>",
    normal: "<p>Hello</p>",
  },
  "bullet single": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
  },
  "number single": {
    reply: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li></ol>",
    normal: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li></ol>",
  },
  "bullet from an empty root": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li></ul>",
  },
  "number from an empty root": {
    reply: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\">b</li></ol>",
    normal: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\">b</li></ol>",
  },
  "bullet: Enter on an empty item exits": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><p>z</p>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><p>z</p>",
  },
  "number: Enter on an empty item exits": {
    reply: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li></ol><p>z</p>",
    normal: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li></ol><p>z</p>",
  },
  "bullet 3 items": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li><li value=\"3\">c</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li><li value=\"3\">c</li></ul>",
  },
  "bullet toggled off": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "number toggled off": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "bullet to number": {
    reply: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li></ol>",
    normal: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li></ol>",
  },
  "number to bullet": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
  },
  "bullet two levels (Indent button)": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">c</li><li value=\"2\">d</li></ul></li></ul></li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">c</li><li value=\"2\">d</li></ul></li></ul></li></ul>",
  },
  "Enter on an empty nested item outdents": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul></li><li value=\"2\">z</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul></li><li value=\"2\">z</li></ul>",
  },
  "Enter x3 on a nested item exits the list": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul></li></ul><p>z</p>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul></li></ul><p>z</p>",
  },
  "Outdent button on a nested item": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li></ul>",
  },
  "Indent then Outdent on the first item": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
  },
  "Outdent on a top-level item": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
  },
  "number multi-level, back out": {
    reply: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ol class=\"editor-list-ol\"><li value=\"1\">b</li><li value=\"2\">c</li></ol></li><li value=\"2\">d</li><li value=\"3\">e</li></ol>",
    normal: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ol class=\"editor-list-ol\"><li value=\"1\">b</li><li value=\"2\">c</li></ol></li><li value=\"2\">d</li><li value=\"3\">e</li></ol>",
  },
  "bullet inside number": {
    reply: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li><li value=\"2\">c</li></ul></li></ol>",
    normal: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li><li value=\"2\">c</li></ul></li></ol>",
  },
  "number inside bullet": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ol class=\"editor-list-ol\"><li value=\"1\">b</li><li value=\"2\">c</li></ol></li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ol class=\"editor-list-ol\"><li value=\"1\">b</li><li value=\"2\">c</li></ol></li></ul>",
  },
  "toggle list off on a nested item": {
    reply: "<p>a</p><p>b</p>",
    normal: "<p>a</p><p>b</p>",
  },
  "br lines to bullets (all)": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li><li value=\"3\">c</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">b</li><li value=\"3\">c</li></ul>",
  },
  "br lines to bullets (middle line only)": {
    reply: "<p>a</p><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul><p>c</p>",
    normal: "<p>a</p><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul><p>c</p>",
  },
  "br lines to numbers, indent the middle": {
    reply: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ol class=\"editor-list-ol\"><li value=\"1\">b</li></ol></li><li value=\"2\">c</li></ol>",
    normal: "<ol class=\"editor-list-ol\"><li value=\"1\">a</li><li value=\"2\" class=\"editor-list-item-nested\"><ol class=\"editor-list-ol\"><li value=\"1\">b</li></ol></li><li value=\"2\">c</li></ol>",
  },
  "br lines with a blank to bullets": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">c</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\"></li><li value=\"3\">c</li></ul>",
  },
  "br lines with a trailing blank to bullets": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\"></li><li value=\"3\"></li></ul>",
  },
  "list, exit, then Enter makes a break": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><p>z<br>y</p>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><p>z</p><p>y</p>",
  },
  "bold item, Enter keeps bold": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\"><b><strong class=\"editor-text-bold\">b</strong></b></li><li value=\"3\"><b><strong class=\"editor-text-bold\">c</strong></b></li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\"><b><strong class=\"editor-text-bold\">b</strong></b></li><li value=\"3\"><b><strong class=\"editor-text-bold\">c</strong></b></li></ul>",
  },
  "bullet list then br lines to numbers (two lists)": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><ol class=\"editor-list-ol\"><li value=\"1\">x</li><li value=\"2\">y</li></ol>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><ol class=\"editor-list-ol\"><li value=\"1\">x</li><li value=\"2\">y</li></ol>",
  },
  "Enter in a list item with a range selected": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">d</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\">d</li></ul>",
  },
  "indent paragraph": {
    reply: "<p style=\"padding-inline-start: 40px;\">a</p>",
    normal: "<p style=\"padding-inline-start: 40px;\">a</p>",
  },
  "indent x2": {
    reply: "<p style=\"padding-inline-start: 80px;\">a</p>",
    normal: "<p style=\"padding-inline-start: 80px;\">a</p>",
  },
  "indent then outdent": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "outdent plain paragraph": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "indent ONE br line (that line only in the reply box)": {
    reply: "<p>a</p><p style=\"padding-inline-start: 40px;\">b</p><p>c</p>",
    normal: "<p>a</p><p style=\"padding-inline-start: 40px;\">b</p><p>c</p>",
  },
  "indent then Enter": {
    reply: "<p style=\"padding-inline-start: 40px;\">a<br>b</p>",
    normal: "<p style=\"padding-inline-start: 40px;\">a</p><p>b</p>",
  },
  "indent the first of two legacy paragraphs": {
    reply: "<p style=\"padding-inline-start: 40px;\">a</p><p>b</p>",
    normal: "<p style=\"padding-inline-start: 40px;\">a</p><p>b</p>",
  },
  "indent a heading": {
    reply: "<h2 style=\"padding-inline-start: 40px;\">a</h2>",
    normal: "<h2 style=\"padding-inline-start: 40px;\">a</h2>",
  },
  "indent a quote": {
    reply: "<blockquote style=\"padding-inline-start: 40px;\">a</blockquote>",
    normal: "<blockquote style=\"padding-inline-start: 40px;\">a</blockquote>",
  },
  "indent then list then outdent": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
  },
  "indent a code block (no-op)": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">a</pre>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">a</pre>",
  },
  "indent from an empty root": {
    reply: "<p style=\"padding-inline-start: 40px;\">x</p>",
    normal: "<p style=\"padding-inline-start: 40px;\">x</p>",
  },
  "indented paragraph then br line to bullet": {
    reply: "<p style=\"padding-inline-start: 40px;\">a</p><ul class=\"editor-list-ul\"><li value=\"1\" class=\"editor-list-item-nested\"><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul></li></ul>",
    normal: "<p style=\"padding-inline-start: 40px;\">a</p><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul>",
  },
  "code block: Enter x2 stays in the block": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">a<br><br>b</pre>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">a<br><br>b</pre>",
  },
  "code block: Enter x3 exits": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">a</pre><p>b</p>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">a</pre><p>b</p>",
  },
  "code block toggled off": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "code block: Shift+Enter": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">a<br>b</pre>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">a<br>b</pre>",
  },
  "code block from br lines (one block in the reply box)": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">a<br>b</pre>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">a</pre><pre class=\"editor-code\" spellcheck=\"false\">b</pre>",
  },
  "code block on the middle br line": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">a<br>b<br>c</pre>",
    normal: "<p>a</p><pre class=\"editor-code\" spellcheck=\"false\">b</pre><p>c</p>",
  },
  "code block between lists": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><pre class=\"editor-code\" spellcheck=\"false\">code</pre><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul><pre class=\"editor-code\" spellcheck=\"false\">code</pre><ul class=\"editor-list-ul\"><li value=\"1\">b</li></ul>",
  },
  "code block to heading": {
    reply: "<h2>a</h2>",
    normal: "<h2>a</h2>",
  },
  "inline code on a word": {
    reply: "<p>run <code spellcheck=\"false\"><span class=\"editor-text-code\">foo</span></code> now</p>",
    normal: "<p>run <code spellcheck=\"false\"><span class=\"editor-text-code\">foo</span></code> now</p>",
  },
  "inline code toggled off": {
    reply: "<p>run foo now</p>",
    normal: "<p>run foo now</p>",
  },
  "inline code across a br": {
    reply: "<p><code spellcheck=\"false\"><span class=\"editor-text-code\">aa</span></code><br><code spellcheck=\"false\"><span class=\"editor-text-code\">bb</span></code></p>",
    normal: "<p><code spellcheck=\"false\"><span class=\"editor-text-code\">aa</span></code></p><p><code spellcheck=\"false\"><span class=\"editor-text-code\">bb</span></code></p>",
  },
  "inline code, toggle off, break, text": {
    reply: "<p>x <code spellcheck=\"false\"><span class=\"editor-text-code\">cc</span></code><br>after</p>",
    normal: "<p>x <code spellcheck=\"false\"><span class=\"editor-text-code\">cc</span></code></p><p>after</p>",
  },
  "inline code in a list item": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\"><code spellcheck=\"false\"><span class=\"editor-text-code\">a</span></code></li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\"><code spellcheck=\"false\"><span class=\"editor-text-code\">a</span></code></li></ul>",
  },
  "inline code and bold on one word": {
    reply: "<p>a <b><code spellcheck=\"false\"><strong class=\"editor-text-bold editor-text-code\">foo</strong></code></b> b</p>",
    normal: "<p>a <b><code spellcheck=\"false\"><strong class=\"editor-text-bold editor-text-code\">foo</strong></code></b> b</p>",
  },
  "image alone": {
    reply: "<p><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"></p>",
    normal: "<p><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></p>",
  },
  "image then type": {
    reply: "<p><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\">after</p>",
    normal: "<p><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\">after</p>",
  },
  "Enter after an image": {
    reply: "<p>a<br><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"><br>z</p>",
    normal: "<p>a<img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></p><p>z</p>",
  },
  "Enter before an image": {
    reply: "<p><br><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"></p>",
    normal: "<p><br></p><p><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></p>",
  },
  "image in a list item": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a<br><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"></li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a<img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></li></ul>",
  },
  "image in a list item, then Enter": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a<br><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"></li><li value=\"2\">b</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a<img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></li><li value=\"2\">b</li></ul>",
  },
  "image between br lines": {
    reply: "<p>a<br><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"><br>b</p>",
    normal: "<p>a</p><p><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></p><p>b</p>",
  },
  "br lines with an image to bullets": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\"><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"></li><li value=\"3\">b</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li><li value=\"2\"><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></li><li value=\"3\">b</li></ul>",
  },
  "image then Align Center": {
    reply: "<p>a</p><p style=\"text-align: center;\"><img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"></p>",
    normal: "<p style=\"text-align: center;\">a<img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></p>",
  },
  "image in a code block": {
    reply: "<pre class=\"editor-code\" spellcheck=\"false\">a<img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;\"></pre>",
    normal: "<pre class=\"editor-code\" spellcheck=\"false\">a<img src=\"blob:https://csm.example.com/1\" alt=\"shot.png\" style=\"max-width: 100%; border-radius: 8px; display: block;\"></pre>",
  },
  "image then undo": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "bold underline strikethrough": {
    reply: "<p><u><s><b><strong class=\"editor-text-bold editor-text-strikethrough editor-text-underline\">abc</strong></b></s></u></p>",
    normal: "<p><u><s><b><strong class=\"editor-text-bold editor-text-strikethrough editor-text-underline\">abc</strong></b></s></u></p>",
  },
  "underline only": {
    reply: "<p><u><span class=\"editor-text-underline\">abc</span></u></p>",
    normal: "<p><u><span class=\"editor-text-underline\">abc</span></u></p>",
  },
  "strikethrough only": {
    reply: "<p><s><span class=\"editor-text-strikethrough\">abc</span></s></p>",
    normal: "<p><s><span class=\"editor-text-strikethrough\">abc</span></s></p>",
  },
  "italic on a word": {
    reply: "<p>a <i><em class=\"editor-text-italic\">bc</em></i> d</p>",
    normal: "<p>a <i><em class=\"editor-text-italic\">bc</em></i> d</p>",
  },
  "bold across a br": {
    reply: "<p><b><strong class=\"editor-text-bold\">aa</strong></b><br><b><strong class=\"editor-text-bold\">bb</strong></b></p>",
    normal: "<p><b><strong class=\"editor-text-bold\">aa</strong></b></p><p><b><strong class=\"editor-text-bold\">bb</strong></b></p>",
  },
  "bold across part of two lines": {
    reply: "<p><b><strong class=\"editor-text-bold\">aa</strong></b><br><b><strong class=\"editor-text-bold\">b</strong></b>b</p>",
    normal: "<p><b><strong class=\"editor-text-bold\">aa</strong></b></p><p><b><strong class=\"editor-text-bold\">b</strong></b>b</p>",
  },
  "bold toggled off again": {
    reply: "<p>abc</p>",
    normal: "<p>abc</p>",
  },
  "heading h2": {
    reply: "<h2>a</h2>",
    normal: "<h2>a</h2>",
  },
  "heading: Enter at end starts a paragraph": {
    reply: "<h1>a</h1><p>b</p>",
    normal: "<h1>a</h1><p>b</p>",
  },
  "heading: Enter in the middle splits it": {
    reply: "<h2>ab</h2><h2>cd</h2>",
    normal: "<h2>ab</h2><h2>cd</h2>",
  },
  "heading: Enter at start": {
    reply: "<p><br></p><h2>abcd</h2>",
    normal: "<p><br></p><h2>abcd</h2>",
  },
  "heading: Shift+Enter": {
    reply: "<h2>ab<br>c</h2>",
    normal: "<h2>ab<br>c</h2>",
  },
  "heading on one br line": {
    reply: "<p>a</p><h2>b</h2><p>c</p>",
    normal: "<p>a</p><h2>b</h2><p>c</p>",
  },
  "heading on all br lines": {
    reply: "<h3>a</h3><h3>b</h3>",
    normal: "<h3>a</h3><h3>b</h3>",
  },
  "heading on a line with a blank before it": {
    reply: "<p>a</p><p><br></p><h2>b</h2>",
    normal: "<p>a</p><p><br></p><h2>b</h2>",
  },
  "heading back to paragraph": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "heading, Enter x2, type": {
    reply: "<h2>a</h2><p><br>z</p>",
    normal: "<h2>a</h2><p><br></p><p>z</p>",
  },
  "quote": {
    reply: "<blockquote>a</blockquote>",
    normal: "<blockquote>a</blockquote>",
  },
  "quote: Enter x2 then type": {
    reply: "<blockquote>a</blockquote><p><br>b</p>",
    normal: "<blockquote>a</blockquote><p><br></p><p>b</p>",
  },
  "quote on one br line": {
    reply: "<blockquote>a<br>b<br>c</blockquote>",
    normal: "<p>a</p><blockquote>b</blockquote><p>c</p>",
  },
  "quote on all br lines": {
    reply: "<blockquote>a<br>b</blockquote>",
    normal: "<blockquote>a</blockquote><blockquote>b</blockquote>",
  },
  "quote back to paragraph": {
    reply: "<p>a</p>",
    normal: "<p>a</p>",
  },
  "quote then bullet": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
  },
  "alignment: Align Left": {
    reply: "<p style=\"text-align: left;\">a</p>",
    normal: "<p style=\"text-align: left;\">a</p>",
  },
  "alignment: Align Center": {
    reply: "<p style=\"text-align: center;\">a</p>",
    normal: "<p style=\"text-align: center;\">a</p>",
  },
  "alignment: Align Right": {
    reply: "<p style=\"text-align: right;\">a</p>",
    normal: "<p style=\"text-align: right;\">a</p>",
  },
  "alignment: Justify": {
    reply: "<p style=\"text-align: justify;\">a</p>",
    normal: "<p style=\"text-align: justify;\">a</p>",
  },
  "center ONE br line (that line only in the reply box)": {
    reply: "<p>a</p><p style=\"text-align: center;\">b</p><p>c</p>",
    normal: "<p>a</p><p style=\"text-align: center;\">b</p><p>c</p>",
  },
  "center then Enter then type": {
    reply: "<p style=\"text-align: center;\">a<br>b</p>",
    normal: "<p style=\"text-align: center;\">a</p><p style=\"text-align: center;\">b</p>",
  },
  "center then Enter x2 then type": {
    reply: "<p style=\"text-align: center;\">a<br><br>b</p>",
    normal: "<p style=\"text-align: center;\">a</p><p style=\"text-align: center;\"><br></p><p style=\"text-align: center;\">b</p>",
  },
  "center a list item": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\" style=\"text-align: center;\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\" style=\"text-align: center;\">a</li></ul>",
  },
  "center from an empty root": {
    reply: "<p style=\"text-align: center;\">a</p>",
    normal: "<p style=\"text-align: center;\">a</p>",
  },
  "center then left": {
    reply: "<p style=\"text-align: left;\">a</p>",
    normal: "<p style=\"text-align: left;\">a</p>",
  },
  "link on a selection": {
    reply: "<p>go <a href=\"https://example.com\" rel=\"noreferrer\" class=\"editor-link\">here</a> now</p>",
    normal: "<p>go <a href=\"https://example.com\" rel=\"noreferrer\" class=\"editor-link\">here</a> now</p>",
  },
  "link toggled off": {
    reply: "<p>go here now</p>",
    normal: "<p>go here now</p>",
  },
  "link with a javascript: URL is refused": {
    reply: "<p>go here</p>",
    normal: "<p>go here</p>",
  },
  "link on a br line": {
    reply: "<p>a<br><a href=\"https://example.com\" rel=\"noreferrer\" class=\"editor-link\">b</a><br>c</p>",
    normal: "<p>a</p><p><a href=\"https://example.com\" rel=\"noreferrer\" class=\"editor-link\">b</a></p><p>c</p>",
  },
  "link, caret after, Enter": {
    reply: "<p>go <a href=\"https://example.com\" rel=\"noreferrer\" class=\"editor-link\">here</a> now<br>z</p>",
    normal: "<p>go <a href=\"https://example.com\" rel=\"noreferrer\" class=\"editor-link\">here</a> now</p><p>z</p>",
  },
  "undo a list conversion of br lines": {
    reply: "<p>a<br>b</p>",
    normal: "<p>a</p><p>b</p>",
  },
  "undo then redo a list": {
    reply: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
    normal: "<ul class=\"editor-list-ul\"><li value=\"1\">a</li></ul>",
  },
  "undo a heading on a br line": {
    reply: "<p>a<br>b<br>c</p>",
    normal: "<p>a</p><p>b</p><p>c</p>",
  },
});
