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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import EditorWithSourceToggle from "@components/rich-text-editor/EditorWithSourceToggle";
import { BULLETIN_HTML } from "@components/rich-text-editor/bulletinFixture";

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn(), clearError: vi.fn(), showErrorBanner: vi.fn() }),
}));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

const SIMPLE_HTML = "<p>Hello <strong>world</strong></p>";

/** The textarea shown in HTML-source mode, or null when the rich editor is showing. */
const sourceBox = (): HTMLTextAreaElement | null =>
  screen.queryByPlaceholderText("<p>Type HTML here…</p>") as HTMLTextAreaElement | null;

const richEditor = (): HTMLElement | null => document.querySelector('[contenteditable="true"]');

describe("EditorWithSourceToggle (keepStyledHtmlAsSource) — content the rich editor cannot represent", () => {
  it("opens in HTML source, with the value untouched, and never reports a rewritten value", async () => {
    const onChange = vi.fn();
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={onChange} />);

    expect(sourceBox()).not.toBeNull();
    expect(sourceBox()?.value).toBe(BULLETIN_HTML);
    expect(richEditor()).toBeNull();
    // Give any stray editor update time to fire: none may.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("explains why it is in source mode", () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={vi.fn()} />);
    expect(screen.getByText(/shown as HTML source to preserve it exactly/i)).toBeInTheDocument();
  });

  it("switches to source when the styled value arrives after the first render, as the request dialog supplies it", async () => {
    const onChange = vi.fn();
    const { rerender } = render(<EditorWithSourceToggle keepStyledHtmlAsSource value="" onChange={onChange} />);
    expect(sourceBox()).toBeNull();

    rerender(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={onChange} />);

    expect(sourceBox()?.value).toBe(BULLETIN_HTML);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("stays in source while the user edits, even after the last styled element is deleted", () => {
    const onChange = vi.fn();
    const { rerender } = render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={onChange} />);

    fireEvent.change(sourceBox() as HTMLTextAreaElement, { target: { value: "<p>now plain</p>" } });
    expect(onChange).toHaveBeenCalledWith("<p>now plain</p>");

    rerender(<EditorWithSourceToggle keepStyledHtmlAsSource value="<p>now plain</p>" onChange={onChange} />);
    expect(sourceBox()?.value).toBe("<p>now plain</p>");
    expect(richEditor()).toBeNull();
  });

  it("asks before switching to the rich editor, and keeps the source if the user declines", async () => {
    const onChange = vi.fn();
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={onChange} />);

    fireEvent.click(screen.getByRole("switch"));
    expect(await screen.findByText(/switch to the rich editor\?/i)).toBeInTheDocument();
    expect(sourceBox()).not.toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /keep html source/i }));
    await waitFor(() => expect(screen.queryByText(/switch to the rich editor\?/i)).not.toBeInTheDocument());
    expect(sourceBox()?.value).toBe(BULLETIN_HTML);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("switches to the rich editor, simplifying the content, only when the user confirms", async () => {
    const onChange = vi.fn();
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={onChange} />);

    fireEvent.click(screen.getByRole("switch"));
    fireEvent.click(await screen.findByRole("button", { name: /switch and simplify/i }));

    await waitFor(() => expect(richEditor()).not.toBeNull());
    expect(sourceBox()).toBeNull();
    // This is the loss the warning is about: the rich editor reports the simplified markup.
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const reported = onChange.mock.calls[onChange.mock.calls.length - 1][0] as string;
    expect(reported).not.toContain("border-radius");
  });
});

describe("EditorWithSourceToggle (keepStyledHtmlAsSource) — ordinary content is unchanged", () => {
  it("opens in the rich editor for content it can represent", () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={SIMPLE_HTML} onChange={vi.fn()} />);
    expect(richEditor()).not.toBeNull();
    expect(sourceBox()).toBeNull();
  });

  it("opens in the rich editor when empty", () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value="" onChange={vi.fn()} />);
    expect(richEditor()).not.toBeNull();
  });

  it("toggles to source and back with no confirmation", async () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={SIMPLE_HTML} onChange={vi.fn()} />);

    fireEvent.click(screen.getByRole("switch"));
    expect(sourceBox()?.value).toBe(SIMPLE_HTML);

    fireEvent.click(screen.getByRole("switch"));
    expect(screen.queryByText(/switch to the rich editor\?/i)).not.toBeInTheDocument();
    await waitFor(() => expect(richEditor()).not.toBeNull());
  });

  it("does not show the styling explanation for ordinary content in source mode", () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={SIMPLE_HTML} onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("switch"));
    expect(screen.queryByText(/shown as HTML source to preserve it exactly/i)).not.toBeInTheDocument();
  });

  it("keeps the HTML source switch disabled when the field is disabled", () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={vi.fn()} disabled />);
    expect(screen.getByRole("switch")).toBeDisabled();
  });
});

describe("EditorWithSourceToggle — ariaRequired", () => {
  it("marks the rich editor and the source box aria-required when set", () => {
    render(<EditorWithSourceToggle value={SIMPLE_HTML} onChange={vi.fn()} ariaRequired />);
    expect(richEditor()).toHaveAttribute("aria-required", "true");

    fireEvent.click(screen.getByRole("switch"));
    expect(sourceBox()).toHaveAttribute("aria-required", "true");
  });

  it("marks neither when not set", () => {
    render(<EditorWithSourceToggle value={SIMPLE_HTML} onChange={vi.fn()} />);
    expect(richEditor()).not.toHaveAttribute("aria-required");

    fireEvent.click(screen.getByRole("switch"));
    expect(sourceBox()).not.toHaveAttribute("aria-required");
  });
});

describe("EditorWithSourceToggle (keepStyledHtmlAsSource) — preview", () => {
  it("shows a preview of the styled HTML under the source box, without changing it", async () => {
    const onChange = vi.fn();
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={onChange} />);

    const preview = screen.getByTestId("html-source-preview");
    expect([...preview.querySelectorAll("div")].some((d) => /border-radius/.test(d.getAttribute("style") ?? ""))).toBe(true);
    expect(sourceBox()?.value).toBe(BULLETIN_HTML);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("previews ordinary HTML typed in source mode too", () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={SIMPLE_HTML} onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("switch"));
    expect(screen.getByTestId("html-source-preview")).toHaveTextContent("Hello world");
  });

  it("has no preview while the rich editor is showing", () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={SIMPLE_HTML} onChange={vi.fn()} />);
    expect(screen.queryByTestId("html-source-preview")).not.toBeInTheDocument();
  });

  it("tells the user, in the switch-back warning, where to see how it will look", async () => {
    render(<EditorWithSourceToggle keepStyledHtmlAsSource value={BULLETIN_HTML} onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("switch"));
    expect(await screen.findByText(/preview below the source box shows how it will look/i)).toBeInTheDocument();
  });
});

// Every other caller (the case comment editor, for one) does not pass the prop and
// must behave exactly as it always did, whatever HTML it is given.
describe("EditorWithSourceToggle without keepStyledHtmlAsSource — behaviour is unchanged", () => {
  it("opens styled content in the rich editor, as before", () => {
    render(<EditorWithSourceToggle value={BULLETIN_HTML} onChange={vi.fn()} />);
    expect(richEditor()).not.toBeNull();
    expect(sourceBox()).toBeNull();
    expect(screen.queryByText(/shown as HTML source to preserve it exactly/i)).not.toBeInTheDocument();
  });

  it("toggles source and back with no confirmation, even for styled content", async () => {
    render(<EditorWithSourceToggle value={BULLETIN_HTML} onChange={vi.fn()} />);

    fireEvent.click(screen.getByRole("switch"));
    expect(sourceBox()).not.toBeNull();
    // Without the opt-in there is no preview either: nothing about this caller changes.
    expect(screen.queryByTestId("html-source-preview")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("switch"));
    expect(screen.queryByText(/switch to the rich editor\?/i)).not.toBeInTheDocument();
    await waitFor(() => expect(richEditor()).not.toBeNull());
  });

  it("does not switch to source when styled content arrives after the first render", () => {
    const { rerender } = render(<EditorWithSourceToggle value="" onChange={vi.fn()} />);
    rerender(<EditorWithSourceToggle value={BULLETIN_HTML} onChange={vi.fn()} />);
    expect(sourceBox()).toBeNull();
  });
});
