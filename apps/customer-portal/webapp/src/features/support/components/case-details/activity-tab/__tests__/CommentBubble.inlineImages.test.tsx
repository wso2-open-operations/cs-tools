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

import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { useAttachmentPreviews } from "@api/useAttachmentPreview";
import CommentBubble from "@case-details-activity/CommentBubble";
import type { CaseComment } from "@features/support/types/cases";

// The global test setup replaces this hook with a passthrough; use the real one
// so the bubble's sanitized HTML is actually resolved.
vi.unmock("@features/support/hooks/useResolvedInlineImageHtml");

vi.mock("@api/useAttachmentPreview", () => ({
  useAttachmentPreview: () => ({ data: undefined, isLoading: false }),
  useAttachmentPreviews: vi.fn(),
}));

const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
const HEX = UUID.replace(/-/g, "");
const DATA = "data:image/png;base64,AAAA";

function renderBubble(content: string) {
  const comment: CaseComment = {
    id: "comment-1",
    content,
    type: "comments",
    createdOn: "2026-02-12 11:15:42",
    createdBy: "support-engineer@wso2.com",
    isEscalated: false,
  };
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <CommentBubble
          comment={comment}
          isCurrentUser={false}
          primaryBg="rgba(250,123,63,0.1)"
        />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

describe("CommentBubble inline images (migrated content)", () => {
  beforeEach(() => {
    vi.mocked(useAttachmentPreviews).mockReset();
    vi.mocked(useAttachmentPreviews).mockReturnValue({
      dataUrls: new Map([[HEX, DATA]]),
      removedIds: new Set<string>(),
      isLoading: false,
    });
  });

  it("resolves a bare-uuid inline image inside a [code] wrapper", () => {
    const { container } = renderBubble(
      `[code]<p><img src="/${UUID}"><br></p>[/code]`,
    );
    expect(vi.mocked(useAttachmentPreviews).mock.calls.at(-1)?.[0]).toEqual([HEX]);
    expect(container.querySelector("img")?.getAttribute("src")).toBe(DATA);
  });

  it("still resolves the .iix form of the same attachment", () => {
    const { container } = renderBubble(
      `[code]<p><img src="/${HEX}.iix"><br></p>[/code]`,
    );
    expect(vi.mocked(useAttachmentPreviews).mock.calls.at(-1)?.[0]).toEqual([HEX]);
    expect(container.querySelector("img")?.getAttribute("src")).toBe(DATA);
  });
});
