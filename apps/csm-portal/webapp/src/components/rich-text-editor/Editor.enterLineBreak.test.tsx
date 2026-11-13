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
  $isElementNode,
  $isTextNode,
  KEY_ENTER_COMMAND,
  type LexicalEditor,
  type LexicalNode,
} from "lexical";
import Editor from "./Editor";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn(), clearError: vi.fn() }),
}));

const getEditable = () => screen.getByTestId("case-description-editor");
const getLexical = () =>
  (getEditable() as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;

/** Puts the caret at the end of the last text node under the first node matching `where`. */
function placeCaret(where: (node: LexicalNode) => boolean): void {
  const editor = getLexical();
  act(() => {
    editor.update(
      () => {
        const find = (node: LexicalNode): LexicalNode | null => {
          if (where(node)) return node;
          if ($isElementNode(node)) {
            for (const child of node.getChildren()) {
              const hit = find(child);
              if (hit) return hit;
            }
          }
          return null;
        };
        const block = find($getRoot());
        if (!block || !$isElementNode(block)) throw new Error("no block to select");
        const last = block.getLastDescendant();
        if (last && $isTextNode(last)) last.select();
        else block.selectEnd();
      },
      { discrete: true },
    );
  });
}

const isNamed = (type: string) => (node: LexicalNode) => node.getType() === type;
const inLast = isNamed;

/** Sends an Enter keydown the way the browser would, through the contenteditable. */
function pressEnter(init: KeyboardEventInit = {}): KeyboardEvent {
  const event = new KeyboardEvent("keydown", {
    key: "Enter",
    code: "Enter",
    keyCode: 13,
    bubbles: true,
    cancelable: true,
    ...init,
  } as KeyboardEventInit);
  act(() => {
    getEditable().dispatchEvent(event);
  });
  return event;
}

function renderEditor(
  props: Partial<React.ComponentProps<typeof Editor>> = {},
  value = "<p>Hi team,</p>",
) {
  const onChange = vi.fn();
  render(
    <Editor
      showToolbar={false}
      value={value}
      onChange={onChange}
      {...props}
    />,
  );
  const html = () => (onChange.mock.calls.at(-1)?.[0] ?? "") as string;
  return { onChange, html };
}

describe("Editor enterInsertsLineBreak", () => {
  it("Enter inserts a <br> inside the paragraph instead of a new paragraph", async () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true });
    placeCaret(inLast("paragraph"));
    const event = pressEnter();
    expect(event.defaultPrevented).toBe(true);
    await waitFor(() => expect(html()).toContain("<br>"));
    expect(html()).toBe("<p>Hi team,<br></p>");
  });

  it("Enter twice makes one blank line (<br><br>), still one paragraph", async () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true });
    placeCaret(inLast("paragraph"));
    pressEnter();
    pressEnter();
    await waitFor(() => expect(html()).toBe("<p>Hi team,<br><br></p>"));
  });

  it("Shift+Enter still inserts a <br>", async () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true });
    placeCaret(inLast("paragraph"));
    pressEnter({ shiftKey: true });
    await waitFor(() => expect(html()).toBe("<p>Hi team,<br></p>"));
  });

  it("keeps Lexical's default Enter in a list (new item)", async () => {
    const { html } = renderEditor(
      { enterInsertsLineBreak: true },
      "<ul><li>one</li></ul>",
    );
    placeCaret(isNamed("listitem"));
    pressEnter();
    await waitFor(() => expect((html().match(/<li/g) ?? []).length).toBe(2));
    expect(html()).not.toContain("<br>");
  });

  it("keeps Lexical's default Enter in a heading (exits to a new paragraph)", async () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true }, "<h2>Title</h2>");
    placeCaret(isNamed("heading"));
    pressEnter();
    await waitFor(() => expect(html()).toMatch(/<h2[^>]*>.*Title.*<\/h2><p/));
    expect(html()).not.toContain("<br><br>");
  });

  it("keeps Lexical's default Enter in a code block (stays one code block)", async () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true }, "<pre>let a = 1;</pre>");
    placeCaret(isNamed("code"));
    const event = pressEnter();
    // Not taken by the line-break plugin; Lexical's code handler inserts its own newline.
    await waitFor(() => expect(html()).toContain("<br>"));
    expect(html()).toMatch(/^<pre/);
    expect(html()).not.toMatch(/<p[ >]/);
    expect(event.defaultPrevented).toBe(true);
  });

  it("does not take Ctrl/Cmd+Enter: it submits and inserts no break", async () => {
    const onSubmitKeyDown = vi.fn();
    const { html } = renderEditor({ enterInsertsLineBreak: true, onSubmitKeyDown });
    placeCaret(inLast("paragraph"));
    pressEnter({ ctrlKey: true });
    pressEnter({ metaKey: true });
    expect(onSubmitKeyDown).toHaveBeenCalledTimes(2);
    expect(html()).not.toContain("<br>");
    expect(getLexical().getEditorState().read(() => $getRoot().getTextContent())).toBe("Hi team,");
  });

  it("still inserts the break when no submit handler is set", async () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true });
    placeCaret(inLast("paragraph"));
    pressEnter();
    await waitFor(() => expect(html()).toContain("<br>"));
  });

  it("does not interfere with an IME composition", () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true });
    placeCaret(inLast("paragraph"));
    const event = new KeyboardEvent("keydown", { key: "Enter", isComposing: true, cancelable: true });
    act(() => {
      getLexical().dispatchCommand(KEY_ENTER_COMMAND, event);
    });
    // Our handler stood aside: Lexical's own default ran (a paragraph, no <br>).
    expect(html()).not.toContain("<br>");
  });

  it("replaces a selected range with the break, as Lexical does", async () => {
    const { html } = renderEditor({ enterInsertsLineBreak: true }, "<p>Hello world</p>");
    const editor = getLexical();
    act(() => {
      editor.update(
        () => {
          const text = $getRoot().getFirstDescendant();
          if (!text || !$isTextNode(text)) throw new Error("no text");
          text.select(5, 11);
        },
        { discrete: true },
      );
    });
    pressEnter();
    await waitFor(() => expect(html()).toBe("<p>Hello<br></p>"));
  });

  it("is a no-op for editors without the flag: Enter still makes a paragraph", async () => {
    const { html } = renderEditor({});
    placeCaret(inLast("paragraph"));
    pressEnter();
    await waitFor(() => expect((html().match(/<p/g) ?? []).length).toBe(2));
    expect(html()).not.toContain("<br><br>");
  });

  it("ignores the flag when enterToSubmit is on (Enter submits)", () => {
    const onSubmitKeyDown = vi.fn();
    const { html } = renderEditor({
      enterInsertsLineBreak: true,
      enterToSubmit: true,
      onSubmitKeyDown,
    });
    placeCaret(inLast("paragraph"));
    pressEnter();
    expect(onSubmitKeyDown).toHaveBeenCalledTimes(1);
    expect(html()).not.toContain("<br>");
  });

  it("keeps Lexical's default Enter in a table cell (no <br> from our plugin)", async () => {
    const { html } = renderEditor(
      { enterInsertsLineBreak: true },
      "<table><tbody><tr><td><p>x</p></td></tr></tbody></table>",
    );
    placeCaret(isNamed("tablecell"));
    pressEnter();
    await waitFor(() => expect((html().match(/<p[ >]/g) ?? []).length).toBe(2));
    // A new (empty) paragraph, not "x<br>" in the same one.
    expect(html()).not.toContain("x<br>");
  });
});
