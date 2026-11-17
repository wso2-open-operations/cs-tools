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


import { render, screen, act, cleanup, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { LexicalEditor } from "lexical";
import Editor from "./Editor";
import { editorHasInlineImages } from "./ImageNode";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

const IMG = '<p>a <img src="/0b9f3c1e-aaaa-4bbb-8ccc-1234567890ab" alt="pic"> b</p>';
const getLexical = () =>
  (screen.getByTestId("case-description-editor") as HTMLElement & { __lexicalEditor: LexicalEditor }).__lexicalEditor;

async function mountWith(flags: { enterInsertsLineBreak?: boolean }): Promise<{ html: () => string }> {
  const onChange = vi.fn();
  render(<Editor showToolbar={false} value={IMG} onChange={onChange} {...flags} />);
  await waitFor(() => expect(onChange).toHaveBeenCalled());
  await act(() => new Promise<void>((r) => setTimeout(r, 20)));
  return { html: () => (onChange.mock.calls.at(-1)?.[0] as string | undefined) ?? "" };
}

describe("inline images: one source of truth (the editor, not a theme key)", () => {
  afterEach(cleanup);

  it("the reply box exports and renders inline-block images, with no theme key in the editor config", async () => {
    const m = await mountWith({ enterInsertsLineBreak: true });
    expect(m.html()).toContain("display: inline-block");
    expect(editorHasInlineImages(getLexical())).toBe(true);
    const span = screen.getByTestId("case-description-editor").querySelector("span[style*='inline-block']");
    expect(span).not.toBeNull();
    expect((getLexical() as unknown as { _config: { theme: Record<string, unknown> } })._config.theme).not.toHaveProperty("inlineImages");
  });

  it("the normal editor exports and renders block images, with no theme key in the editor config", async () => {
    const m = await mountWith({});
    expect(m.html()).toContain("display: block");
    expect(m.html()).not.toContain("inline-block");
    expect(editorHasInlineImages(getLexical())).toBe(false);
    expect((getLexical() as unknown as { _config: { theme: Record<string, unknown> } })._config.theme).not.toHaveProperty("inlineImages");
  });
});
