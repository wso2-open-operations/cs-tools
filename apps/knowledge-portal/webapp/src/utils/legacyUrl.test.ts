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
import { legacyIdToUuid, legacyKbTarget, previousArticleTarget, previousBrowseTarget } from "@utils/legacyUrl";

const HEX = "c824fa411b9599109871eac5624bcb6e";
const UUID = "c824fa41-1b95-9910-9871-eac5624bcb6e";

describe("legacyIdToUuid", () => {
  it("formats 32 hex chars as a lowercase uuid", () => {
    expect(legacyIdToUuid(HEX)).toBe(UUID);
    expect(legacyIdToUuid(HEX.toUpperCase())).toBe(UUID);
  });

  it("rejects anything else", () => {
    for (const bad of [null, "", HEX.slice(1), HEX + "0", "g" + HEX.slice(1), UUID, `${HEX}/../x`]) {
      expect(legacyIdToUuid(bad)).toBeNull();
    }
  });
});

describe("legacyKbTarget", () => {
  it("maps a knowledge base search", () => {
    expect(legacyKbTarget(`?id=kb_search&kb_knowledge_base=${HEX}`)).toBe(`/knowledge-bases/${UUID}`);
  });

  it("keeps a search term from q or keyword", () => {
    expect(legacyKbTarget(`?id=kb_search&kb_knowledge_base=${HEX}&q=a%20b`)).toBe(
      `/knowledge-bases/${UUID}?q=a%20b`,
    );
    expect(legacyKbTarget("?id=kb_search&keyword=oauth")).toBe("/knowledge-bases?q=oauth");
  });

  it("maps a search without a knowledge base to all articles", () => {
    expect(legacyKbTarget("?id=kb_search")).toBe("/knowledge-bases");
    expect(legacyKbTarget("?id=kb_search&kb_knowledge_base=")).toBe("/knowledge-bases");
  });

  it("maps an article view", () => {
    expect(legacyKbTarget(`?id=kb_article_view&sys_kb_id=${HEX}`)).toBe(`/knowledge-articles/${UUID}`);
  });

  it("returns null for anything unmappable", () => {
    for (const search of [
      "",
      "?id=kb_home",
      "?id=kb_search&kb_knowledge_base=nothex",
      "?id=kb_search&kb_knowledge_base=//evil.example",
      "?id=kb_article_view&sys_kb_id=bad",
      "?id=kb_article_view&sysparm_article=KB0012345",
      "?id=kb_article_view",
    ]) {
      expect(legacyKbTarget(search)).toBeNull();
    }
  });
});

describe("previous format targets", () => {
  it("maps /browse params", () => {
    expect(previousBrowseTarget("")).toBe("/knowledge-bases");
    expect(previousBrowseTarget("?q=x")).toBe("/knowledge-bases?q=x");
    expect(previousBrowseTarget("?knowledgeBaseId=kb_1&q=x")).toBe("/knowledge-bases/kb_1?q=x");
    expect(previousBrowseTarget("?knowledgeBaseId=//evil.example")).toBe("/knowledge-bases");
  });

  it("maps /articles ids", () => {
    expect(previousArticleTarget("abc")).toBe("/knowledge-articles/abc");
    expect(previousArticleTarget("bad id")).toBeNull();
    expect(previousArticleTarget(undefined)).toBeNull();
  });
});
