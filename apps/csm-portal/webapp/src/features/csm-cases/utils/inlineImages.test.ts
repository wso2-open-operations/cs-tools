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
    expect(out).toContain("Image unavailable");
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
