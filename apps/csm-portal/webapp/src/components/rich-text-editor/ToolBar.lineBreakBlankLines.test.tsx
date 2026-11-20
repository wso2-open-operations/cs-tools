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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { useEffect } from "react";
import { LexicalComposer } from "@lexical/react/LexicalComposer";
import { RichTextPlugin } from "@lexical/react/LexicalRichTextPlugin";
import { ContentEditable } from "@lexical/react/LexicalContentEditable";
import { LexicalErrorBoundary } from "@lexical/react/LexicalErrorBoundary";
import { ListPlugin } from "@lexical/react/LexicalListPlugin";
import { HistoryPlugin } from "@lexical/react/LexicalHistoryPlugin";
import { OnChangePlugin } from "@lexical/react/LexicalOnChangePlugin";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { $generateHtmlFromNodes } from "@lexical/html";
import {
  $createLineBreakNode,
  $createParagraphNode,
  $createRangeSelection,
  $createTextNode,
  $getRoot,
  $setSelection,
  UNDO_COMMAND,
  type LexicalNode,
} from "lexical";
import { ListItemNode, ListNode } from "@lexical/list";
import { HeadingNode, QuoteNode } from "@lexical/rich-text";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { $createImageNode, ImageNode } from "./ImageNode";
import Toolbar from "./ToolBar";

const initialConfig = {
  namespace: "ToolBarBlankLinesTest",
  nodes: [ListNode, ListItemNode, HeadingNode, QuoteNode, ImageNode],
  onError: (error: Error) => {
    throw error;
  },
  editable: true,
};

/** One entry per line; "" is a blank line, "IMG" a line holding only an image. */
type Seed = { lines: string[]; from?: number; to?: number };

const SeedPlugin = ({ lines, from = 0, to = lines.length - 1 }: Seed) => {
  const [editor] = useLexicalComposerContext();
  useEffect(() => {
    editor.update(() => {
      const root = $getRoot();
      root.clear();
      const paragraph = $createParagraphNode();
      const lineNodes: LexicalNode[][] = lines.map((line) =>
        line === "IMG"
          ? [$createImageNode("https://example.com/a.png", "a")]
          : line === ""
            ? []
            : [$createTextNode(line)],
      );
      lineNodes.forEach((nodes, i) => {
        if (i > 0) paragraph.append($createLineBreakNode());
        paragraph.append(...nodes);
      });
      root.append(paragraph);
      const selection = $createRangeSelection();
      const point = (lineIndex: number, atEnd: boolean) => {
        const node = lineNodes[lineIndex][0];
        if (node && node.getType() === "text") {
          const text = node as ReturnType<typeof $createTextNode>;
          return [text.getKey(), atEnd ? text.getTextContentSize() : 0, "text"] as const;
        }
        // A blank (or image) line: the point sits on the paragraph, at that line.
        let offset = 0;
        for (let i = 0; i < lineIndex; i++) offset += lineNodes[i].length + 1;
        return [paragraph.getKey(), offset + (atEnd ? lineNodes[lineIndex].length : 0), "element"] as const;
      };
      const [ak, ao, at] = point(from, false);
      const [fk, fo, ft] = point(to, true);
      selection.anchor.set(ak, ao, at);
      selection.focus.set(fk, fo, ft);
      $setSelection(selection);
    });
  }, [editor, lines, from, to]);
  return null;
};

function renderHarness(seed: Seed, enterInsertsLineBreak = true) {
  const onChange = vi.fn();
  render(
    <ThemeProvider theme={createTheme()}>
      <ErrorBannerProvider>
        <LexicalComposer initialConfig={initialConfig}>
          <RichTextPlugin
            contentEditable={<ContentEditable data-testid="editable" />}
            placeholder={null}
            ErrorBoundary={LexicalErrorBoundary}
          />
          <Toolbar enterInsertsLineBreak={enterInsertsLineBreak} />
          <ListPlugin />
          <HistoryPlugin />
          <SeedPlugin {...seed} />
          <OnChangePlugin
            onChange={(_state, editor) => {
              editor.getEditorState().read(() => {
                onChange($generateHtmlFromNodes(editor));
              });
            }}
          />
        </LexicalComposer>
      </ErrorBannerProvider>
    </ThemeProvider>,
  );
  return () => onChange.mock.calls.at(-1)?.[0] as string;
}

const plain = (html: string) =>
  html
    .replace(/ (class|dir|style|value|src|alt|data-[a-z-]+)="[^"]*"/g, "")
    .replace(/<\/?span>/g, "");
const itemTexts = (html: string) =>
  Array.from(html.matchAll(/<li[^>]*>(.*?)<\/li>/g)).map((m) => plain(m[1]));

const EXAMPLE = ["Hi team,", "Line two", "", "Para two", "soft"];

describe("list conversion drops the blank lines it splits out (enterInsertsLineBreak)", () => {
  it.each([
    ["Bullet List", "<ul"],
    ["Numbered List", "<ol"],
  ])("%s: the example gives four items, no empty one", async (name, tag) => {
    const html = renderHarness({ lines: EXAMPLE });
    fireEvent.click(screen.getByRole("button", { name }));
    await waitFor(() => expect(html()).toContain(tag));
    expect(itemTexts(html())).toEqual(["Hi team,", "Line two", "Para two", "soft"]);
  });

  it("a blank line at the start of the selection is dropped", async () => {
    const html = renderHarness({ lines: ["", "a", "b"] });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    expect(itemTexts(html())).toEqual(["a", "b"]);
  });

  it("a blank line at the end of the selection is dropped", async () => {
    const html = renderHarness({ lines: ["a", "b", ""] });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    expect(itemTexts(html())).toEqual(["a", "b"]);
  });

  it("two consecutive blank lines are both dropped", async () => {
    const html = renderHarness({ lines: ["a", "", "", "b"] });
    fireEvent.click(screen.getByRole("button", { name: "Numbered List" }));
    await waitFor(() => expect(html()).toContain("<ol"));
    expect(itemTexts(html())).toEqual(["a", "b"]);
  });

  it("a line holding only an image is kept", async () => {
    const html = renderHarness({ lines: ["a", "IMG", "", "b"] });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    expect(Array.from(html().matchAll(/<li/g)).length).toBe(3);
    expect(html()).toContain("<img");
  });

  it("blank lines outside the selection are untouched", async () => {
    const html = renderHarness({ lines: ["a", "", "b", "", "c"], from: 2, to: 2 });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    expect(itemTexts(html())).toEqual(["b"]);
    // Before: "a<br>" kept together; after: "<br>c".
    expect((html().match(/<br>/g) ?? []).length).toBe(2);
  });

  it("when every selected line is blank, one empty item is made", async () => {
    const html = renderHarness({ lines: ["a", "", "", "b"], from: 1, to: 2 });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    expect(itemTexts(html())).toEqual([""]);
  });

  it("is one undo step", async () => {
    const html = renderHarness({ lines: EXAMPLE });
    const before = await waitFor(() => {
      const h = html();
      expect(h).toBeTruthy();
      return h;
    });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    const editable = screen.getByTestId("editable") as HTMLElement & {
      __lexicalEditor: { dispatchCommand: (c: unknown, p: unknown) => void };
    };
    act(() => {
      editable.__lexicalEditor.dispatchCommand(UNDO_COMMAND, undefined);
    });
    await waitFor(() => expect(html()).not.toContain("<ul"));
    expect(html()).toBe(before);
  });

  it("flag off: the blank line becomes an empty item as before", async () => {
    const html = renderHarness({ lines: EXAMPLE }, false);
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    // Lexical's own conversion: one item holding the <br>s.
    expect(Array.from(html().matchAll(/<li/g)).length).toBe(1);
  });
});
