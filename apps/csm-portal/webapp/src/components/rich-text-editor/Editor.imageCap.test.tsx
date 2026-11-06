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


import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeAll } from "vitest";
import "@testing-library/jest-dom/vitest";

import Editor from "./Editor";
import { MAX_COMMENT_IMAGES } from "./richTextConstants";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

// jsdom has no ClipboardEvent; the stand-in only has to carry clipboardData.
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
  if (!Range.prototype.getBoundingClientRect) {
    Range.prototype.getBoundingClientRect = () =>
      ({ x: 0, y: 0, width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0 }) as DOMRect;
  }
});

const png = (name = "shot.png") => new File([new Uint8Array(4)], name, { type: "image/png" });
const MB = 1024 * 1024;
/** An image file that reports `mb` megabytes without allocating them. */
const bigPng = (mb: number, name = "big.png") => {
  const file = png(name);
  Object.defineProperty(file, "size", { value: mb * MB });
  return file;
};

/** Pastes `files` (all as clipboard image items) into the editor; returns the event. */
async function pasteImages(files: File[]): Promise<ClipboardEvent> {
  const clipboardData = {
    getData: () => "",
    items: files.map((f) => ({ type: f.type, kind: "file", getAsFile: () => f })),
  } as unknown as DataTransfer;
  const event = new ClipboardEvent("paste", {
    bubbles: true,
    cancelable: true,
    clipboardData,
  } as ClipboardEventInit & { clipboardData: DataTransfer });
  await act(async () => {
    screen.getByTestId("case-description-editor").dispatchEvent(event);
  });
  return event;
}

const editorImages = () =>
  screen.getByTestId("case-description-editor").querySelectorAll("img");

function setup(withUploader = true) {
  let n = 0;
  const onImageUpload = vi.fn(() => `blob:https://csm.example.com/${(n += 1)}`);
  const onPasteError = vi.fn();
  render(
    <Editor
      autoFocus
      showToolbar={false}
      onChange={() => {}}
      onPasteError={onPasteError}
      onImageUpload={withUploader ? onImageUpload : undefined}
    />,
  );
  return { onImageUpload, onPasteError };
}

describe("Editor image cap (upload on paste)", () => {
  it("accepts images up to the cap, then refuses the next without uploading it", async () => {
    const { onImageUpload, onPasteError } = setup();
    for (let i = 0; i < MAX_COMMENT_IMAGES; i += 1) await pasteImages([png()]);
    await waitFor(() => expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES));
    expect(onImageUpload).toHaveBeenCalledTimes(MAX_COMMENT_IMAGES);
    expect(onPasteError).not.toHaveBeenCalled();

    const event = await pasteImages([png("sixth.png")]);

    expect(event.defaultPrevented).toBe(true);
    expect(onPasteError).toHaveBeenCalledWith("count");
    expect(onImageUpload).toHaveBeenCalledTimes(MAX_COMMENT_IMAGES);
    expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES);
  });

  it("refuses an image that would take the images past 25 MB in total", async () => {
    const { onImageUpload, onPasteError } = setup();
    await pasteImages([bigPng(10, "a.png")]);
    await pasteImages([bigPng(10, "b.png")]);
    await waitFor(() => expect(editorImages()).toHaveLength(2));

    const event = await pasteImages([bigPng(6, "c.png")]);

    expect(event.defaultPrevented).toBe(true);
    expect(onPasteError).toHaveBeenCalledWith("total");
    expect(onImageUpload).toHaveBeenCalledTimes(2);
    expect(editorImages()).toHaveLength(2);

    await pasteImages([bigPng(5, "d.png")]);
    await waitFor(() => expect(editorImages()).toHaveLength(3));
  });

  it("refuses a single image over the limit and counts a multi-image paste cumulatively", async () => {
    const { onImageUpload, onPasteError } = setup();
    await pasteImages([bigPng(26)]);
    expect(onPasteError).toHaveBeenCalledWith("size");
    expect(onImageUpload).not.toHaveBeenCalled();

    onPasteError.mockClear();
    await pasteImages([bigPng(20, "a.png"), bigPng(6, "b.png")]);
    await waitFor(() => expect(editorImages()).toHaveLength(1));
    expect(onPasteError).toHaveBeenCalledWith("total");
    expect(onImageUpload).toHaveBeenCalledTimes(1);
  });

  it("frees a slot when an image is removed from the draft", async () => {
    const { onImageUpload, onPasteError } = setup();
    for (let i = 0; i < MAX_COMMENT_IMAGES; i += 1) await pasteImages([png()]);
    await waitFor(() => expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES));

    fireEvent.click(screen.getAllByRole("button", { name: "Remove image" })[0]);
    await waitFor(() => expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES - 1));

    await pasteImages([png()]);
    await waitFor(() => expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES));
    expect(onImageUpload).toHaveBeenCalledTimes(MAX_COMMENT_IMAGES + 1);
    expect(onPasteError).not.toHaveBeenCalled();
  });

  it("takes only as many images of a multi-image paste as still fit, and reports the rest", async () => {
    const { onImageUpload, onPasteError } = setup();
    for (let i = 0; i < MAX_COMMENT_IMAGES - 2; i += 1) await pasteImages([png()]);
    await waitFor(() => expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES - 2));

    await pasteImages([png("a.png"), png("b.png"), png("c.png"), png("d.png")]);

    await waitFor(() => expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES));
    expect(onImageUpload).toHaveBeenCalledTimes(MAX_COMMENT_IMAGES);
    expect(onPasteError).toHaveBeenCalledTimes(1);
    expect(onPasteError).toHaveBeenCalledWith("count");
  });

  it("does not cap an editor without onImageUpload (it embeds base64; the server caps those)", async () => {
    const { onPasteError } = setup(false);
    const readAsDataURL = vi
      .spyOn(FileReader.prototype, "readAsDataURL")
      .mockImplementation(function (this: FileReader) {
        Object.defineProperty(this, "result", { value: "data:image/png;base64,AAAA" });
        this.onload?.({ target: this } as unknown as ProgressEvent<FileReader>);
      });
    try {
      for (let i = 0; i < MAX_COMMENT_IMAGES + 2; i += 1) await pasteImages([png()]);
      await waitFor(() => expect(editorImages()).toHaveLength(MAX_COMMENT_IMAGES + 2));
      expect(onPasteError).not.toHaveBeenCalled();
    } finally {
      readAsDataURL.mockRestore();
    }
  });
});
