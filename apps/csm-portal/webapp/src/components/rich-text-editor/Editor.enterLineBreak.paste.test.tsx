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
import Editor from "./Editor";
import { $getRoot, $isElementNode, $isTextNode, type LexicalEditor, type LexicalNode } from "lexical";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

// jsdom has no ClipboardEvent and no Range#getBoundingClientRect; same stand-ins
// as Editor.pasteFormatPrompt.test.tsx.
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
const TEXT = "Hi team,\n\nLine A\nLine B";

describe("Editor plain-text paste with enterInsertsLineBreak", () => {
  it("plain-text paste keeps single newlines and blank lines as line breaks in one paragraph", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} enterInsertsLineBreak onChange={onChange} />);
    paste(getEditable(), { "text/plain": TEXT });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(onChange.mock.calls.at(-1)?.[0]).toBe("<p>Hi team,<br><br>Line A<br>Line B</p>");
  });

  it("'Remove Formatting' does the same", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} enterInsertsLineBreak onChange={onChange} />);
    paste(getEditable(), { "text/html": "<p>x</p>", "text/plain": TEXT });
    fireEvent.click(await screen.findByRole("button", { name: "Remove Formatting" }));
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(onChange.mock.calls.at(-1)?.[0]).toBe("<p>Hi team,<br><br>Line A<br>Line B</p>");
  });

  it("editors without the flag keep the paragraph tokenizer", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
    paste(getEditable(), { "text/plain": TEXT });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(onChange.mock.calls.at(-1)?.[0]).toBe("<p>Hi team,</p><p>Line A</p><p>Line B</p>");
  });

  /** Puts the caret at the end of the first text node under the first block of `type`. */
  function caretIn(type: string): void {
    const editor = (getEditable() as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;
    act(() => {
      editor.update(
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

  it("in a list item keeps paragraph tokens: three lines are three items", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} enterInsertsLineBreak value="<ul><li>x</li></ul>" onChange={onChange} />);
    caretIn("listitem");
    paste(getEditable(), { "text/plain": "a\nb\nc" });
    await waitFor(() => expect((onChange.mock.calls.at(-1)?.[0].match(/<li/g) ?? []).length).toBe(3));
    expect(onChange.mock.calls.at(-1)?.[0]).not.toContain("<br>");
  });

  it("in a heading keeps paragraph tokens (no <br> inside the heading)", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} enterInsertsLineBreak value="<h2>T</h2>" onChange={onChange} />);
    caretIn("heading");
    paste(getEditable(), { "text/plain": "a\nb" });
    await waitFor(() => expect(onChange.mock.calls.at(-1)?.[0]).toContain("b"));
    expect(onChange.mock.calls.at(-1)?.[0]).not.toContain("<br>");
  });

  it("'Remove Formatting' in a list item also keeps paragraph tokens", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} enterInsertsLineBreak value="<ul><li>x</li></ul>" onChange={onChange} />);
    caretIn("listitem");
    paste(getEditable(), { "text/html": "<p>x</p>", "text/plain": "a\nb\nc" });
    fireEvent.click(await screen.findByRole("button", { name: "Remove Formatting" }));
    await waitFor(() => expect((onChange.mock.calls.at(-1)?.[0].match(/<li/g) ?? []).length).toBe(3));
  });

  it("in a table cell uses Lexical's default (no <br>)", async () => {
    const onChange = vi.fn();
    render(
      <Editor autoFocus showToolbar={false} enterInsertsLineBreak value="<table><tbody><tr><td><p>x</p></td></tr></tbody></table>" onChange={onChange} />,
    );
    caretIn("tablecell");
    paste(getEditable(), { "text/plain": "a\nb" });
    await waitFor(() => expect(onChange.mock.calls.at(-1)?.[0]).toContain("b"));
    expect(onChange.mock.calls.at(-1)?.[0]).not.toContain("<br>");
  });
});
