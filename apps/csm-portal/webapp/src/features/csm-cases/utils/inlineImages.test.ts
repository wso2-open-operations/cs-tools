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
  extractIixAttachmentIds,
  extractInlineImageRefId,
  isInlineImageRefSrc,
  isRawBase64ImageSrc,
  replaceInlineImageSrcs,
  sysidToUuid,
} from "@features/csm-cases/utils/inlineImages";

const SYSID = "0123456789abcdef0123456789abcdef";

describe("replaceInlineImageSrcs", () => {
  it("replaces a resolved reference with its data URL", () => {
    const html = `<p>see <img src="/inline/${SYSID}.iix"></p>`;
    const out = replaceInlineImageSrcs(
      html,
      new Map([[SYSID, "data:image/png;base64,AAAA"]]),
    );
    expect(out).toContain('src="data:image/png;base64,AAAA"');
    expect(out).not.toContain(".iix");
  });

  it("shows a permission placeholder for a reference in deniedIds, not a blank img", () => {
    const html = `<img src="${SYSID}.iix">`;
    const out = replaceInlineImageSrcs(html, new Map(), new Set([SYSID]));
    expect(out).not.toContain("<img");
    expect(out).toContain('data-unresolved-reason="permission"');
    expect(out).toContain("You don't have permission to view this image");
  });

  it("shows a generic placeholder for an unresolved reference not in deniedIds", () => {
    const html = `<img src="${SYSID}.iix">`;
    const out = replaceInlineImageSrcs(html, new Map(), new Set());
    expect(out).not.toContain("<img");
    expect(out).toContain('data-unresolved-reason="error"');
    expect(out).toContain("This image is not available.");
  });

  it("shows a removed placeholder for a reference in removedIds, without the id", () => {
    const html = `<img src="${SYSID}.iix">`;
    const out = replaceInlineImageSrcs(html, new Map(), undefined, false, new Set([SYSID]));
    expect(out).not.toContain("<img");
    expect(out).toContain('data-unresolved-reason="removed"');
    expect(out).toContain("This image was removed.");
    expect(out).not.toContain(SYSID);
  });

  it("defaults to the generic placeholder when deniedIds is omitted entirely", () => {
    const html = `<img src="${SYSID}.iix">`;
    const out = replaceInlineImageSrcs(html, new Map());
    expect(out).toContain('data-unresolved-reason="error"');
  });

  it("leaves a non-.iix img tag untouched", () => {
    const html = '<img src="https://example.com/logo.png">';
    expect(replaceInlineImageSrcs(html, new Map())).toBe(html);
  });

  it("resolves several references in one document independently", () => {
    const other = "fedcba9876543210fedcba9876543210";
    const html = `<img src="${SYSID}.iix"><img src="${other}.iix">`;
    const out = replaceInlineImageSrcs(
      html,
      new Map([[SYSID, "data:image/png;base64,AAAA"]]),
      new Set([other]),
    );
    expect(out).toContain('src="data:image/png;base64,AAAA"');
    expect(out).toContain('data-unresolved-reason="permission"');
  });

  it("leaves a raw base64 image untouched when denyRawBase64 is not set", () => {
    const html = '<img src="data:image/png;base64,AAAA">';
    expect(replaceInlineImageSrcs(html, new Map())).toBe(html);
  });

  it("hides a raw base64 image behind the permission placeholder when denyRawBase64 is true", () => {
    const html = '<p>see <img src="data:image/png;base64,AAAA"></p>';
    const out = replaceInlineImageSrcs(html, new Map(), undefined, true);
    expect(out).not.toContain("<img");
    expect(out).toContain('data-unresolved-reason="permission"');
    expect(out).toContain("You don't have permission to view this image");
  });

  it("denyRawBase64 does not affect a .iix reference the caller can resolve", () => {
    const html = `<img src="${SYSID}.iix">`;
    const out = replaceInlineImageSrcs(
      html,
      new Map([[SYSID, "data:image/png;base64,AAAA"]]),
      new Set(),
      true,
    );
    expect(out).toContain('src="data:image/png;base64,AAAA"');
  });

  it("denyRawBase64 does not touch a non-image, non-.iix src", () => {
    const html = '<img src="https://example.com/logo.png">';
    expect(replaceInlineImageSrcs(html, new Map(), undefined, true)).toBe(html);
  });

  // Regression test: a regex-based "scan for src=" grammar can be fooled by
  // src-shaped text inside a DIFFERENT attribute (e.g. alt) that appears
  // earlier in the tag than the real src -- it would treat the alt text as
  // the image's src, leaving the real src (and its real base64 payload)
  // completely unexamined. DOMParser-based parsing (see parseImgElements's
  // own doc comment) can't be confused this way: img.getAttribute("src") can
  // only ever return the actual src attribute.
  it("is not fooled by src-shaped text inside a different attribute (e.g. alt)", () => {
    const html = `<img alt='look at this src="data:image/png;base64,DECOY"' src="data:image/png;base64,REALSECRET">`;
    const out = replaceInlineImageSrcs(html, new Map(), undefined, true);
    expect(out).not.toContain("REALSECRET");
    expect(out).not.toContain("DECOY");
    expect(out).not.toContain("<img");
    expect(out).toContain('data-unresolved-reason="permission"');
  });

  it("the same alt-confusion input still resolves correctly for a .iix reference", () => {
    const html = `<img alt='src="${SYSID}.iix"' src="${SYSID}.iix">`;
    const out = replaceInlineImageSrcs(
      html,
      new Map([[SYSID, "data:image/png;base64,REAL"]]),
    );
    expect(out).toContain('src="data:image/png;base64,REAL"');
  });
});

describe("isRawBase64ImageSrc", () => {
  it("matches a base64-embedded image src", () => {
    expect(isRawBase64ImageSrc("data:image/png;base64,AAAA")).toBe(true);
  });

  it("rejects a .iix reference, a real URL, and a non-image data URI", () => {
    expect(isRawBase64ImageSrc(`${SYSID}.iix`)).toBe(false);
    expect(isRawBase64ImageSrc("https://example.com/logo.png")).toBe(false);
    expect(isRawBase64ImageSrc("data:text/plain;base64,AAAA")).toBe(false);
  });
});

describe("extractIixAttachmentIds / extractInlineImageRefId", () => {
  it("extracts a bare 32-char sysid from a full path", () => {
    expect(extractInlineImageRefId(`/inline/${SYSID}.iix`)).toBe(SYSID);
  });

  it("extracts every distinct .iix id referenced in a document, in order, deduplicated", () => {
    const other = "fedcba9876543210fedcba9876543210";
    const html = `<img src="${SYSID}.iix"><img src="${other}.iix"><img src="${SYSID}.iix">`;
    expect(extractIixAttachmentIds(html)).toEqual([SYSID, other]);
  });

  it("ignores an img tag whose src is not a .iix reference", () => {
    expect(
      extractIixAttachmentIds('<img src="https://example.com/logo.png">'),
    ).toEqual([]);
  });

  it("extracts the real src's id even when a different attribute contains src-shaped text first", () => {
    const other = "fedcba9876543210fedcba9876543210";
    const html = `<img alt='src="${other}.iix"' src="${SYSID}.iix">`;
    expect(extractIixAttachmentIds(html)).toEqual([SYSID]);
  });
});

describe("sysidToUuid", () => {
  it("re-inserts hyphens into a bare 32-char sysid", () => {
    expect(sysidToUuid(SYSID)).toBe(
      "01234567-89ab-cdef-0123-456789abcdef",
    );
  });

  it("returns an already-differently-shaped id unchanged", () => {
    expect(sysidToUuid("not-a-sysid")).toBe("not-a-sysid");
  });
});

// Content migrated from the legacy data source carries inline images as a
// bare attachment id (`<img src="/<uuid>">`), with no `.iix` suffix.
describe("bare attachment-id srcs (migrated content)", () => {
  const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
  const HEX = UUID.replace(/-/g, "");
  const DATA = "data:image/png;base64,AAAA";

  it.each([
    ["hyphenated uuid with leading slash", `/${UUID}`],
    ["hyphenated uuid without leading slash", UUID],
    ["uppercase hyphenated uuid", `/${UUID.toUpperCase()}`],
    ["32-hex id with leading slash", `/${HEX}`],
    ["32-hex id without leading slash", HEX],
    ["hyphenated uuid with .iix", `/${UUID}.iix`],
    ["surrounding whitespace", `  /${UUID} `],
  ])("treats %s as an attachment reference and resolves it", (_n, src) => {
    expect(isInlineImageRefSrc(src)).toBe(true);
    expect(extractInlineImageRefId(src)).toBe(HEX);
    expect(extractIixAttachmentIds(`<img src="${src}">`)).toEqual([HEX]);
    const out = replaceInlineImageSrcs(
      `<p><img src="${src}"><br></p>`,
      new Map([[HEX, DATA]]),
    );
    expect(out).toContain(`src="${DATA}"`);
  });

  it("hands the hook an id that sysidToUuid turns back into the canonical uuid", () => {
    expect(sysidToUuid(extractInlineImageRefId(`/${UUID}`))).toBe(UUID);
    expect(sysidToUuid(UUID)).toBe(UUID);
  });

  it("keeps .iix references working", () => {
    expect(extractIixAttachmentIds(`<img src="/${HEX}.iix">`)).toEqual([HEX]);
  });

  it.each([
    ["https URL", `https://example.com/${UUID}`],
    ["path prefix", `/some/path/${UUID}`],
    ["query string", `/${UUID}?x=1`],
    ["other extension", `/${UUID}.png`],
    ["short id", "/0f15cbcc"],
    ["data URI", "data:image/png;base64,AAAA"],
    ["double slash", `//${UUID}`],
  ])("leaves %s untouched", (_n, src) => {
    expect(isInlineImageRefSrc(src)).toBe(false);
    const html = `<img src="${src}">`;
    expect(extractIixAttachmentIds(html)).toEqual([]);
    expect(replaceInlineImageSrcs(html, new Map())).toBe(html);
  });

  it("dedupes a bare id against the .iix form of the same attachment", () => {
    const other = "fedcba9876543210fedcba9876543210";
    const html = `<img src="/${UUID}"><img src="/${HEX}.iix"><img src="${other}.iix"><img src="${UUID}">`;
    expect(extractIixAttachmentIds(html)).toEqual([HEX, other]);
  });

  it("shows the permission placeholder for a denied bare id", () => {
    const out = replaceInlineImageSrcs(
      `<img src="/${UUID}">`,
      new Map(),
      new Set([HEX]),
    );
    expect(out).not.toContain("<img");
    expect(out).toContain('data-unresolved-reason="permission"');
  });

  it("shows the unavailable placeholder for an unresolved bare id", () => {
    const out = replaceInlineImageSrcs(`<img src="/${UUID}">`, new Map());
    expect(out).not.toContain("<img");
    expect(out).toContain('data-unresolved-reason="error"');
    expect(out).toContain("This image is not available.");
  });
});
