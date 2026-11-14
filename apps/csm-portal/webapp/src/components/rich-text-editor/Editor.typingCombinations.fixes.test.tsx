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


// Regression tests for typing / toolbar bugs found by the combination run
// (Editor.typingCombinations.test.tsx has the frozen output of every scenario).

import { afterEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, act } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { $getRoot } from "lexical";
import { CONFIGS, cleanup, mount, type Config, type Ctx } from "./typingHarness";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn(), clearError: vi.fn() }),
}));

afterEach(cleanup);

const both = (name: string, run: (c: Ctx, cfg: Config) => void | Promise<void>, value = ""): void => {
  describe(name, () => {
    for (const cfg of CONFIGS) {
      it(cfg, async () => {
        const c = mount(cfg, value);
        await run(c, cfg);
      });
    }
  });
};
const reply = (name: string, run: (c: Ctx) => void | Promise<void>, value = ""): void => {
  it(`${name} (reply box)`, async () => {
    await run(mount("reply", value));
  });
};
const settle = () => act(async () => { await new Promise((r) => setTimeout(r, 0)); });
const LIST = (items: string) => `<ul class="editor-list-ul">${items}</ul>`;

// Font variant -> Quote / Normal over two or more blocks used to do nothing: `$patchStyleText`
// ran on the selection from before `$setBlocksType` replaced the nodes and threw.
both("Quote over two paragraphs applies to both", async (c) => {
  c.selectAll();
  await c.block("quote");
  await settle();
  expect(c.html()).toBe("<blockquote>a</blockquote><blockquote>b</blockquote>");
}, "<p>a</p><p>b</p>");

both("Normal over two headings applies to both", async (c) => {
  c.selectAll();
  await c.block("paragraph");
  await settle();
  expect(c.html()).not.toContain("<h2");
}, "<h2>a</h2><h2>b</h2>");

both("Quote over a paragraph and a heading", async (c) => {
  c.selectAll();
  await c.block("quote");
  await settle();
  expect(c.html()).toBe("<blockquote>a</blockquote><blockquote>b</blockquote>");
}, "<p>a</p><h2>b</h2>");

// The empty editor in Chrome is a collapsed element selection on the root: the block
// conversions used to leave the caret there and the next typed text went into a new
// paragraph above the converted block.
for (const [label, run, tag] of [
  ["Heading", (c: Ctx) => c.block("h2"), "h2"],
  ["Quote", (c: Ctx) => c.block("quote"), "blockquote"],
  ["Code Block", (c: Ctx) => c.click("Code Block"), "pre"],
] as const) {
  both(`${label} on a truly empty editor, then typing, types into the block`, async (c) => {
    c.root();
    await run(c);
    c.type("x");
    await settle();
    expect(c.html()).toMatch(new RegExp(`^<${tag}[^>]*>x</${tag}>$`));
  });
}

// Normal editor: Enter on the root caret acts like Enter in an empty paragraph.
it("normal editor: Enter on a truly empty editor then typing writes on the 2nd line", async () => {
  const c = mount("normal");
  c.root();
  c.enter();
  c.type("x");
  await settle();
  expect(c.html()).toBe("<p><br></p><p>x</p>");
});

it("normal editor: Enter x3 on a truly empty editor then typing writes on the 4th line", async () => {
  const c = mount("normal");
  c.root();
  c.enter(3);
  c.type("z");
  await settle();
  expect(c.html()).toBe("<p><br></p><p><br></p><p><br></p><p>z</p>");
});

// The Link button read the selection outside an editor read and threw, so the Text field
// was never pre-filled with the selected text.
both("Link button pre-fills the text field and does not throw", async (c) => {
  c.root();
  c.type("go here now");
  c.select("here");
  const errors: string[] = [];
  const onError = (event: ErrorEvent) => { errors.push(event.message); event.preventDefault(); };
  window.addEventListener("error", onError);
  try {
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Link" }));
    });
    await settle();
  } finally {
    window.removeEventListener("error", onError);
  }
  expect(errors).toEqual([]);
  expect(screen.getByLabelText("Text (Optional)")).toHaveValue("here");
});

// Reply box: the blank line typed before a converted line is kept as an empty paragraph,
// not a trailing <br> (the editor shows that as a blank line, a browser does not).
for (const [label, run, after] of [
  ["Heading", (c: Ctx) => c.block("h2"), "<h2>b</h2>"],
  ["Bullet List", (c: Ctx) => c.click("Bullet List"), LIST('<li value="1">b</li>')],
] as const) {
  reply(`${label} on the line after a blank line keeps the blank line`, async (c) => {
    c.root();
    c.type("a");
    c.enter(2);
    c.type("b");
    c.select("b");
    await run(c);
    await settle();
    expect(c.html()).toBe(`<p>a</p><p><br></p>${after}`);
  });
}

reply("Bullet List straight after 'Steps:' Enter Enter keeps the blank line", async (c) => {
  c.root();
  c.type("Steps:");
  c.enter(2);
  await c.click("Bullet List");
  expect(c.html()).toBe(`<p>Steps:</p><p><br></p>${LIST('<li value="1"></li>')}`);
});

reply("two blank lines before a converted line become two empty paragraphs", async (c) => {
  c.root();
  c.type("a");
  c.enter(3);
  c.type("b");
  c.select("b");
  await c.block("h2");
  expect(c.html()).toBe("<p>a</p><p><br></p><p><br></p><h2>b</h2>");
});

reply("a conversion with a blank line before it is one undo step", async (c) => {
  c.root();
  c.type("a");
  c.enter(2);
  c.type("b");
  c.select("b");
  await c.block("h2");
  await c.undo();
  expect(c.html()).toBe("<p>a<br><br>b</p>");
});

// Reply box: images are inline-block, so one Enter after an image starts the next line
// directly (with a block image the `<br>` showed, and was exported, as a blank line).
const IMG = '<img src="blob:https://csm.example.com/1" alt="shot.png" style="max-width: 100%; border-radius: 8px; display: inline-block; vertical-align: bottom;">';

reply("an image after text starts on its own line", async (c) => {
  c.root();
  c.type("a");
  await c.upload();
  await settle();
  expect(c.html()).toBe(`<p>a<br>${IMG}</p>`);
});

reply("one Enter after an image starts the next line directly", async (c) => {
  c.root();
  await c.upload();
  c.enter();
  c.type("z");
  await settle();
  expect(c.html()).toBe(`<p>${IMG}<br>z</p>`);
});

reply("Enter twice after an image is one blank line", async (c) => {
  c.root();
  await c.upload();
  c.enter(2);
  c.type("z");
  await settle();
  expect(c.html()).toBe(`<p>${IMG}<br><br>z</p>`);
});

reply("an image after a line break stays on that line", async (c) => {
  c.root();
  c.type("a");
  c.enter();
  await c.upload();
  await settle();
  expect(c.html()).toBe(`<p>a<br>${IMG}</p>`);
});

it("normal editor: images stay display:block", async () => {
  const c = mount("normal");
  c.root();
  c.type("a");
  await c.upload();
  await settle();
  expect(c.html()).toContain("display: block;");
  expect(c.html()).not.toContain("inline-block");
});

both("inserting an image into an empty editor leaves no trailing empty paragraph", async (c) => {
  c.root();
  await c.upload();
  expect(c.html()).toMatch(/^<p><img[^>]*><\/p>$/);
});

// Reply box: Indent / Outdent and alignment apply to the selected lines only.
reply("Indent applies to the selected line only, and is one undo step", async (c) => {
  c.root();
  c.type("a");
  c.enter();
  c.type("b");
  c.enter();
  c.type("c");
  c.select("b");
  await c.click("Indent");
  expect(c.html()).toBe('<p>a</p><p style="padding-inline-start: 40px;">b</p><p>c</p>');
  await c.undo();
  expect(c.html()).toBe("<p>a<br>b<br>c</p>");
});

reply("Outdent applies to the selected line only", async (c) => {
  c.root();
  c.type("a");
  c.enter();
  c.type("b");
  c.selectAll();
  await c.click("Indent");
  expect(c.html()).toBe('<p style="padding-inline-start: 40px;">a</p><p style="padding-inline-start: 40px;">b</p>');
  c.select("b");
  await c.click("Outdent");
  expect(c.html()).toBe('<p style="padding-inline-start: 40px;">a</p><p>b</p>');
});

reply("Align Right applies to the selected lines, blank lines kept", async (c) => {
  c.root();
  c.type("a");
  c.enter(2);
  c.type("b");
  c.selectAll();
  await c.click("Align Right");
  expect(c.html()).toBe(
    '<p style="text-align: right;">a</p><p style="text-align: right;"><br></p><p style="text-align: right;">b</p>',
  );
});

it("normal editor: Indent still applies to the whole paragraph", async () => {
  const c = mount("normal", "<p>a</p><p>b</p>");
  c.select("a");
  await c.click("Indent");
  expect(c.html()).toBe('<p style="padding-inline-start: 40px;">a</p><p>b</p>');
});

// Tab in a list nests, Shift+Tab un-nests; outside a list Tab keeps its default (focus moves).
const twoItems = async (c: Ctx) => {
  c.root();
  c.type("a");
  await c.click("Bullet List");
  c.enter();
  c.type("b");
};
const NESTED = LIST(
  '<li value="1">a</li><li value="2" class="editor-list-item-nested">' + LIST('<li value="1">b</li>') + "</li>",
);

both("Tab in a list item nests it and is consumed", async (c) => {
  await twoItems(c);
  const event = c.tab();
  await settle();
  expect(event.defaultPrevented).toBe(true);
  expect(c.html()).toBe(NESTED);
});

both("Shift+Tab un-nests a nested item", async (c) => {
  await twoItems(c);
  c.tab();
  const event = c.shiftTab();
  await settle();
  expect(event.defaultPrevented).toBe(true);
  expect(c.html()).toBe(LIST('<li value="1">a</li><li value="2">b</li>'));
});

both("Shift+Tab on a top-level item is left to the browser (no focus trap)", async (c) => {
  await twoItems(c);
  const event = c.shiftTab();
  expect(event.defaultPrevented).toBe(false);
});

both("Tab on the first item of a list is left to the browser (cannot nest)", async (c) => {
  c.root();
  c.type("a");
  await c.click("Bullet List");
  expect(c.tab().defaultPrevented).toBe(false);
});

both("repeated Tab nests once, then focus moves on", async (c) => {
  await twoItems(c);
  expect(c.tab().defaultPrevented).toBe(true);
  await settle();
  expect(c.tab().defaultPrevented).toBe(false);
});

both("after Escape the next Tab and Shift+Tab are never handled", async (c) => {
  await twoItems(c);
  c.key({ key: "Escape", keyCode: 27 });
  expect(c.tab().defaultPrevented).toBe(false);
  await settle();
  expect(c.html()).toBe(LIST('<li value="1">a</li><li value="2">b</li>'));
  // The hatch is used up: the next Tab nests again.
  expect(c.tab().defaultPrevented).toBe(true);
  await settle();
  c.key({ key: "Escape", keyCode: 27 });
  expect(c.shiftTab().defaultPrevented).toBe(false);
});

both("another key after Escape re-arms Tab", async (c) => {
  await twoItems(c);
  c.key({ key: "Escape", keyCode: 27 });
  c.key({ key: "ArrowLeft", keyCode: 37 });
  expect(c.tab().defaultPrevented).toBe(true);
});

both("a click after Escape re-arms Tab", async (c) => {
  await twoItems(c);
  c.key({ key: "Escape", keyCode: 27 });
  fireEvent.click(c.editable());
  expect(c.tab().defaultPrevented).toBe(true);
});

both("Tab outside a list is left to the browser (no focus trap)", (c) => {
  c.root();
  c.type("a");
  expect(c.tab().defaultPrevented).toBe(false);
  expect(c.shiftTab().defaultPrevented).toBe(false);
});

both("Tab in a code block keeps the code block's own handling", async (c) => {
  c.root();
  c.type("a");
  await c.click("Code Block");
  c.tab();
  expect(c.html()).toMatch(/^<pre/);
  expect(c.html()).not.toContain("<ul");
});

// The normal editor's root-caret normalisation must not run when Enter is a submit: it would
// add a paragraph and fire onChange before the submit.
it("normal editor: Ctrl+Enter on an empty editor submits without touching the document", async () => {
  const c = mount("normal");
  c.root();
  await settle();
  const before = c.onChange.mock.calls.length;
  const tree = c.tree();
  c.ctrlEnter();
  await settle();
  expect(c.onSubmit).toHaveBeenCalledTimes(1);
  expect(c.tree()).toBe(tree);
  expect(c.onChange.mock.calls.length).toBe(before);
});

it("normal editor with enterToSubmit: plain Enter on an empty editor submits without touching the document", async () => {
  const c = mount("normal", "", { enterToSubmit: true });
  c.root();
  await settle();
  const before = c.onChange.mock.calls.length;
  const tree = c.tree();
  c.enter();
  await settle();
  expect(c.onSubmit).toHaveBeenCalledTimes(1);
  expect(c.tree()).toBe(tree);
  expect(c.onChange.mock.calls.length).toBe(before);
});

both("the exported nested-list wrapper carries list-style-type: none", async (c) => {
  await twoItems(c);
  c.tab();
  await settle();
  expect(c.rawHtml()).toContain('<li value="2" class="editor-list-item-nested" style="list-style-type: none">');
});

// Real Chrome: a click into an empty editor leaves a collapsed element selection on the root
// (with the default empty paragraph present, or on a root with no child at all). Each toolbar
// button must act on it and the next typed text must land in the result.
const rootStates: [string, (c: Ctx) => void][] = [
  ["root caret, default empty paragraph", (c) => c.root()],
  ["root caret after the default paragraph", (c) => c.update(() => { $getRoot().select(1, 1); })],
  ["root caret, no children at all", (c) => c.update(() => { $getRoot().clear(); $getRoot().select(0, 0); })],
];
for (const [state, setup] of rootStates) {
  for (const [button, expected] of [
    ["Bullet List", LIST('<li value="1">one</li>')],
    ["Numbered List", '<ol class="editor-list-ol"><li value="1">one</li></ol>'],
    ["Indent", '<p style="padding-inline-start: 40px;">one</p>'],
    ["Align Center", '<p style="text-align: center;">one</p>'],
  ] as const) {
    both(`empty editor (${state}): ${button}, then typing`, async (c) => {
      setup(c);
      await c.click(button);
      c.type("one");
      await settle();
      expect(c.html()).toBe(expected);
    });
  }
  reply(`empty reply box (${state}): Bullet List, type, Enter, type nests a new item`, async (c) => {
    setup(c);
    await c.click("Bullet List");
    c.type("one");
    c.enter();
    c.type("two");
    await settle();
    expect(c.html()).toBe(LIST('<li value="1">one</li><li value="2">two</li>'));
  });
}

// Draft restore: `$generateNodesFromDOM` ignores a `<br>` that is the last child of a block, so
// `<p>Hi<br></p>` (Enter at the end of a line) came back as `<p>Hi</p>` and typing joined the lines.
describe("draft restore keeps editor-authored trailing line breaks", () => {
  for (const cfg of CONFIGS) {
    for (const html of ["<p>Hi<br></p>", "<p>Hi<br><br></p>", "<p>a<br>b<br></p>"]) {
      it(`[${cfg}] ${html} exports unchanged`, async () => {
        const c = mount(cfg, html);
        await settle();
        expect(c.html()).toBe(html);
      });
    }
    it(`[${cfg}] content with no trailing break gains none`, async () => {
      const c = mount(cfg, "<p>Hi</p><p><br></p><p>a<br>b</p>");
      await settle();
      expect(c.html()).toBe("<p>Hi</p><p><br></p><p>a<br>b</p>");
    });
  }
  it("typing after restoring `Hi` + Enter lands on the new line (reply box)", async () => {
    const typed = mount("reply", "<p>Hi</p>");
    typed.end();
    typed.enter();
    await settle();
    const saved = typed.html();
    expect(saved).toBe("<p>Hi<br></p>");
    cleanup();
    const c = mount("reply", saved);
    await settle();
    c.end();
    c.type("x");
    await settle();
    expect(c.html()).toBe("<p>Hi<br>x</p>");
  });
});

// An existing empty paragraph (HTML import of `<p><br></p>` has no LineBreakNode) inside the
// selection was counted as kept content, so Bullet/Numbered List left an empty item for it.
describe("reply box: list conversion drops empty paragraphs of the selection, like blank <br> lines", () => {
  const ITEMS = (...texts: string[]) =>
    `<ul class="editor-list-ul">${texts.map((t, i) => `<li value="${i + 1}">${t}</li>`).join("")}</ul>`;
  reply("`<p>a</p><p><br></p><p>b</p>` + select all + Bullet List is two items", async (c) => {
    c.selectAll();
    await c.click("Bullet List");
    await settle();
    expect(c.html()).toBe(ITEMS("a", "b"));
  }, "<p>a</p><p><br></p><p>b</p>");
  reply("several empty paragraphs between lines are all dropped (Numbered List)", async (c) => {
    c.selectAll();
    await c.click("Numbered List");
    await settle();
    expect(c.html()).toBe('<ol class="editor-list-ol"><li value="1">a</li><li value="2">b</li></ol>');
  }, "<p>a</p><p><br></p><p><br></p><p>b</p>");
  reply("an empty paragraph mixed with a `<br>` blank line is dropped too", async (c) => {
    c.selectAll();
    await c.click("Bullet List");
    await settle();
    expect(c.html()).toBe(ITEMS("a", "b", "c"));
  }, "<p>a</p><p><br></p><p>b<br><br>c</p>");
  reply("every selected line blank: one empty item remains", async (c) => {
    c.selectAll();
    await c.click("Bullet List");
    await settle();
    expect((c.html().match(/<li/g) ?? []).length).toBe(1);
  }, "<p><br></p><p><br></p>");
  it("normal editor (flag off): the empty paragraph becomes an empty item as before", async () => {
    const c = mount("normal", "<p>a</p><p><br></p><p>b</p>");
    c.selectAll();
    await c.click("Bullet List");
    await settle();
    expect((c.html().match(/<li/g) ?? []).length).toBe(3);
  });
  reply("a heading conversion keeps the empty paragraph (blank lines are dropped for lists only)", async (c) => {
    c.selectAll();
    await c.block("h2");
    await settle();
    expect(c.html()).toBe("<h2>a</h2><h2><br></h2><h2>b</h2>");
  }, "<p>a</p><p><br></p><p>b</p>");
});
