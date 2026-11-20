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
import type { BeCommentTemplate } from "@api/backend/types";
import {
  filterTemplates,
  isBlankBody,
  templatePlainText,
  templatesForMode,
} from "./commentTemplates";

const t = (id: string, name: string, appliesTo: BeCommentTemplate["appliesTo"], body = "<p>Text</p>"): BeCommentTemplate => ({
  id,
  name,
  body,
  appliesTo,
  shared: false,
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-01T00:00:00Z",
});

const all = [t("1", "Ack", "COMMENT"), t("2", "Internal", "WORK_NOTE"), t("3", "Either", "BOTH")];

describe("templatesForMode", () => {
  it("never offers a work note template for a public comment", () => {
    expect(templatesForMode(all, false).map((x) => x.name)).toEqual(["Ack", "Either"]);
  });
  it("offers work note and both templates for a work note", () => {
    expect(templatesForMode(all, true).map((x) => x.name)).toEqual(["Internal", "Either"]);
  });
});

describe("filterTemplates", () => {
  const list = [t("1", "Closing note", "BOTH", "<p>Thanks for your patience.</p>"), t("2", "Ack", "BOTH", "<p>We are on it.</p>")];
  it("matches the name or the text, ignoring case", () => {
    expect(filterTemplates(list, "closing").map((x) => x.id)).toEqual(["1"]);
    expect(filterTemplates(list, "ON IT").map((x) => x.id)).toEqual(["2"]);
  });
  it("returns everything for a blank query", () => {
    expect(filterTemplates(list, "  ")).toHaveLength(2);
  });
});

describe("templatePlainText and isBlankBody", () => {
  it("drops markup and collapses whitespace", () => {
    expect(templatePlainText("<p>First</p>\n<p>  second <b>part</b></p>")).toBe("First second part");
  });
  it("treats an empty editor paragraph as blank", () => {
    expect(isBlankBody("<p><br></p>")).toBe(true);
    expect(isBlankBody("<p>Hi</p>")).toBe(false);
  });
});
