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
import { OnChangePlugin } from "@lexical/react/LexicalOnChangePlugin";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { $generateHtmlFromNodes } from "@lexical/html";
import {
  $createLineBreakNode,
  $createParagraphNode,
  $createTextNode,
  $getRoot,
  $setSelection,
  $createRangeSelection,
  type ParagraphNode,
  type TextNode,
} from "lexical";
import { ListItemNode, ListNode } from "@lexical/list";
import { HeadingNode, QuoteNode } from "@lexical/rich-text";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import Toolbar from "./ToolBar";

const initialConfig = {
  namespace: "ToolBarLineBreakEdgeTest",
  nodes: [ListNode, ListItemNode, HeadingNode, QuoteNode],
  onError: (error: Error) => {
    throw error;
  },
  editable: true,
};

type Selection = { from: number; to: number };

/**
 * Seeds `<p>one<br>two<br>three</p>` and selects from line `from` to line
 * `to` (0-based): the whole of each line in between.
 */
const SeedPlugin = ({ from, to, twoParas }: Selection & { twoParas?: boolean }) => {
  const [editor] = useLexicalComposerContext();
  useEffect(() => {
    editor.update(() => {
      const root = $getRoot();
      root.clear();
      const paragraph: ParagraphNode = $createParagraphNode();
      const texts: TextNode[] = ["one", "two", "three"].map((t) => $createTextNode(t));
      texts.forEach((text, i) => {
        if (i > 0) paragraph.append($createLineBreakNode());
        paragraph.append(text);
      });
      root.append(paragraph);
      if (twoParas) {
        // <p>one<br>two</p><p>three</p>
        const second = $createParagraphNode();
        second.append(texts[2]);
        root.append(second);
        paragraph.getLastChild()!.remove(); // drop the break before "three"
      }
      const selection = $createRangeSelection();
      selection.anchor.set(texts[from].getKey(), 0, "text");
      selection.focus.set(texts[to].getKey(), texts[to].getTextContentSize(), "text");
      $setSelection(selection);
    });
  }, [editor, from, to, twoParas]);
  return null;
};

function renderHarness(opts: { enterInsertsLineBreak: boolean; twoParas?: boolean } & Selection) {
  const onChange = vi.fn();
  render(
    <ThemeProvider theme={createTheme()}>
      <ErrorBannerProvider>
        <LexicalComposer initialConfig={initialConfig}>
          <RichTextPlugin
            contentEditable={<ContentEditable />}
            placeholder={null}
            ErrorBoundary={LexicalErrorBoundary}
          />
          <Toolbar enterInsertsLineBreak={opts.enterInsertsLineBreak} />
          <ListPlugin />
          <SeedPlugin from={opts.from} to={opts.to} twoParas={opts.twoParas} />
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
  html.replace(/ (class|dir|style|value)="[^"]*"/g, "").replace(/<\/?span>/g, "");

describe("ToolBar block conversions: edge cases", () => {
  it("bullet list on the middle line only", async () => {
    const html = renderHarness({ enterInsertsLineBreak: true, from: 1, to: 1 });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    expect(plain(html())).toBe("<p>one</p><ul><li>two</li></ul><p>three</p>");
  });

  it("toggling the list back off leaves sensible paragraphs (no data loss)", async () => {
    const html = renderHarness({ enterInsertsLineBreak: true, from: 1, to: 1 });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).not.toContain("<ul"));
    const text = plain(html()).replace(/<[^>]+>/g, "|");
    expect(text).toContain("one");
    expect(text).toContain("two");
    expect(text).toContain("three");
    expect(plain(html())).toBe("<p>one</p><p>two</p><p>three</p>");
  });

  it("numbered list on all lines", async () => {
    const html = renderHarness({ enterInsertsLineBreak: true, from: 0, to: 2 });
    fireEvent.click(screen.getByRole("button", { name: "Numbered List" }));
    await waitFor(() => expect(html()).toContain("<ol"));
    expect(plain(html())).toBe("<ol><li>one</li><li>two</li><li>three</li></ol>");
  });

  it("lines spanning two paragraphs: each line becomes an item", async () => {
    // <p>one<br>two</p><p>three</p>, selection from 'one' to 'three'
    const html = renderHarness({ enterInsertsLineBreak: true, from: 0, to: 2, twoParas: true });
    fireEvent.click(screen.getByRole("button", { name: "Bullet List" }));
    await waitFor(() => expect(html()).toContain("<ul"));
    expect(plain(html())).toBe("<ul><li>one</li><li>two</li><li>three</li></ul>");
  });

  it("heading on a single middle line leaves the other lines together", async () => {
    const html = renderHarness({ enterInsertsLineBreak: true, from: 1, to: 1 });
    act(() => { fireEvent.change(screen.getByRole("combobox"), { target: { value: "h2" } }); });
    await waitFor(() => expect(html()).toContain("<h2"));
    expect(plain(html())).toBe("<p>one</p><h2>two</h2><p>three</p>");
  });

  it("flag OFF: heading on a middle line converts the whole paragraph (unchanged behaviour)", async () => {
    const html = renderHarness({ enterInsertsLineBreak: false, from: 1, to: 1 });
    act(() => { fireEvent.change(screen.getByRole("combobox"), { target: { value: "h2" } }); });
    await waitFor(() => expect(html()).toContain("<h2"));
    expect((html().match(/<h2/g) ?? []).length).toBe(1);
    expect(html()).toContain("<br>");
  });
});
