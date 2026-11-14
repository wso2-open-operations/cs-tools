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

// Clicking into an EMPTY editor in a real browser leaves the selection as an element
// point on the root, not inside a paragraph. These tests set that state first.

import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { describe, expect, it, vi, beforeAll } from "vitest";
import "@testing-library/jest-dom/vitest";
import { $getRoot, $getSelection, $isRangeSelection, $setSelection, type LexicalEditor } from "lexical";
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

/** The selection a click into an empty editor leaves in Chrome: (root, 0), element type. */
function selectRootStart(): void {
  act(() => {
    getLexical().update(() => { $getRoot().select(0, 0); }, { discrete: true });
  });
}

function clearSelection(): void {
  act(() => {
    getLexical().update(() => { $setSelection(null); }, { discrete: true });
  });
}

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

function pressEnter(): void {
  act(() => {
    getEditable().dispatchEvent(
      new KeyboardEvent("keydown", { key: "Enter", code: "Enter", keyCode: 13, bubbles: true, cancelable: true }),
    );
  });
}

function setup(props: Partial<React.ComponentProps<typeof Editor>> = {}, value?: string) {
  const onChange = vi.fn();
  render(<Editor autoFocus showToolbar={false} onChange={onChange} value={value} {...props} />);
  return () => (onChange.mock.calls.at(-1)?.[0] ?? "") as string;
}

const STARTS = [
  ["an empty root", undefined],
  ["a root holding one empty paragraph", "<p><br></p>"],
] as const;

describe.each(STARTS)("with the caret on the root of %s", (_name, value) => {
  it("plain-text paste uses line breaks (enterInsertsLineBreak)", async () => {
    const html = setup({ enterInsertsLineBreak: true }, value);
    selectRootStart();
    paste({ "text/plain": "Hi team,\nLine two\n\nPara two" });
    await waitFor(() => expect(html()).toBe("<p>Hi team,<br>Line two<br><br>Para two</p>"));
  });

  it("Enter then typing gives one paragraph with a leading break", async () => {
    const html = setup({ enterInsertsLineBreak: true }, value);
    selectRootStart();
    pressEnter();
    act(() => {
      getLexical().update(
        () => {
          const selection = $getSelection();
          if ($isRangeSelection(selection)) selection.insertText("x");
        },
        { discrete: true },
      );
    });
    await waitFor(() => expect(html()).toContain("x"));
    expect(html()).toBe("<p><br>x</p>");
  });

  it("Paste as Markdown merges paragraphs and leaves no stray empty paragraph", async () => {
    const html = setup({ enterInsertsLineBreak: true, markdownPaste: true }, value);
    selectRootStart();
    paste({ "text/plain": "## Steps\n\n1. Stop it\n2. Start it\n\nDone **now**.\n\nSecond para" });
    fireEvent.click(await screen.findByRole("button", { name: "Paste as Markdown" }));
    await waitFor(() => expect(html()).toContain("<h2"));
    expect(html()).toMatch(/<\/ol><p>Done <b><strong[^>]*>now<\/strong><\/b>\.<br><br>Second para<\/p>$/);
    expect(html()).not.toContain("<p><br></p>");
  });

  it("Keep Formatting leaves no stray empty paragraph", async () => {
    const html = setup({ markdownPaste: true }, value);
    selectRootStart();
    paste({ "text/html": "<h2>Title</h2><p>Body</p>", "text/plain": "Title\nBody" });
    fireEvent.click(await screen.findByRole("button", { name: "Keep Formatting" }));
    await waitFor(() => expect(html()).toContain("<h2"));
    expect(html()).not.toContain("<p><br></p>");
  });

  it("Remove Formatting uses line breaks", async () => {
    const html = setup({ enterInsertsLineBreak: true, markdownPaste: true }, value);
    selectRootStart();
    paste({ "text/plain": "# T\n\n**b**" });
    fireEvent.click(await screen.findByRole("button", { name: "Remove Formatting" }));
    await waitFor(() => expect(html()).toBe("<p># T<br><br>**b**</p>"));
  });

  it("without the flags a plain-text paste still makes paragraphs", async () => {
    const html = setup({}, value);
    selectRootStart();
    paste({ "text/plain": "a\nb" });
    await waitFor(() => expect(html()).toContain("b"));
    expect(html()).toMatch(/^<p>a<\/p><p>b<\/p>/);
  });
});

describe("a paste held when the editor has no selection", () => {
  it.each([
    ["Paste as Markdown", { "text/plain": "# Title" }, "<h1"],
    ["Keep Formatting", { "text/html": "<h2>Title</h2>", "text/plain": "Title" }, "<h2"],
    ["Remove Formatting", { "text/plain": "# Title" }, "# Title"],
  ])("%s still inserts", async (button, data, expected) => {
    const html = setup({ markdownPaste: true });
    paste(data);
    await screen.findByRole("button", { name: button });
    clearSelection();
    fireEvent.click(screen.getByRole("button", { name: button }));
    await waitFor(() => expect(html()).toContain(expected));
  });
});
