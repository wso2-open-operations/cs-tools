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
import { EMPTY_BANNER_CONTENT, buildBannerHtml, parseBannerHtml } from "@features/csm-admin/banners/bannerHtml";

describe("banner HTML", () => {
  it("builds a custom banner from the pasted HTML and reads it back as pasted", () => {
    const pasted = '<div style="background:#123456;color:#fff;line-height:3rem;text-align:center">Maintenance <b>tonight</b></div>';
    const html = buildBannerHtml({
      type: "custom",
      title: "",
      message: pasted,
      textColor: "#f9fafb",
      backgroundColor: "#1f2937",
      iconColor: "",
      imageUrl: "",
      linkUrl: "",
      closeable: true,
    });
    expect(html).toContain('data-banner="1"');
    expect(html).toContain('data-banner-type="custom"');
    expect(html).toContain("data-banner-custom");
    expect(html).toContain('data-close-banner="true"');
    expect(html).toContain("background:#123456");
    expect(html).not.toContain("data-bg=");
    expect(parseBannerHtml(html)).toMatchObject({ type: "custom", message: pasted, closeable: true });
  });

  it("stamps the banner id on a custom banner", () => {
    const html = buildBannerHtml(
      { ...EMPTY_BANNER_CONTENT, type: "custom", message: "<p>Hi</p>" },
      "11111111-1111-4111-8111-111111111111",
    );
    expect(html).toContain('data-id="11111111-1111-4111-8111-111111111111"');
  });

  it("removes scripts, event handlers, style tags and forms from pasted HTML", () => {
    const html = buildBannerHtml({
      ...EMPTY_BANNER_CONTENT,
      type: "custom",
      message:
        '<style>body{display:none}</style><script>alert(1)</script><form action="/x"><input name="a"></form><a href="javascript:alert(1)" onclick="alert(2)">go</a><span>kept</span>',
    });
    expect(html).not.toMatch(/<style|<script|<form|<input|onclick|javascript:/i);
    expect(html).toContain("<span>kept</span>");
  });

  it("keeps a link that opens in a new tab, safely", () => {
    const html = buildBannerHtml({
      ...EMPTY_BANNER_CONTENT,
      type: "custom",
      message: '<a href="https://example.com/status" target="_blank">Status</a>',
    });
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it("opens a custom banner saved with colours as HTML that keeps its look", () => {
    const legacy =
      '<div data-banner="1" data-banner-type="custom" data-text-color="#ffeeaa" data-bg="#223344" data-closeable="false" style="x">' +
      '<div data-banner-text style="margin:0;color:#ffeeaa"><p style="margin:0">Old notice</p></div></div>';
    const parsed = parseBannerHtml(legacy);
    expect(parsed.type).toBe("custom");
    expect(parsed.message).toContain("Old notice");
    expect(parsed.message).toContain("background-color:#223344");
    expect(parsed.message).toContain("color:#ffeeaa");
  });

  it("round-trips a text banner, including a close button", () => {
    const html = buildBannerHtml({
      type: "info",
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
    expect(html).toContain('data-banner-type="info"');
    expect(html).not.toContain("data-title=");
    expect(html).toContain("<strong>");
    expect(parseBannerHtml(html)).toMatchObject({
      type: "info",
      message: '<p style="margin:0">Hello <strong>"team"</strong></p>',
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
