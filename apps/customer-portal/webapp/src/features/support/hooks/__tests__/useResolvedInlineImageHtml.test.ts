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

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAttachmentPreviews } from "@api/useAttachmentPreview";
import { useResolvedInlineImageHtml } from "@features/support/hooks/useResolvedInlineImageHtml";

// The global test setup replaces this hook with a passthrough; use the real one.
vi.unmock("@features/support/hooks/useResolvedInlineImageHtml");

vi.mock("@api/useAttachmentPreview", () => ({
  useAttachmentPreviews: vi.fn(),
}));

describe("useResolvedInlineImageHtml", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deduplicates .iix ids and replaces matching image sources", () => {
    const html =
      "<p>Hi</p><img src='/abc123abc123abc123abc123abc123ab.iix' /><img src='/abc123abc123abc123abc123abc123ab.iix' /><img src='/xyzxyzxyzxyzxyzxyzxyzxyzxyzxyzxy.iix' />";

    vi.mocked(useAttachmentPreviews).mockReturnValue({
      dataUrls: new Map([["abc123abc123abc123abc123abc123ab", "data:image/png;base64,AAA"]]),
      isLoading: false,
    });

    const { result } = renderHook(() => useResolvedInlineImageHtml(html));

    expect(useAttachmentPreviews).toHaveBeenCalledWith([
      "abc123abc123abc123abc123abc123ab",
      "xyzxyzxyzxyzxyzxyzxyzxyzxyzxyzxy",
    ],
      undefined,
    );
    expect(result.current.resolvedHtml).toContain("data:image/png;base64,AAA");
    expect(result.current.resolvedHtml).toContain("data-unresolved=\"true\"");
    expect(result.current.isLoading).toBe(false);
  });

  it("replaces every image with a not-available note, showing the UUID, when all fetches fail", () => {
    vi.mocked(useAttachmentPreviews).mockReturnValue({
      dataUrls: new Map(),
      isLoading: false,
    });

    const { result } = renderHook(() =>
      useResolvedInlineImageHtml(
        '<img src="https://wso2.service-now.com/d29f3f669773c350182338f11153af3f.iix">',
      ),
    );

    expect(result.current.resolvedHtml).not.toContain("service-now");
    expect(result.current.resolvedHtml).not.toContain("<img");
    expect(result.current.resolvedHtml).toContain("Image not available");
    expect(result.current.resolvedHtml).toContain(
      "ID: d29f3f66-9773-c350-1823-38f11153af3f",
    );
  });

  it("forwards the caseId to the attachment preview fetch as an authorization hint", () => {
    vi.mocked(useAttachmentPreviews).mockReturnValue({
      dataUrls: new Map(),
      isLoading: false,
    });

    renderHook(() =>
      useResolvedInlineImageHtml(
        '<img src="/abc123abc123abc123abc123abc123ab.iix">',
        "case-1",
      ),
    );

    expect(useAttachmentPreviews).toHaveBeenCalledWith(
      ["abc123abc123abc123abc123abc123ab"],
      "case-1",
    );
  });

  it("keeps loading false when html has no inline images", () => {
    vi.mocked(useAttachmentPreviews).mockReturnValue({
      dataUrls: new Map(),
      isLoading: true,
    });

    const { result } = renderHook(() =>
      useResolvedInlineImageHtml("<p>No images here</p>"),
    );

    expect(useAttachmentPreviews).toHaveBeenCalledWith([], undefined);
    expect(result.current.resolvedHtml).toBe("<p>No images here</p>");
    expect(result.current.isLoading).toBe(false);
  });

  describe("bare attachment-id src (migrated content)", () => {
    const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
    const HEX = UUID.replace(/-/g, "");
    const DATA = "data:image/png;base64,AAA";

    it("requests the 32-hex id for a bare hyphenated uuid and resolves it", () => {
      vi.mocked(useAttachmentPreviews).mockReturnValue({
        dataUrls: new Map([[HEX, DATA]]),
        isLoading: false,
      });
      const { result } = renderHook(() =>
        useResolvedInlineImageHtml(`<p><img src="/${UUID}"><br></p>`),
      );
      expect(useAttachmentPreviews).toHaveBeenCalledWith([HEX], undefined);
      expect(result.current.resolvedHtml).toContain(`src="${DATA}"`);
    });

    it.each([
      ["bare 32-hex with slash", `/${HEX}`],
      ["bare 32-hex without slash", HEX],
      ["bare uuid without slash", UUID],
      ["uppercase uuid", UUID.toUpperCase()],
    ])("resolves %s", (_name, src) => {
      vi.mocked(useAttachmentPreviews).mockReturnValue({
        dataUrls: new Map([[HEX, DATA]]),
        isLoading: false,
      });
      const { result } = renderHook(() =>
        useResolvedInlineImageHtml(`<img src="${src}">`),
      );
      expect(useAttachmentPreviews).toHaveBeenCalledWith([HEX], undefined);
      expect(result.current.resolvedHtml).toContain(`src="${DATA}"`);
    });

    it("dedupes the same attachment across bare and .iix forms", () => {
      vi.mocked(useAttachmentPreviews).mockReturnValue({
        dataUrls: new Map([[HEX, DATA]]),
        isLoading: false,
      });
      const html = `<img src="/${UUID}"><img src="/${HEX}.iix"><img src="/${HEX}">`;
      const { result } = renderHook(() => useResolvedInlineImageHtml(html));
      expect(useAttachmentPreviews).toHaveBeenCalledWith([HEX], undefined);
      expect(result.current.resolvedHtml.match(/data:image\/png/g)).toHaveLength(3);
    });

    it("strips an unresolved bare-id src like an unresolved .iix src", () => {
      vi.mocked(useAttachmentPreviews).mockReturnValue({
        dataUrls: new Map([["ffffffffffffffffffffffffffffffff", DATA]]),
        isLoading: false,
      });
      const { result } = renderHook(() =>
        useResolvedInlineImageHtml(`<img src="/${UUID}">`),
      );
      expect(result.current.resolvedHtml).toContain('data-unresolved="true"');
      expect(result.current.resolvedHtml).not.toContain("<img");
      // The id is shown (as a UUID) in the "not available" note, not as a src.
      expect(result.current.resolvedHtml).toContain(`ID: ${UUID}`);
    });

    it.each([
      ["query string", `/${UUID}?x=1`],
      ["extra path segment", `/img/${UUID}`],
      ["absolute url", `https://example.com/${UUID}`],
      ["protocol-relative", `//${UUID}`],
      ["other extension", `/${UUID}.png`],
      ["data uri", "data:image/png;base64,AAAA"],
    ])("does not fetch or rewrite a %s src", (_name, src) => {
      vi.mocked(useAttachmentPreviews).mockReturnValue({
        dataUrls: new Map([[HEX, DATA]]),
        isLoading: false,
      });
      const html = `<img src="${src}">`;
      const { result } = renderHook(() => useResolvedInlineImageHtml(html));
      expect(useAttachmentPreviews).toHaveBeenCalledWith([], undefined);
      expect(result.current.resolvedHtml).toBe(html);
    });
  });
});
