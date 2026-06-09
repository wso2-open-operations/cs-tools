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
import { deriveAltFromFilename, sanitizeUrl } from "./richTextEditor";

describe("sanitizeUrl", () => {
  it("allows safe protocols and anchors", () => {
    expect(sanitizeUrl("https://wso2.com")).toBe("https://wso2.com");
    expect(sanitizeUrl("http://example.com")).toBe("http://example.com");
    expect(sanitizeUrl("mailto:support@wso2.com")).toBe(
      "mailto:support@wso2.com",
    );
    expect(sanitizeUrl("tel:+15551234")).toBe("tel:+15551234");
    expect(sanitizeUrl("#section")).toBe("#section");
  });

  it("allows single-slash relative paths but rejects protocol-relative URLs", () => {
    expect(sanitizeUrl("/cases/123")).toBe("/cases/123");
    expect(sanitizeUrl("//evil.com")).toBe("");
  });

  it("allows base64 inline images", () => {
    const data = "data:image/png;base64,iVBORw0KGgo=";
    expect(sanitizeUrl(data)).toBe(data);
  });

  it("rejects dangerous protocols", () => {
    expect(sanitizeUrl("javascript:alert(1)")).toBe("");
    expect(sanitizeUrl("vbscript:msgbox(1)")).toBe("");
    expect(sanitizeUrl("data:text/html;base64,PHNjcmlwdD4=")).toBe("");
  });

  it("decodes &amp; and trims before testing", () => {
    expect(sanitizeUrl("  https://a.com/?x=1&amp;y=2  ")).toBe(
      "https://a.com/?x=1&y=2",
    );
  });
});

describe("deriveAltFromFilename", () => {
  it("extracts the base filename from an absolute URL", () => {
    expect(deriveAltFromFilename("https://cdn.example.com/img/diagram.png")).toBe(
      "diagram",
    );
  });

  it("strips query and hash for non-URL paths", () => {
    expect(deriveAltFromFilename("/uploads/screen-shot.jpeg?v=2")).toBe(
      "screen-shot",
    );
  });

  it("falls back to Image when no usable name is present", () => {
    expect(deriveAltFromFilename("")).toBe("Image");
  });

  it("decodes percent-encoded filenames", () => {
    expect(
      deriveAltFromFilename("https://cdn.example.com/img/screen%20shot.png"),
    ).toBe("screen shot");
    expect(deriveAltFromFilename("/uploads/my%2Bfile.jpeg")).toBe("my+file");
  });

  it("falls back to the raw segment when decoding fails", () => {
    // A malformed percent-escape must not throw.
    expect(deriveAltFromFilename("/uploads/bad%E0%A4name.png")).toBe(
      "bad%E0%A4name",
    );
  });
});
