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

import { describe, expect, it, vi } from "vitest";
import { dropDataImagesMatchingFiles, uploadPastedDataImages } from "@components/rich-text-editor/pastedDataImages";

const PNG = "data:image/png;base64,iVBORw0KGgo=";
const room = { count: 0, bytes: 0 };
const dom = (html: string) => new DOMParser().parseFromString(html, "text/html");

describe("uploadPastedDataImages", () => {
  it("hands a base64 image to the uploader and keeps only the preview address", () => {
    const upload = vi.fn((file: File) => (file ? "blob:http://localhost/abc" : ""));
    const doc = dom(`<p>Hi</p><img src="${PNG}">`);
    const refused = vi.fn();
    const left = uploadPastedDataImages(doc, room, upload, refused);
    expect(upload).toHaveBeenCalledTimes(1);
    expect(upload.mock.calls[0][0].type).toBe("image/png");
    expect(doc.querySelector("img")?.getAttribute("src")).toBe("blob:http://localhost/abc");
    expect(doc.body.innerHTML).not.toContain("base64");
    expect(refused).not.toHaveBeenCalled();
    expect(left.count).toBe(1);
  });

  it("removes the image and says so when there is no uploader", () => {
    const doc = dom(`<img src="${PNG}">`);
    const refused = vi.fn();
    uploadPastedDataImages(doc, room, undefined, refused);
    expect(doc.querySelector("img")).toBeNull();
    expect(refused).toHaveBeenCalledWith("disabled");
  });

  it("refuses an unsupported type, and leaves other images alone", () => {
    const doc = dom(`<img src="data:image/gif;base64,R0lGODlh"><img src="https://x.test/a.png">`);
    const upload = vi.fn(() => "blob:x");
    const refused = vi.fn();
    uploadPastedDataImages(doc, room, upload, refused);
    expect(refused).toHaveBeenCalledWith("type");
    expect(upload).not.toHaveBeenCalled();
    expect(doc.querySelectorAll("img")).toHaveLength(1);
    expect(doc.querySelector("img")?.getAttribute("src")).toBe("https://x.test/a.png");
  });

  it("refuses an image once the draft is full", () => {
    const doc = dom(`<img src="${PNG}">`);
    const refused = vi.fn();
    uploadPastedDataImages(doc, { count: 5, bytes: 0 }, () => "blob:x", refused);
    expect(refused).toHaveBeenCalledWith("count");
    expect(doc.querySelector("img")).toBeNull();
  });

  it("sizes an image by its payload without the whitespace in it", () => {
    // PNG is 8 bytes here; line breaks in the base64 must not make it look bigger.
    const doc = dom(`<img src="data:image/png;base64,iVBORw0K\nGgo=">`);
    const upload = vi.fn((file: File) => (file ? "blob:x" : ""));
    uploadPastedDataImages(doc, room, upload, vi.fn());
    expect(upload).toHaveBeenCalledTimes(1);
  });
});

describe("dropDataImagesMatchingFiles", () => {
  it("drops only the image that is the HTML copy of a clipboard file", () => {
    const doc = dom(`<img src="${PNG}"><img src="data:image/png;base64,AAAAAAAAAAAAAAAA">`);
    dropDataImagesMatchingFiles(doc, [{ type: "image/png", size: 8 }]);
    const left = doc.querySelectorAll("img");
    expect(left).toHaveLength(1);
    expect(left[0].getAttribute("src")).toBe("data:image/png;base64,AAAAAAAAAAAAAAAA");
  });

  it("keeps every image when no clipboard file matches", () => {
    const doc = dom(`<img src="${PNG}">`);
    dropDataImagesMatchingFiles(doc, [{ type: "image/png", size: 999 }]);
    expect(doc.querySelectorAll("img")).toHaveLength(1);
  });
});
