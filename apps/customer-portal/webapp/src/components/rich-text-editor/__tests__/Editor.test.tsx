// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.  See the License for the specific language governing
// permissions and limitations under the License.

import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import Editor from "@components/rich-text-editor/Editor";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";

function renderEditor(props: Parameters<typeof Editor>[0] = {}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <ErrorBannerProvider>
        <Editor {...props} />
      </ErrorBannerProvider>
    </ThemeProvider>,
  );
}

describe("Editor", () => {
  it("renders without throwing", () => {
    renderEditor();
    expect(screen.getByTestId("case-description-editor")).toBeInTheDocument();
  });

  it("renders placeholder text", () => {
    renderEditor();
    expect(screen.getByText("Enter description...")).toBeInTheDocument();
  });

  it("renders when disabled", () => {
    renderEditor({ disabled: true });
    expect(screen.getByTestId("case-description-editor")).toBeInTheDocument();
  });

  it("renders attachments section when attachments provided", () => {
    const file = new File(["content"], "test.txt", { type: "text/plain" });
    renderEditor({ attachments: [file] });
    expect(screen.getByText(/Attachments \(1\)/)).toBeInTheDocument();
  });

  it("updates content when value prop changes from empty", async () => {
    const { rerender } = render(
      <ThemeProvider theme={createTheme()}>
        <ErrorBannerProvider>
          <Editor value="" onChange={vi.fn()} />
        </ErrorBannerProvider>
      </ThemeProvider>,
    );

    expect(screen.getByTestId("case-description-editor")).toBeInTheDocument();

    rerender(
      <ThemeProvider theme={createTheme()}>
        <ErrorBannerProvider>
          <Editor value="<p>AI Generated Content</p>" onChange={vi.fn()} />
        </ErrorBannerProvider>
      </ThemeProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("case-description-editor")).toHaveTextContent(
        "AI Generated Content",
      );
    });
  });

  describe("pasting an image with onImageUpload", () => {
    const file = new File([new Uint8Array([1, 2, 3])], "image.png", {
      type: "image/png",
    });
    const PREVIEW = "blob:http://localhost/preview-1";

    function paste(target: HTMLElement, f: File = file, type = "image/png") {
      const event = new Event("paste", { bubbles: true, cancelable: true });
      Object.defineProperty(event, "clipboardData", {
        value: { items: [{ type, getAsFile: () => f }] },
      });
      target.dispatchEvent(event);
      return event;
    }

    beforeEach(() => {
      Object.assign(URL, {
        createObjectURL: vi.fn(() => PREVIEW),
        revokeObjectURL: vi.fn(),
      });
    });
    afterEach(() => {
      vi.restoreAllMocks();
    });

    it("shows the image from a blob: URL and hands the file to the handler instead of reading base64", async () => {
      const onImageUpload = vi.fn(() => new Promise<void>(() => {}));
      const readAsDataURL = vi.spyOn(FileReader.prototype, "readAsDataURL");
      renderEditor({ onImageUpload });
      const input = screen.getByTestId("case-description-editor");

      const event = paste(input);

      expect(event.defaultPrevented).toBe(true);
      expect(onImageUpload).toHaveBeenCalledWith(file, PREVIEW);
      expect(readAsDataURL).not.toHaveBeenCalled();
      await waitFor(() => {
        expect(input.querySelector("img")?.getAttribute("src")).toBe(PREVIEW);
      });
    });

    it("takes the image out again when the upload fails", async () => {
      let reject: (e: Error) => void = () => {};
      const onImageUpload = vi.fn(
        () => new Promise<void>((_, r) => (reject = r)),
      );
      renderEditor({ onImageUpload });
      const input = screen.getByTestId("case-description-editor");
      paste(input);
      await waitFor(() => expect(input.querySelector("img")).not.toBeNull());

      reject(new Error("boom"));

      await waitFor(() => expect(input.querySelector("img")).toBeNull());
      expect(URL.revokeObjectURL).toHaveBeenCalledWith(PREVIEW);
    });

    it("keeps an image that uploaded", async () => {
      const onImageUpload = vi.fn(() => Promise.resolve());
      renderEditor({ onImageUpload });
      const input = screen.getByTestId("case-description-editor");
      paste(input);
      await waitFor(() => expect(input.querySelector("img")).not.toBeNull());
      await Promise.resolve();
      expect(input.querySelector("img")?.getAttribute("src")).toBe(PREVIEW);
    });

    it("does not upload an unsupported type and reports it", () => {
      const onImageUpload = vi.fn();
      const onPasteError = vi.fn();
      renderEditor({ onImageUpload, onPasteError });
      paste(
        screen.getByTestId("case-description-editor"),
        new File(["x"], "a.gif", { type: "image/gif" }),
        "image/gif",
      );
      expect(onImageUpload).not.toHaveBeenCalled();
      expect(onPasteError).toHaveBeenCalledWith("type");
    });

    it("does not upload an image over 10 MB and reports it", () => {
      const onImageUpload = vi.fn();
      const onPasteError = vi.fn();
      renderEditor({ onImageUpload, onPasteError });
      const big = new File(["x"], "big.png", { type: "image/png" });
      Object.defineProperty(big, "size", { value: 10 * 1024 * 1024 + 1 });
      paste(screen.getByTestId("case-description-editor"), big);
      expect(onImageUpload).not.toHaveBeenCalled();
      expect(onPasteError).toHaveBeenCalledWith("size");
    });
  });
});
