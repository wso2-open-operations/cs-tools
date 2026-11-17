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

// Editor logs via useLogger, which requires a LoggerProvider this test
// doesn't otherwise set up -- same pattern used by other component tests
// that render production trees pulling in the logger transitively.
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    warn: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

// jsdom does not implement the ClipboardEvent constructor. The editor's own
// paste handler guards with `event instanceof ClipboardEvent` (exactly what a
// real browser paste dispatches), so tests need a minimal stand-in that
// satisfies that check and carries a fake `clipboardData.getData(type)`.
beforeAll(() => {
  if (typeof globalThis.ClipboardEvent === "undefined") {
    class ClipboardEventPolyfill extends Event {
      clipboardData: DataTransfer | null;
      constructor(
        type: string,
        eventInitDict: EventInit & { clipboardData?: DataTransfer } = {},
      ) {
        super(type, eventInitDict);
        this.clipboardData = eventInitDict.clipboardData ?? null;
      }
    }
    // @ts-expect-error -- test-only polyfill; jsdom has no native ClipboardEvent.
    globalThis.ClipboardEvent = ClipboardEventPolyfill;
  }

  // jsdom's Range does not implement `getBoundingClientRect` (a documented
  // jsdom gap, unrelated to this feature). Lexical's own post-commit
  // "scroll pasted text into view" step reads it for a collapsed text
  // selection, which a plain-text ("Remove Formatting") insertion produces --
  // without this stub that step throws and the commit's own update
  // listeners (this editor's `onChange`) never fire.
  if (!Range.prototype.getBoundingClientRect) {
    Range.prototype.getBoundingClientRect = () =>
      ({
        x: 0,
        y: 0,
        width: 0,
        height: 0,
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        toJSON() {
          return this;
        },
      }) as DOMRect;
  }
});

/** A fake clipboard: its text formats, and one item per type (an `image/*` item yields `file`). */
function fakeClipboardData(data: Record<string, string>, itemTypes: string[] = [], file: File | null = null): DataTransfer {
  return {
    getData: (type: string) => data[type] ?? "",
    items: itemTypes.map((type) => ({
      kind: type.startsWith("image/") ? "file" : "string",
      type,
      getAsFile: () => (type.startsWith("image/") ? file : null),
    })),
  } as unknown as DataTransfer;
}

/** Dispatches a paste of `data` (and `itemTypes`, with `file` as the image item) on `target`. */
function paste(target: Element, data: Record<string, string>, itemTypes: string[] = [], file: File | null = null): void {
  const event = new ClipboardEvent("paste", {
    bubbles: true,
    cancelable: true,
    clipboardData: fakeClipboardData(data, itemTypes, file),
  } as ClipboardEventInit & { clipboardData: DataTransfer });
  // The paste handler's `setPendingPaste` (React state, for the dialog) is
  // triggered synchronously from this dispatch but outside any React event
  // handler React itself is aware of -- wrap in `act` so that state update is
  // flushed the same way a real browser-dispatched paste event's would be.
  act(() => {
    target.dispatchEvent(event);
  });
}

// Representative Word clipboard fragment: mso-* inline style, one of many
// possible rich-HTML shapes -- the prompt no longer depends on this marker,
// any non-empty text/html triggers it.
const WORD_HTML =
  '<p class=MsoNormal style="mso-margin-top-alt:auto">Hello <b>Word</b></p>';

// Representative Gmail clipboard fragment: rich HTML with no mso-*/
// docs-internal-guid marker anywhere -- must still trigger the prompt, same
// as every other HTML paste source.
const GMAIL_HTML = '<div dir="ltr">Hello <b>Gmail</b></div>';

const getEditable = () => screen.getByTestId("case-description-editor");

describe("Editor paste-format prompt", () => {
  it("shows the Keep/Remove Formatting prompt for a Word-fingerprinted paste", async () => {
    render(<Editor autoFocus showToolbar={false} onChange={() => {}} />);
    paste(getEditable(), { "text/html": WORD_HTML, "text/plain": "Hello Word" });

    expect(
      await screen.findByText("Paste formatted content?"),
    ).toBeInTheDocument();
  });

  it("shows the Keep/Remove Formatting prompt for a Gmail-style paste too (no Word/GDocs fingerprint)", async () => {
    render(<Editor autoFocus showToolbar={false} onChange={() => {}} />);
    paste(getEditable(), {
      "text/html": GMAIL_HTML,
      "text/plain": "Hello Gmail",
    });

    expect(
      await screen.findByText("Paste formatted content?"),
    ).toBeInTheDocument();
  });

  it("does not show the prompt for a plain-text-only paste (no text/html on the clipboard)", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
    paste(getEditable(), { "text/plain": "Hello plain text" });

    // The plain-text path still normalizes and inserts silently.
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(
      screen.queryByText("Paste formatted content?"),
    ).not.toBeInTheDocument();
  });

  it("'Keep Formatting' inserts the pasted content with its structure intact", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
    paste(getEditable(), { "text/html": WORD_HTML, "text/plain": "Hello Word" });

    const keepButton = await screen.findByRole("button", {
      name: "Keep Formatting",
    });
    fireEvent.click(keepButton);

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const lastHtml = onChange.mock.calls.at(-1)?.[0] as string;
    expect(lastHtml).toContain("Word");
    expect(lastHtml.toLowerCase()).toContain("<b>");
    expect(
      screen.queryByText("Paste formatted content?"),
    ).not.toBeInTheDocument();
  });

  it("'Remove Formatting' inserts plain text only, discarding all structure", async () => {
    const onChange = vi.fn();
    render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
    paste(getEditable(), { "text/html": WORD_HTML, "text/plain": "Hello Word" });

    const removeButton = await screen.findByRole("button", {
      name: "Remove Formatting",
    });
    fireEvent.click(removeButton);

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const lastHtml = onChange.mock.calls.at(-1)?.[0] as string;
    expect(lastHtml).toContain("Hello Word");
    expect(lastHtml.toLowerCase()).not.toContain("<b>");
    expect(
      screen.queryByText("Paste formatted content?"),
    ).not.toBeInTheDocument();
  });

  describe("a picture copied from a web page", () => {
    const IMG_ONLY = `<meta charset='utf-8'><img src="https://example.com/a.png" alt="A | B" style="max-width: 100%;">`;

    it("pastes only the stored image, not a second hot-linked copy from its HTML", async () => {
      const onChange = vi.fn();
      const onImageUpload = vi.fn(() => "blob:http://localhost/stored-1");
      const file = new File([new Uint8Array(100)], "a.png", { type: "image/png" });
      render(<Editor autoFocus showToolbar={false} onChange={onChange} onImageUpload={onImageUpload} />);
      paste(getEditable(), { "text/html": IMG_ONLY }, ["text/html", "image/png"], file);

      await waitFor(() => expect(onImageUpload).toHaveBeenCalledTimes(1));
      expect(onImageUpload).toHaveBeenCalledWith(file);
      await waitFor(() => expect(onChange.mock.calls.some(([html]) => String(html).includes("stored-1"))).toBe(true));
      const html = String(onChange.mock.calls.at(-1)?.[0]);
      expect(html.match(/<img /g)).toHaveLength(1);
      expect(html).toContain("blob:http://localhost/stored-1");
      expect(html).not.toContain("example.com");
      expect(screen.queryByText("Paste formatted content?")).not.toBeInTheDocument();
    });

    it("still asks about HTML that has text beside the image", async () => {
      render(<Editor autoFocus showToolbar={false} onChange={() => {}} />);
      paste(
        getEditable(),
        { "text/html": `<p>Look</p>${IMG_ONLY}`, "text/plain": "Look" },
        ["text/html", "image/png"],
      );
      expect(await screen.findByText("Paste formatted content?")).toBeInTheDocument();
    });

    it("still asks about an image's HTML when the clipboard has no image file", async () => {
      render(<Editor autoFocus showToolbar={false} onChange={() => {}} />);
      paste(getEditable(), { "text/html": IMG_ONLY }, ["text/html"]);
      expect(await screen.findByText("Paste formatted content?")).toBeInTheDocument();
    });
  });

  describe("pasted HTML with nothing to keep", () => {
    it("a lone <meta> beside text/plain pastes the text with no prompt", async () => {
      const onChange = vi.fn();
      render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
      paste(getEditable(), { "text/html": '<meta charset="utf-8">', "text/plain": "hello world" });
      await waitFor(() => expect(String(onChange.mock.calls.at(-1)?.[0])).toContain("hello world"));
      expect(screen.queryByText("Paste formatted content?")).not.toBeInTheDocument();
    });

    it("a lone <meta> with Markdown text/plain still offers Paste as Markdown in the reply box", async () => {
      render(<Editor autoFocus showToolbar={false} onChange={() => {}} enterInsertsLineBreak markdownPaste />);
      paste(getEditable(), { "text/html": '<meta charset="utf-8">', "text/plain": "# Title\n\n- a\n- b" });
      expect(await screen.findByRole("button", { name: "Paste as Markdown" })).toBeInTheDocument();
    });

    it("whitespace-only blocks count as nothing too; with no text/plain either the paste is left alone", async () => {
      render(<Editor autoFocus showToolbar={false} onChange={() => {}} />);
      paste(getEditable(), { "text/html": "<!--StartFragment--><p><br></p><!--EndFragment-->" });
      expect(screen.queryByText("Paste formatted content?")).not.toBeInTheDocument();
    });
  });

  describe("Remove Formatting with no text/plain", () => {
    it("keeps blocks, <br>s and list items on their own lines", async () => {
      const onChange = vi.fn();
      render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
      paste(getEditable(), { "text/html": "<p>one</p><ul><li>two</li><li>three<br>four</li></ul><p>five</p>" });
      fireEvent.click(await screen.findByRole("button", { name: "Remove Formatting" }));
      await waitFor(() => expect(onChange).toHaveBeenCalled());
      expect(String(onChange.mock.calls.at(-1)?.[0])).toBe("<p>one</p><p>two</p><p>three</p><p>four</p><p>five</p>");
    });
  });

  describe("a paste into a code block", () => {
    async function inCode(props: Record<string, unknown> = {}) {
      const onChange = vi.fn();
      render(<Editor autoFocus showToolbar={false} onChange={onChange} value="<pre><code>c</code></pre>" {...props} />);
      await act(async () => {
        await new Promise((r) => setTimeout(r, 20));
      });
      const editable = getEditable() as HTMLElement & { __lexicalEditor: { update: (fn: () => void, o?: object) => void } };
      const { $getRoot, $isElementNode } = await import("lexical");
      act(() => {
        editable.__lexicalEditor.update(
          () => {
            const last = $getRoot().getLastDescendant();
            if (last && !$isElementNode(last)) (last as unknown as { select: () => void }).select();
          },
          { discrete: true },
        );
      });
      return { editable, onChange, last: () => String(onChange.mock.calls.at(-1)?.[0]) };
    }

    it("Keep Formatting HTML pastes the text as code lines with no prompt and no <p> inside the <pre>", async () => {
      const { editable, last } = await inCode();
      paste(editable, { "text/html": "<p>one</p><p>two <b>x</b></p>", "text/plain": "one\ntwo x" });
      await waitFor(() => expect(last()).toContain("two x"));
      expect(screen.queryByText("Paste formatted content?")).not.toBeInTheDocument();
      expect(last()).not.toMatch(/<pre[^>]*>[\s\S]*<p[ >]/);
      expect(last()).toContain("one<br>two x");
    });

    it("text/plain only keeps blank lines and indentation", async () => {
      const { editable, last } = await inCode();
      paste(editable, { "text/plain": "a\n\n  b\n\n\nc" });
      await waitFor(() => expect(last()).toContain("<br>c</pre>"));
      expect(last()).toContain("a<br><br>  b<br><br><br>c");
      expect(last()).not.toMatch(/<pre[^>]*>[\s\S]*<p[ >]/);
    });

    it("a meta-only clipboard pastes its text/plain the same way", async () => {
      const { editable, last } = await inCode();
      paste(editable, { "text/html": '<meta charset="utf-8">', "text/plain": "a\n\n  b" });
      await waitFor(() => expect(last()).toContain("a<br><br>  b"));
      expect(screen.queryByText("Paste formatted content?")).not.toBeInTheDocument();
    });

    it("Markdown-looking text is never prompted in the reply box", async () => {
      const { editable, last } = await inCode({ enterInsertsLineBreak: true, markdownPaste: true });
      paste(editable, { "text/plain": "# Title\n\n- a\n- b" });
      await waitFor(() => expect(last()).toContain("# Title"));
      expect(screen.queryByRole("button", { name: /Markdown|Formatting/ })).not.toBeInTheDocument();
      expect(last()).toContain("# Title<br><br>- a<br>- b");
    });
  });

  describe("Word lists", () => {
    const WORD_LISTS =
      "<p class=MsoNormal>Intro<o:p></o:p></p>" +
      '<p class=MsoListParagraphCxSpFirst style="text-indent:-.25in;mso-list:l0 level1 lfo1"><![if !supportLists]><span style="font-family:Symbol;mso-list:Ignore">&middot;<span style="font:7.0pt \'Times New Roman\'">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; </span></span><![endif]>First<o:p></o:p></p>' +
      '<p class=MsoListParagraphCxSpMiddle style="margin-left:1.0in;text-indent:-.25in;mso-list:l0 level2 lfo1"><![if !supportLists]><span style="font-family:\'Courier New\';mso-list:Ignore">o<span style="font:7.0pt \'Times New Roman\'">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; </span></span><![endif]>Nested<o:p></o:p></p>' +
      '<p class=MsoListParagraphCxSpLast style="text-indent:-.25in;mso-list:l0 level1 lfo1"><![if !supportLists]><span style="font-family:Symbol;mso-list:Ignore">&middot;<span style="font:7.0pt \'Times New Roman\'">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; </span></span><![endif]>Last<o:p></o:p></p>' +
      '<p class=MsoListParagraphCxSpFirst style="text-indent:-.25in;mso-list:l1 level1 lfo2"><![if !supportLists]><span style="mso-list:Ignore">1.<span style="font:7.0pt \'Times New Roman\'">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; </span></span><![endif]>Step one<o:p></o:p></p>' +
      '<p class=MsoListParagraphCxSpLast style="text-indent:-.25in;mso-list:l1 level1 lfo2"><![if !supportLists]><span style="mso-list:Ignore">2.<span style="font:7.0pt \'Times New Roman\'">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; </span></span><![endif]>Step two<o:p></o:p></p>' +
      "<p class=MsoNormal>Outro<o:p></o:p></p>";

    it("Keep Formatting makes real nested bulleted and numbered lists between the paragraphs, with no glyphs", async () => {
      const onChange = vi.fn();
      render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
      paste(getEditable(), { "text/html": WORD_LISTS, "text/plain": "Intro\n·  First\no  Nested" });
      fireEvent.click(await screen.findByRole("button", { name: "Keep Formatting" }));
      await waitFor(() => expect(onChange).toHaveBeenCalled());
      const html = String(onChange.mock.calls.at(-1)?.[0]);
      expect(html).not.toContain("·");
      expect(html).not.toContain("o\u00a0");
      expect(html.indexOf("Intro")).toBeLessThan(html.indexOf("<ul"));
      expect(html).toMatch(/<ul[^>]*><li[^>]*>First<\/li><li[^>]*class="editor-list-item-nested"[^>]*><ul[^>]*><li[^>]*>Nested<\/li><\/ul><\/li><li[^>]*>Last<\/li><\/ul>/);
      expect(html).toMatch(/<ol[^>]*><li[^>]*>Step one<\/li><li[^>]*>Step two<\/li><\/ol>/);
      expect(html.indexOf("</ol>")).toBeLessThan(html.indexOf("Outro"));
    });

    it("Remove Formatting keeps today's plain text", async () => {
      const onChange = vi.fn();
      render(<Editor autoFocus showToolbar={false} onChange={onChange} />);
      paste(getEditable(), { "text/html": WORD_LISTS, "text/plain": "Intro\n·  First" });
      fireEvent.click(await screen.findByRole("button", { name: "Remove Formatting" }));
      await waitFor(() => expect(onChange).toHaveBeenCalled());
      expect(String(onChange.mock.calls.at(-1)?.[0])).toBe("<p>Intro</p><p>·  First</p>");
    });
  });
});
