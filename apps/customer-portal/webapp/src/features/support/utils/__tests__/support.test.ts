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
  compareByCreatedOnThenId,
  convertCodeTagsToHtml,
  extractInlineImageRefId,
  hasSingleCodeWrapper,
  hasSubmittableEditorContent,
  linkifyBareUrls,
  normalizeCaseTypeOptions,
  replaceInlineImageSources,
  stripCodeWrapper,
} from "@features/support/utils/support";

describe("extractInlineImageRefId", () => {
  it("extracts 32-char id from service-now style .iix paths", () => {
    expect(
      extractInlineImageRefId(
        "https://host/path/1234567890abcdef1234567890abcdef.iix?x=1",
      ),
    ).toBe("1234567890abcdef1234567890abcdef");
  });
});

describe("replaceInlineImageSources", () => {
  it("replaces matching inline image with preview url and keeps unmatched", () => {
    const html =
      "<p>Images</p><img src='/1234567890abcdef1234567890abcdef.iix' /><img src='/no-match.iix' />";
    const output = replaceInlineImageSources(html, [
      {
        id: "1234567890abcdef1234567890abcdef",
        previewUrl: "data:image/png;base64,AAA",
      },
    ]);

    expect(output).toContain('src="data:image/png;base64,AAA"');
    expect(output).toContain('src="/no-match.iix"');
  });
});

describe("code-wrapper helpers", () => {
  it("detects and strips only a single [code] wrapper", () => {
    const content = "[code]Hello[/code]";
    expect(hasSingleCodeWrapper(content)).toBe(true);
    expect(stripCodeWrapper(content)).toBe("Hello");
  });

  it("converts escaped or repeated [code] tags to html code blocks", () => {
    const content = "[code]A[/code][code]B[/code]";
    expect(convertCodeTagsToHtml(content)).toContain("<code>A</code>");
    expect(convertCodeTagsToHtml(content)).toContain("<code>B</code>");
  });
});

describe("hasSubmittableEditorContent", () => {
  it("accepts image-only rich text as submit-worthy", () => {
    expect(hasSubmittableEditorContent("<p><img src='x.png' /></p>")).toBe(true);
  });

  it("rejects empty html wrappers", () => {
    expect(hasSubmittableEditorContent("<p>   </p>")).toBe(false);
  });
});

describe("normalizeCaseTypeOptions", () => {
  it("filters announcement and merges query+incident as Case option", () => {
    const normalized = normalizeCaseTypeOptions([
      { id: "1", label: "Announcement" },
      { id: "2", label: "Incident" },
      { id: "3", label: "Query" },
      { id: "4", label: "Task" },
    ]);

    expect(normalized).toEqual([
      { label: "Task", value: "4" },
      { label: "Case", value: "2,3" },
    ]);
  });
});

describe("compareByCreatedOnThenId", () => {
  it("orders human comment before bot when timestamps tie", () => {
    const rows = [
      { id: "2", createdOn: "2026-02-01 10:00:00", createdBy: "Novera", type: "bot" },
      { id: "1", createdOn: "2026-02-01 10:00:00", createdBy: "Alice", type: "human" },
    ];
    rows.sort(compareByCreatedOnThenId);

    expect(rows.map((r) => r.id)).toEqual(["1", "2"]);
  });
});

describe("linkifyBareUrls", () => {
  it("linkifies plain urls but skips href values", () => {
    const html = '<a href="https://already.linked">ok</a> visit https://wso2.com/docs';
    const output = linkifyBareUrls(html);

    expect(output).toContain('<a href="https://already.linked">ok</a>');
    expect(output).toContain(
      '<a href="https://wso2.com/docs" target="_blank" rel="noopener noreferrer"',
    );
  });
});
