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

import { describe, expect, it } from "vitest";
import {
  hasBlobImageSource,
  rewriteBlobImageSources,
} from "@features/support/utils/inlineCommentImages";

const ID = "0F15CBCC-C36B-4310-AF2F-404599013196";
const URL_A = "blob:http://localhost:5173/aaaa-1111";
const URL_B = "blob:http://localhost:5173/bbbb-2222";

describe("rewriteBlobImageSources", () => {
  it("replaces an uploaded preview with a lowercase /<id> reference", () => {
    const html = `<p>Look</p><img src="${URL_A}" alt="x" style="max-width:100%">`;
    expect(rewriteBlobImageSources(html, new Map([[URL_A, ID]]))).toBe(
      `<p>Look</p><img src="/${ID.toLowerCase()}" alt="x" style="max-width:100%">`,
    );
  });

  it("replaces every preview and keeps single quotes", () => {
    const html = `<img src='${URL_A}'><img src="${URL_B}">`;
    const out = rewriteBlobImageSources(
      html,
      new Map([
        [URL_A, "11111111-1111-4111-8111-111111111111"],
        [URL_B, "22222222-2222-4222-8222-222222222222"],
      ]),
    );
    expect(out).toBe(
      `<img src='/11111111-1111-4111-8111-111111111111'><img src="/22222222-2222-4222-8222-222222222222">`,
    );
  });

  it("leaves a preview with no uploaded id, and every other image, alone", () => {
    const html = `<img src="${URL_B}"><img src="https://example.com/a.png"><img src="/${ID.toLowerCase()}">`;
    expect(rewriteBlobImageSources(html, new Map([[URL_A, ID]]))).toBe(html);
  });
});

describe("hasBlobImageSource", () => {
  it("is true only while an image still has a blob: src", () => {
    const html = `<p>a</p><img src="${URL_A}">`;
    expect(hasBlobImageSource(html)).toBe(true);
    expect(hasBlobImageSource(html)).toBe(true);
    expect(
      hasBlobImageSource(rewriteBlobImageSources(html, new Map([[URL_A, ID]]))),
    ).toBe(false);
    expect(hasBlobImageSource('<img src="data:image/png;base64,AAAA">')).toBe(false);
    expect(hasBlobImageSource('<a href="blob:http://x/1">x</a>')).toBe(false);
  });
});
