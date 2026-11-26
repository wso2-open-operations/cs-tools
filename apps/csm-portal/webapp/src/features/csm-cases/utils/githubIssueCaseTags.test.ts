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
import { githubIssueCaseTags } from "./githubIssueCaseTags";

describe("githubIssueCaseTags", () => {
  it("tags the case s_dip, the tag the case filters exclude on, when the issue is not a patch", () => {
    expect(githubIssueCaseTags({ reason: "default" })).toEqual(["s_dip"]);
    expect(githubIssueCaseTags({ reason: "default", issueTypeLabel: "Type/Discussion" })).toEqual([
      "s_dip",
    ]);
  });

  it("tags a Feature Request s_dip like a Discussion, plus migration when it is one", () => {
    expect(githubIssueCaseTags({ reason: "default", issueTypeLabel: "Type/Feature" })).toEqual([
      "s_dip",
    ]);
    expect(githubIssueCaseTags({ reason: "migration", issueTypeLabel: "Type/Feature" })).toEqual([
      "s_dip",
      "migration",
    ]);
  });

  it("tags the case patch, and not s_dip, for a patch issue", () => {
    expect(githubIssueCaseTags({ reason: "default", issueTypeLabel: "Type/Patch" })).toEqual([
      "patch",
    ]);
    expect(githubIssueCaseTags({ reason: "default", issueTypeLabel: " Type/Patch " })).toEqual([
      "patch",
    ]);
  });

  it("adds the regression and migration tags to a patch after patch, in the same order", () => {
    expect(
      githubIssueCaseTags({ reason: "migration", regression: true, issueTypeLabel: "Type/Patch" }),
    ).toEqual(["patch", "s_rg", "migration"]);
  });

  it("does not match a patch type by a partial or differently cased name", () => {
    for (const issueTypeLabel of ["Type/patch", "patch", "Type/Patch-ish", ""]) {
      expect(githubIssueCaseTags({ reason: "default", issueTypeLabel })).toEqual(["s_dip"]);
    }
  });

  it("never uses the old mistyped s_dp tag", () => {
    for (const payload of [
      { reason: "default" as const },
      { reason: "migration" as const, regression: true },
      { reason: "rd_ticket" as const },
    ]) {
      expect(githubIssueCaseTags(payload)).not.toContain("s_dp");
    }
  });

  it("adds s_rg for a regression", () => {
    expect(githubIssueCaseTags({ reason: "default", regression: true })).toEqual([
      "s_dip",
      "s_rg",
    ]);
  });

  it("does not add s_rg when regression is false or unset", () => {
    expect(githubIssueCaseTags({ reason: "default", regression: false })).toEqual(["s_dip"]);
    expect(githubIssueCaseTags({ reason: "default" })).toEqual(["s_dip"]);
  });

  it("adds migration for a migration issue", () => {
    expect(githubIssueCaseTags({ reason: "migration" })).toEqual(["s_dip", "migration"]);
  });

  it("adds both extras in a stable order when both apply", () => {
    expect(githubIssueCaseTags({ reason: "migration", regression: true })).toEqual([
      "s_dip",
      "s_rg",
      "migration",
    ]);
  });

  it("adds no migration tag for an R&D ticket", () => {
    expect(githubIssueCaseTags({ reason: "rd_ticket" })).toEqual(["s_dip"]);
  });
});
