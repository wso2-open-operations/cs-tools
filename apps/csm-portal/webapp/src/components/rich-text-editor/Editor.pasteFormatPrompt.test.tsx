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

function fakeClipboardData(data: Record<string, string>): DataTransfer {
  return {
    getData: (type: string) => data[type] ?? "",
  } as unknown as DataTransfer;
}

function paste(target: Element, data: Record<string, string>): void {
  const event = new ClipboardEvent("paste", {
    bubbles: true,
    cancelable: true,
    clipboardData: fakeClipboardData(data),
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
});
