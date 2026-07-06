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
    ]);
    expect(result.current.resolvedHtml).toContain("data:image/png;base64,AAA");
    expect(result.current.resolvedHtml).toContain("data-unresolved=\"true\"");
    expect(result.current.isLoading).toBe(false);
  });

  it("keeps loading false when html has no inline images", () => {
    vi.mocked(useAttachmentPreviews).mockReturnValue({
      dataUrls: new Map(),
      isLoading: true,
    });

    const { result } = renderHook(() =>
      useResolvedInlineImageHtml("<p>No images here</p>"),
    );

    expect(useAttachmentPreviews).toHaveBeenCalledWith([]);
    expect(result.current.resolvedHtml).toBe("<p>No images here</p>");
    expect(result.current.isLoading).toBe(false);
  });
});
