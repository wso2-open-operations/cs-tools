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

import { render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import Editor from "./Editor";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn(), clearError: vi.fn(), showErrorBanner: vi.fn() }),
}));

function lastHtml(onChange: ReturnType<typeof vi.fn>): string {
  const calls = onChange.mock.calls;
  return calls.length ? String(calls[calls.length - 1][0]) : "";
}

describe("Editor insertHtml", () => {
  it("adds the requested HTML after what is already there and reports it once", async () => {
    const onChange = vi.fn();
    const onInserted = vi.fn();
    const { rerender } = render(
      <Editor value="<p>First paragraph.</p>" onChange={onChange} onHtmlInserted={onInserted} />,
    );
    await waitFor(() => expect(lastHtml(onChange)).toContain("First paragraph."));

    const request = { html: "<p>Template text.</p>", nonce: 1 };
    rerender(
      <Editor value="<p>First paragraph.</p>" onChange={onChange} onHtmlInserted={onInserted} insertHtml={request} />,
    );
    await waitFor(() => expect(lastHtml(onChange)).toContain("Template text."));
    expect(lastHtml(onChange)).toContain("First paragraph.");
    // Its own paragraph, never glued onto the existing sentence.
    expect(lastHtml(onChange)).not.toMatch(/First paragraph\.\s*Template text\./);
    expect(lastHtml(onChange).match(/<p[\s>]/g)?.length ?? 0).toBeGreaterThanOrEqual(2);
    expect(onInserted).toHaveBeenCalledTimes(1);

    // The same request again (a re-render) is not inserted a second time.
    rerender(
      <Editor value="<p>First paragraph.</p>" onChange={onChange} onHtmlInserted={onInserted} insertHtml={{ ...request }} />,
    );
    await waitFor(() => expect(onInserted).toHaveBeenCalledTimes(1));
    expect(lastHtml(onChange).match(/Template text\./g)).toHaveLength(1);
  });

  it("never runs a script carried in the template", async () => {
    const onChange = vi.fn();
    const hit = vi.fn();
    (window as unknown as { __templateHit: () => void }).__templateHit = hit;
    render(
      <Editor
        onChange={onChange}
        insertHtml={{ html: `<p>Hello</p><img src="x" onerror="window.__templateHit()"><script>window.__templateHit()</script>`, nonce: 1 }}
      />,
    );
    await waitFor(() => expect(lastHtml(onChange)).toContain("Hello"));
    expect(lastHtml(onChange)).not.toContain("onerror");
    expect(lastHtml(onChange)).not.toContain("<script");
    expect(hit).not.toHaveBeenCalled();
  });
});
