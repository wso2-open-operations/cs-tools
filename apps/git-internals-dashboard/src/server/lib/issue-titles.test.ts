// src/server/lib/issue-titles.test.ts
import { describe, expect, it } from "vitest";
import { buildQuery } from "./issue-titles";

describe("buildQuery", () => {
  it("groups by repo with alias-safe names and escaped strings", () => {
    const q = buildQuery([
      { id: 1, owner: "wso2-enterprise", name: "wso2-iam-internal", number: 7366 },
      { id: 2, owner: "wso2-enterprise", name: "wso2-iam-internal", number: 12 },
      { id: 3, owner: "wso2-enterprise", name: "wso2-apim-internal", number: 99 },
    ]);
    expect(q).toContain('r0: repository(owner: "wso2-enterprise", name: "wso2-iam-internal")');
    expect(q).toContain("n7366: issue(number: 7366) { title }");
    expect(q).toContain('r1: repository(owner: "wso2-enterprise", name: "wso2-apim-internal")');
    expect(q).toContain("n99: issue(number: 99) { title }");
  });
});
