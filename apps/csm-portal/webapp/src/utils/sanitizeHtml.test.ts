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

import DOMPurify from "dompurify";
import { describe, expect, it } from "vitest";
import {
  escapeHtml,
  hasVisibleContent,
  isBlankHtml,
  looksLikeHtml,
  sanitizeAnnouncementHtml,
  sanitizeDescriptionHtml,
  sanitizeRichTextHtml,
  sanitizeStructuredHtml,
  stripLightModeInlineStyles,
} from "./sanitizeHtml";

describe("sanitizeRichTextHtml", () => {
  it("forces rel=noopener noreferrer on target=_blank anchors when target is preserved", () => {
    // DOMPurify's default ALLOWED_ATTR doesn't include `target`, so a bare
    // `sanitizeRichTextHtml()` call already strips it (and with it, the need
    // for a hardening `rel`) — this exercises the module-load hook itself via
    // a config that keeps `target`, which is what any caller opting into
    // `target="_blank"` output (e.g. a future ADD_ATTR override) would rely on.
    const out = DOMPurify.sanitize(
      '<a href="https://example.com" target="_blank">link</a>',
      { ADD_ATTR: ["target"] },
    );
    expect(out).toContain('rel="noopener noreferrer"');
  });

  it("strips the target attribute by default (so no dangling target=_blank ships unhardened)", () => {
    const out = sanitizeRichTextHtml(
      '<a href="https://example.com" target="_blank">link</a>',
    );
    expect(out).not.toContain("target=");
  });

  it("strips script tags", () => {
    const out = sanitizeRichTextHtml("<p>hi</p><script>alert(1)</script>");
    expect(out).not.toContain("<script>");
  });

  it("keeps tables and code blocks (permissive comment policy)", () => {
    const out = sanitizeRichTextHtml("<table><tr><td>x</td></tr></table><code>y</code>");
    expect(out).toContain("<table>");
    expect(out).toContain("<code>");
  });

  it("keeps the call-request in-app marker's data attribute, role, and tabindex", () => {
    // `replaceCallRequestLinks` runs *after* `sanitizeRichTextHtml` in
    // `CsmCaseCommentBubble`'s actual pipeline, so this marker is never
    // itself passed back through DOMPurify there — but it's still worth
    // pinning that DOMPurify's default policy (`ALLOW_DATA_ATTR` on by
    // default) wouldn't strip it if that ordering ever changed.
    const out = sanitizeRichTextHtml(
      '<span data-call-request-sysid="7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41" role="button" tabindex="0">CTASK0012345</span>',
    );
    expect(out).toContain('data-call-request-sysid="7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41"');
    expect(out).toContain('role="button"');
    expect(out).toContain('tabindex="0"');
  });

  it("keeps the alert/smart-alert in-app marker's data attributes, role, and tabindex", () => {
    const out = sanitizeRichTextHtml(
      '<span data-sn-link-type="alert" data-sn-link-id="7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41" role="button" tabindex="0">View alert</span>',
    );
    expect(out).toContain('data-sn-link-type="alert"');
    expect(out).toContain('data-sn-link-id="7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41"');
    expect(out).toContain('role="button"');
    expect(out).toContain('tabindex="0"');
  });
});

describe("sanitizeDescriptionHtml", () => {
  it("strips tables and code blocks", () => {
    const out = sanitizeDescriptionHtml(
      "<p>desc</p><table><tr><td>x</td></tr></table><pre><code>y</code></pre>",
    );
    expect(out).toContain("<p>desc</p>");
    expect(out).not.toContain("<table>");
    expect(out).not.toContain("<pre>");
    expect(out).not.toContain("<code>");
  });
});

describe("isBlankHtml", () => {
  it("is true for an empty paragraph", () => {
    expect(isBlankHtml("<p></p>")).toBe(true);
  });

  it("is false for a description with text", () => {
    expect(isBlankHtml("<p>Some description</p>")).toBe(false);
  });
});

describe("hasVisibleContent", () => {
  it("is false for what an empty or cleared editor sends", () => {
    expect(hasVisibleContent("")).toBe(false);
    expect(hasVisibleContent("<p></p>")).toBe(false);
    expect(hasVisibleContent("<p><br></p>")).toBe(false);
    expect(hasVisibleContent("<p>&nbsp;</p>")).toBe(false);
  });

  it("is true for text", () => {
    expect(hasVisibleContent("<p>Some description</p>")).toBe(true);
  });

  it("counts an image-only body as content, where isBlankHtml does not", () => {
    const imageOnly = '<p><img src="data:image/png;base64,AAAA" alt=""></p>';
    expect(isBlankHtml(imageOnly)).toBe(true);
    expect(hasVisibleContent(imageOnly)).toBe(true);
    expect(hasVisibleContent('<IMG SRC="/abc123.iix">')).toBe(true);
  });
});

describe("stripLightModeInlineStyles", () => {
  it("removes a pure-white inline background declaration", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #ffffff; color: red;">x</div>',
    );
    expect(out).not.toContain("background-color");
    expect(out).toContain("color: red");
  });

  it("removes a near-white rgb background", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background: rgb(250, 250, 250);">x</div>',
    );
    expect(out).not.toContain("rgb(250");
  });

  it("removes a dark text color declaration", () => {
    const out = stripLightModeInlineStyles('<span style="color: #000000;">x</span>');
    expect(out).not.toContain("color");
  });

  it("leaves other declarations untouched", () => {
    const out = stripLightModeInlineStyles(
      '<div style="border: 1px solid #333; padding: 4px;">x</div>',
    );
    expect(out).toContain("border: 1px solid #333");
    expect(out).toContain("padding: 4px");
  });

  it("handles single-quoted style attributes the same as double-quoted", () => {
    const out = stripLightModeInlineStyles(
      "<div style='background: #ffffff; color: red;'>x</div>",
    );
    expect(out).not.toContain("background");
    expect(out).toContain("color: red");
  });

  it("does not treat a bright #1xxxxx/#2xxxxx color as dark", () => {
    const blue = stripLightModeInlineStyles('<span style="color: #1f75fe;">x</span>');
    expect(blue).toContain("color: #1f75fe");
    const cyan = stripLightModeInlineStyles('<span style="color: #2fffff;">x</span>');
    expect(cyan).toContain("color: #2fffff");
  });

  it("removes a light pastel background (e.g. a ServiceNow call-note highlight)", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #bce4e8; padding: 0.01em 16px;">x</div>',
    );
    expect(out).not.toContain("background-color");
    expect(out).toContain("padding: 0.01em 16px");
  });

  it("removes a mid-gray background whose contrast against light text is below WCAG AA", () => {
    // #808080 (luminance ~0.216) contrasts with white text at ~3.95:1, below
    // the 4.5:1 AA minimum for normal text — a fixed 0.55 luminance cutoff
    // missed this; the contrast-derived threshold must catch it.
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #808080;">x</div>',
    );
    expect(out).not.toContain("background-color");
  });

  it("still removes near-white backgrounds (no regression)", () => {
    const hex = stripLightModeInlineStyles('<div style="background-color: #f4f4f4;">x</div>');
    expect(hex).not.toContain("background-color");
    const rgb = stripLightModeInlineStyles('<div style="background: rgb(250, 250, 250);">x</div>');
    expect(rgb).not.toContain("rgb(250");
  });

  it("leaves a dark/saturated background alone", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #1a1a1a; color: red;">x</div>',
    );
    expect(out).toContain("background-color: #1a1a1a");
    expect(out).toContain("color: red");
  });
});

describe("escapeHtml", () => {
  it("escapes the HTML-significant characters", () => {
    expect(escapeHtml(`&<>"'`)).toBe("&amp;&lt;&gt;&quot;&#039;");
  });

  it("escapes the ampersand first, so an escaped entity is not double-escaped wrongly", () => {
    expect(escapeHtml("a & <b>")).toBe("a &amp; &lt;b&gt;");
  });
});


describe("looksLikeHtml", () => {
  it("detects real markup", () => {
    expect(looksLikeHtml("<p>hi</p>")).toBe(true);
    expect(looksLikeHtml("<table><tr><td>a</td></tr></table>")).toBe(true);
    expect(looksLikeHtml("line one<br>line two")).toBe(true);
  });

  it("leaves plain text with angle brackets alone", () => {
    expect(looksLikeHtml("Use <token> in the command")).toBe(false);
    expect(looksLikeHtml("if a <b and c> d")).toBe(false);
    expect(looksLikeHtml("1 < 2 and 3 > 2")).toBe(false);
    expect(looksLikeHtml("kubectl get pod <pod-name> -n <namespace>")).toBe(false);
  });
});

describe("sanitizeStructuredHtml", () => {
  it("keeps table structure and text emphasis", () => {
    const out = sanitizeStructuredHtml(
      '<table><tbody><tr><td style="font-weight:bold;width:30%">k</td><td>v</td></tr></tbody></table>',
    );
    expect(out).toContain("<table>");
    expect(out).toContain("font-weight:bold");
    expect(out).toContain("width:30%");
  });

  it("drops style elements (and their CSS text), images and form controls", () => {
    const out = sanitizeStructuredHtml(
      '<style>body{display:none}</style><img src="https://t.example/p.png"><form action="https://x"><input name="a"><button>go</button></form><p>ok</p>',
    );
    expect(out).not.toMatch(/<style|<img|<form|<input|<button/i);
    expect(out).not.toContain("display:none");
    expect(out).toContain("<p>ok</p>");
  });

  it("filters inline CSS to the allowlist (no background, colour, url(), position)", () => {
    const out = sanitizeStructuredHtml(
      '<p style="background:url(https://t.example/x);color:red;position:fixed;font-style:italic;width:calc(100% - 1px)">x</p>',
    );
    expect(out).toContain("font-style:italic");
    expect(out).not.toMatch(/background|color|position|url\(|calc/i);
  });

  it("removes the style attribute when nothing is allowed", () => {
    expect(sanitizeStructuredHtml('<p style="color:red">x</p>')).toBe("<p>x</p>");
  });

  it("opens safe links in a new tab and strips javascript: links", () => {
    const out = sanitizeStructuredHtml('<a href="https://a.example/x">a</a><a href="javascript:alert(1)">b</a>');
    expect(out).toContain('href="https://a.example/x"');
    expect(out).toContain('target="_blank"');
    expect(out).toContain('rel="noopener noreferrer"');
    expect(out).not.toContain("javascript:");
  });

  it("does not change what the shared policy does elsewhere (hook is scoped)", () => {
    expect(sanitizeRichTextHtml('<p style="color:red">x</p>')).toContain("color:red");
  });
});

describe("sanitizeAnnouncementHtml", () => {
  const anchors = (html: string): HTMLAnchorElement[] => [
    ...new DOMParser().parseFromString(html, "text/html").querySelectorAll("a"),
  ];

  it("keeps target=_blank and always sets rel=noopener noreferrer", () => {
    const [a, b] = anchors(
      sanitizeAnnouncementHtml(
        '<a href="https://x.test/a" target="_blank">a</a><a href="https://x.test/b" target="_blank" rel="opener">b</a>',
      ),
    );
    for (const link of [a, b]) {
      expect(link.getAttribute("target")).toBe("_blank");
      expect(link.getAttribute("rel")).toBe("noopener noreferrer");
    }
  });

  it("accepts the keyword in any case", () => {
    const [a] = anchors(sanitizeAnnouncementHtml('<a href="https://x.test" target="_BLANK">a</a>'));
    expect(a.getAttribute("target")).toBe("_blank");
  });

  it("leaves a link without a target alone", () => {
    const [a] = anchors(sanitizeAnnouncementHtml('<a href="https://x.test">a</a>'));
    expect(a.hasAttribute("target")).toBe(false);
  });

  it("drops any other target (_top, _parent, a frame name)", () => {
    const links = anchors(
      sanitizeAnnouncementHtml(
        '<a href="https://x.test/1" target="_top">1</a><a href="https://x.test/2" target="_parent">2</a><a href="https://x.test/3" target="frame">3</a>',
      ),
    );
    expect(links).toHaveLength(3);
    for (const link of links) expect(link.hasAttribute("target")).toBe(false);
  });

  it("keeps a target on links only, not on forms or image maps", () => {
    const out = sanitizeAnnouncementHtml(
      '<a href="https://x.test" target="_blank">a</a><form action="https://x.test" target="_blank"><input name="q"></form>' +
        '<map name="m"><area href="https://x.test" target="_blank" shape="rect" coords="0,0,1,1"></map>',
    );
    const doc = new DOMParser().parseFromString(out, "text/html");
    expect(doc.querySelector("a")?.getAttribute("target")).toBe("_blank");
    expect(doc.querySelectorAll("[target]")).toHaveLength(1);
  });

  it("still removes scripts, event handlers and javascript: links", () => {
    const out = sanitizeAnnouncementHtml(
      '<a href="javascript:alert(1)" target="_blank" onclick="alert(2)">x</a><script>alert(3)</script>',
    );
    expect(out).not.toMatch(/javascript:|onclick|<script/i);
    expect(out).toContain("x");
  });

  it("keeps the styling the default policy keeps", () => {
    expect(sanitizeAnnouncementHtml('<div style="padding:15px;border-radius:12px">x</div>')).toContain("border-radius:12px");
  });

  it("does not change how the shared policy treats target elsewhere", () => {
    expect(sanitizeRichTextHtml('<a href="https://x.test" target="_blank">a</a>')).not.toContain("target=");
  });

  it("returns exactly what the default sanitizer returns for HTML with no link target", () => {
    for (const html of [
      "plain text & more",
      "a < b and c > d",
      "",
      "<p>Hi &amp; bye</p>",
      '<a href="https://x.test">no target</a>',
      '<div style="padding:15px;border-radius:12px"><span>x</span></div><script>alert(1)</script>',
      "<table><tbody><tr><td>13</td></tr></tbody></table>",
      '<p>the word target in text, <a href="https://x.test">link</a></p>',
    ]) {
      expect(sanitizeAnnouncementHtml(html)).toBe(DOMPurify.sanitize(html));
    }
  });
});
