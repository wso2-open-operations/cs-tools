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
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it } from "vitest";
import { LegacyKbRedirect, PreviousArticleRedirect, PreviousBrowseRedirect } from "@components/LegacyRedirect";

const Where = () => {
  const { pathname, search } = useLocation();
  return <div data-testid="where">{pathname + search}</div>;
};

// Same route shapes as App, with a probe in place of the real pages.
const finalLocation = (from: string): string | null => {
  render(
    <MemoryRouter initialEntries={[from]}>
      <Routes>
        <Route path="/" element={<Where />} />
        <Route path="/knowledge-bases" element={<Where />} />
        <Route path="/knowledge-bases/:id" element={<Where />} />
        <Route path="/knowledge-articles/:id" element={<Where />} />
        <Route path="/kb" element={<LegacyKbRedirect />} />
        <Route path="/browse" element={<PreviousBrowseRedirect />} />
        <Route path="/articles/:id" element={<PreviousArticleRedirect />} />
      </Routes>
    </MemoryRouter>,
  );
  return screen.getByTestId("where").textContent;
};

const HEX = "c824fa411b9599109871eac5624bcb6e";
const UUID = "c824fa41-1b95-9910-9871-eac5624bcb6e";

describe("legacy redirects", () => {
  it.each([
    [`/kb?id=kb_search&kb_knowledge_base=${HEX}`, `/knowledge-bases/${UUID}`],
    [`/kb?id=kb_search&kb_knowledge_base=${HEX}&q=jwt`, `/knowledge-bases/${UUID}?q=jwt`],
    ["/kb?id=kb_search", "/knowledge-bases"],
    [`/kb?id=kb_article_view&sys_kb_id=${HEX}`, `/knowledge-articles/${UUID}`],
    ["/kb?id=kb_article_view&sysparm_article=KB0012345", "/"],
    ["/kb?id=kb_search&kb_knowledge_base=zzz", "/"],
    ["/kb?id=unknown", "/"],
    ["/kb", "/"],
    ["/browse", "/knowledge-bases"],
    ["/browse?q=jwt", "/knowledge-bases?q=jwt"],
    ["/browse?knowledgeBaseId=kb_1&q=jwt", "/knowledge-bases/kb_1?q=jwt"],
    ["/browse?knowledgeBaseId=a/b", "/knowledge-bases"],
    ["/articles/abc", "/knowledge-articles/abc"],
    ["/articles/bad%20id", "/"],
  ])("%s -> %s", (from, to) => {
    expect(finalLocation(from)).toBe(to);
  });
});
