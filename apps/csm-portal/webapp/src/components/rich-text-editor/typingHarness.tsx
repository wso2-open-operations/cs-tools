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

// Shared harness for the Editor.typingCombinations*.test.tsx files (test-only).

import { render, screen, fireEvent, act, waitFor, cleanup } from "@testing-library/react";
import {
  $createTabNode,
  $getRoot,
  $getSelection,
  $isElementNode,
  $isRangeSelection,
  $isTextNode,
  $createRangeSelection,
  $setSelection,
  type LexicalEditor,
  type LexicalNode,
  type TextNode,
} from "lexical";
import Editor from "./Editor";

const rect = () => ({ x: 0, y: 0, width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0 }) as DOMRect;
if (!Range.prototype.getBoundingClientRect) Range.prototype.getBoundingClientRect = rect;
if (!Range.prototype.getClientRects) Range.prototype.getClientRects = () => [] as unknown as DOMRectList;

export type Config = "reply" | "normal";
export const CONFIGS: Config[] = ["reply", "normal"];

export const png = (name = "shot.png") => new File([new Uint8Array(4)], name, { type: "image/png" });

export interface Ctx {
  html: () => string;
  /** The exported HTML exactly as sent (html() strips the nested-list marker style). */
  rawHtml: () => string;
  editable: () => HTMLElement;
  lexical: () => LexicalEditor;
  /** Chrome's empty-editor selection: element point (root, 0). */
  root: () => void;
  start: () => void;
  end: () => void;
  /** Caret just after the first occurrence of `text` (or before, with before=true). */
  after: (text: string) => void;
  before: (text: string) => void;
  /** Selects the first occurrence of `text` inside one text node. */
  select: (text: string) => void;
  /** Selects from the start of the first text to the end of the last text. */
  selectAll: () => void;
  /** Select from `from` text start to `to` text end (across nodes). */
  selectBetween: (from: string, to: string) => void;
  type: (text: string) => void;
  key: (init: KeyboardEventInit & { key: string; keyCode: number }) => KeyboardEvent;
  enter: (times?: number) => void;
  shiftEnter: () => void;
  ctrlEnter: () => KeyboardEvent;
  tab: () => KeyboardEvent;
  shiftTab: () => KeyboardEvent;
  click: (name: string) => Promise<void>;
  block: (value: string) => Promise<void>;
  undo: () => Promise<void>;
  redo: () => Promise<void>;
  backspace: () => void;
  /** Synchronous toolbar click (keeps a collapsed selection's pending format). */
  fmt: (name: string) => void;
  link: (url: string, text?: string) => Promise<void>;
  upload: (file?: File) => Promise<void>;
  tabChar: () => void;
  /** Where the selection is, e.g. `root:1` or `text(3):2`. */
  sel: () => string;
  /** The Lexical tree as `type(text)` for debugging. */
  tree: () => string;
  /** Run a callback in an editor update. */
  update: (fn: () => void) => void;
  uploader: ReturnType<typeof vi.fn>;
  onSubmit: ReturnType<typeof vi.fn>;
  onChange: ReturnType<typeof vi.fn>;
}

import { vi } from "vitest";

function findText(root: LexicalNode, text: string): TextNode | null {
  if ($isTextNode(root) && root.getTextContent().includes(text)) return root;
  if ($isElementNode(root)) {
    for (const child of root.getChildren()) {
      const hit = findText(child, text);
      if (hit) return hit;
    }
  }
  return null;
}

function dump(node: LexicalNode): string {
  if ($isTextNode(node)) return JSON.stringify(node.getTextContent()) + (node.getFormat() ? `[f${node.getFormat()}]` : "");
  if ($isElementNode(node)) {
    return `${node.getType()}${node.getIndent() ? `{i${node.getIndent()}}` : ""}(${node.getChildren().map(dump).join(" ")})`;
  }
  return node.getType();
}

const flush = () => act(async () => { await new Promise((r) => setTimeout(r, 0)); });

export function mount(config: Config, value = "", options: { enterToSubmit?: boolean } = {}): Ctx {
  const onChange = vi.fn();
  const uploader = vi.fn(() => "blob:https://csm.example.com/1");
  const onSubmit = vi.fn();
  const reply = config === "reply";
  render(
    <Editor
      autoFocus
      value={value}
      onChange={onChange}
      toolbarVariant="full"
      enterToSubmit={options.enterToSubmit ?? false}
      onSubmitKeyDown={onSubmit}
      onImageUpload={uploader}
      {...(reply ? { enterInsertsLineBreak: true, markdownPaste: true } : {})}
    />,
  );
  const editable = () => screen.getByTestId("case-description-editor");
  const lexical = () => (editable() as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;
  const update = (fn: () => void) => {
    act(() => {
      lexical().update(fn, { discrete: true });
    });
  };
  const key = (init: KeyboardEventInit & { key: string; keyCode: number }) => {
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, code: init.key, ...init } as KeyboardEventInit);
    act(() => {
      editable().dispatchEvent(event);
    });
    return event;
  };
  const place = (text: string, where: "after" | "before") =>
    update(() => {
      const node = findText($getRoot(), text);
      if (!node) throw new Error(`no text ${text}`);
      const at = node.getTextContent().indexOf(text) + (where === "after" ? text.length : 0);
      node.select(at, at);
    });
  return {
    // The nested-list wrapper's `list-style-type: none` is covered by one explicit test
    // (fixes file); stripped here so a markup tweak does not break every frozen export.
    html: () => ((onChange.mock.calls.at(-1)?.[0] ?? "") as string).replace(/ style="list-style-type: none"/g, ""),
    rawHtml: () => (onChange.mock.calls.at(-1)?.[0] ?? "") as string,
    editable,
    lexical,
    root: () => update(() => { $getRoot().select(0, 0); }),
    start: () => update(() => { $getRoot().getFirstDescendant()?.getParentOrThrow().selectStart(); }),
    end: () => update(() => {
      const last = $getRoot().getLastDescendant();
      if (last && $isTextNode(last)) last.select();
      else if (last && $isElementNode(last)) last.selectEnd();
      else last?.selectNext();
    }),
    after: (t) => place(t, "after"),
    before: (t) => place(t, "before"),
    select: (text) =>
      update(() => {
        const node = findText($getRoot(), text);
        if (!node) throw new Error(`no text ${text}`);
        const at = node.getTextContent().indexOf(text);
        node.select(at, at + text.length);
      }),
    selectAll: () =>
      update(() => {
        const sel = $createRangeSelection();
        const first = $getRoot().getFirstDescendant();
        const last = $getRoot().getLastDescendant();
        if (!first || !last) return;
        if ($isTextNode(first)) sel.anchor.set(first.getKey(), 0, "text");
        else sel.anchor.set(first.getParentOrThrow().getKey(), first.getIndexWithinParent(), "element");
        if ($isTextNode(last)) sel.focus.set(last.getKey(), last.getTextContentSize(), "text");
        else sel.focus.set(last.getParentOrThrow().getKey(), last.getIndexWithinParent() + 1, "element");
        $setSelection(sel);
      }),
    selectBetween: (from, to) =>
      update(() => {
        const a = findText($getRoot(), from);
        const b = findText($getRoot(), to);
        if (!a || !b) throw new Error("no text");
        const sel = $createRangeSelection();
        sel.anchor.set(a.getKey(), a.getTextContent().indexOf(from), "text");
        sel.focus.set(b.getKey(), b.getTextContent().indexOf(to) + to.length, "text");
        $setSelection(sel);
      }),
    type: (text) =>
      update(() => {
        const selection = $getSelection();
        if ($isRangeSelection(selection)) selection.insertText(text);
        else throw new Error("no range selection to type into");
      }),
    key,
    enter: (times = 1) => {
      for (let i = 0; i < times; i += 1) key({ key: "Enter", keyCode: 13 });
    },
    shiftEnter: () => { key({ key: "Enter", keyCode: 13, shiftKey: true }); },
    ctrlEnter: () => key({ key: "Enter", keyCode: 13, ctrlKey: true }),
    tab: () => key({ key: "Tab", keyCode: 9 }),
    shiftTab: () => key({ key: "Tab", keyCode: 9, shiftKey: true }),
    click: async (name) => {
      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name }));
      });
      await flush();
    },
    block: async (value) => {
      await act(async () => {
        fireEvent.change(document.querySelector("select[aria-label='Font variant']") as HTMLSelectElement, { target: { value } });
      });
      await flush();
    },
    undo: async () => {
      await act(async () => {
        fireEvent.click(document.querySelector("button[value='undo']") as HTMLElement);
      });
      await flush();
    },
    redo: async () => {
      await act(async () => {
        fireEvent.click(document.querySelector("button[value='redo']") as HTMLElement);
      });
      await flush();
    },
    backspace: () => {
      update(() => {
        const selection = $getSelection();
        if ($isRangeSelection(selection)) selection.deleteCharacter(true);
      });
    },
    fmt: (name) => {
      fireEvent.click(screen.getByRole("button", { name }));
    },
    link: async (url, text) => {
      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Link" }));
      });
      await flush();
      await act(async () => {
        fireEvent.change(screen.getByLabelText("URL"), { target: { value: url } });
        if (text !== undefined) fireEvent.change(screen.getByLabelText("Text (Optional)"), { target: { value: text } });
      });
      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Add Link" }));
      });
      await flush();
    },
    upload: async (file = png()) => {
      const input = document.querySelector('input[type="file"][accept*="image"]') as HTMLInputElement;
      if (!input) throw new Error("no image input");
      await act(async () => {
        fireEvent.change(input, { target: { files: [file] } });
      });
      await flush();
      await waitFor(() => expect(editable().querySelector("img")).not.toBeNull());
    },
    tabChar: () => update(() => {
      const selection = $getSelection();
      if ($isRangeSelection(selection)) selection.insertNodes([$createTabNode()]);
    }),
    sel: () => {
      let s = "none";
      lexical().getEditorState().read(() => {
        const selection = $getSelection();
        if ($isRangeSelection(selection)) {
          const a = selection.anchor;
          s = `${a.type}:${a.getNode().getType()}@${a.offset}${selection.isCollapsed() ? "" : "..range"}`;
        }
      });
      return s;
    },
    tree: () => {
      let out = "";
      lexical().getEditorState().read(() => { out = $getRoot().getChildren().map(dump).join(" "); });
      return out;
    },
    update,
    uploader,
    onSubmit,
    onChange,
  };
}

import { expect } from "vitest";
export { cleanup, act, screen, fireEvent, waitFor };
