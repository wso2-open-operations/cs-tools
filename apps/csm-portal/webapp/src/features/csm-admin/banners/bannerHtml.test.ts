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
import { buildBannerHtml, parseBannerHtml } from "@features/csm-admin/banners/bannerHtml";

describe("banner HTML", () => {
  it("round-trips the fields the engineer fills in, including a close button", () => {
    const html = buildBannerHtml({
      type: "custom",
      title: "Hello",
      message: '<p>Hello <strong>"team"</strong></p>',
      textColor: "#f9fafb",
      backgroundColor: "#1f2937",
      iconColor: "",
      imageUrl: "",
      linkUrl: "",
      closeable: true,
    });
    expect(html).toContain('data-close-banner="true"');
    expect(html).toContain('data-banner-type="custom"');
    expect(html).not.toContain("data-title=");
    expect(html).not.toContain("data-banner-title");
    expect(html).toContain("<strong>");
    expect(html).not.toContain("<img");
    expect(parseBannerHtml(html)).toEqual({
      type: "custom",
      title: "",
      message: '<p style="margin:0">Hello <strong>"team"</strong></p>',
      textColor: "#f9fafb",
      backgroundColor: "#1f2937",
      iconColor: "",
      imageUrl: "",
      linkUrl: "",
      closeable: true,
    });
  });

  it("builds an image banner as a full-width picture that opens the link", () => {
    const html = buildBannerHtml({
      type: "image",
      title: "",
      message: "",
      textColor: "#ffffff",
      iconColor: "",
      backgroundColor: "#000000",
      imageUrl: "https://cdn.example.com/event-banner.png",
      linkUrl: "https://example.com",
      closeable: false,
    }, "11111111-1111-4111-8111-111111111111");
    expect(html).toContain('data-id="11111111-1111-4111-8111-111111111111"');
    expect(html).toContain('data-banner-type="image"');
    expect(html).toContain("height:3rem");
    expect(html).toContain("width:100%");
    expect(html).toContain('href="https://example.com"');
    expect(html).toContain('rel="noopener noreferrer"');
    expect(parseBannerHtml(html).type).toBe("image");
  });

  it("keeps bold text and drops a script", () => {
    const html = buildBannerHtml({
      type: "info",
      title: "",
      message: "<p>Hello <strong>engineers</strong></p><script>alert(1)</script>",
      textColor: "#ffffff",
      iconColor: "",
      backgroundColor: "#01579b",
      imageUrl: "",
      linkUrl: "",
      closeable: false,
    });
    expect(html).toContain("<strong>engineers</strong>");
    expect(html).toContain("<p style=\"margin:0\">");
    expect(html).not.toContain("script");
    expect(parseBannerHtml(html).message).toContain("<strong>engineers</strong>");
  });

  it("draws a warning with the Oxygen alert icon", () => {
    const html = buildBannerHtml({
      type: "warning",
      title: "",
      message: "Development environment",
      textColor: "#663c00",
      backgroundColor: "#fff4e5",
      iconColor: "#ed6c02",
      imageUrl: "",
      linkUrl: "",
      closeable: false,
    });
    expect(html).toContain("M12 5.99L19.53 19H4.47");
    expect(html).toContain("fill:#ed6c02");
    expect(html).toContain("background-color:#fff4e5");
    expect(html).toContain("height:3rem");
    expect(html).not.toContain("min-height");
  });

  it("drops a link that is not http or https", () => {
    const html = buildBannerHtml({
      type: "warning",
      title: "",
      message: "Hi",
      textColor: "#000000",
      iconColor: "",
      backgroundColor: "#ed6c02",
      imageUrl: "javascript:alert(1)",
      linkUrl: "javascript:alert(1)",
      closeable: false,
    });
    expect(html).not.toContain("javascript:");
    expect(parseBannerHtml(html).imageUrl).toBe("");
    expect(parseBannerHtml(html).linkUrl).toBe("");
  });
});
