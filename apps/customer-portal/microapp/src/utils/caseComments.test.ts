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
import { commentEntries, createdCommentDto, toCommentDto } from "./caseComments";

describe("commentEntries", () => {
  it("keeps only comment entries, across pages", () => {
    const entries = commentEntries([
      {
        activities: [
          { id: "c1", type: "comment", createdOn: "x" },
          { id: "a1", type: "attachment", createdOn: "x" },
        ],
        hasMore: true,
      },
      {
        activities: [
          { id: "f1", type: "field_change", createdOn: "x" },
          { id: "c2", type: "comment", createdOn: "x" },
        ],
      },
      {},
      { activities: null },
    ]);
    expect(entries.map((e) => e.id)).toEqual(["c1", "c2"]);
  });
});

describe("toCommentDto", () => {
  it("maps a feed entry to the comment shape, preferring the full name", () => {
    expect(
      toCommentDto({
        id: "c1",
        type: "comment",
        content: "Hi\nthere",
        createdOn: "2026-11-10T08:00:00Z",
        createdBy: "jdoe@wso2.com",
        createdByFullName: " Jane Doe ",
      }),
    ).toEqual({
      id: "c1",
      content: "Hi<br>there",
      type: "comment",
      createdOn: "2026-11-10T08:00:00Z",
      createdBy: "Jane Doe",
      isEscalated: "false",
      hasInlineAttachments: false,
      inlineAttachments: [],
    });
  });

  it("falls back to createdBy, then to an empty name", () => {
    expect(toCommentDto({ id: "c", type: "comment", createdOn: "x", createdBy: "system" }).createdBy).toBe("system");
    expect(toCommentDto({ id: "c", type: "comment", createdOn: "x" }).createdBy).toBe("");
  });
});

describe("createdCommentDto", () => {
  it("fills the comment shape from backend-v2's short response and the text that was sent", () => {
    expect(createdCommentDto({ id: "c9", createdOn: "2026-11-10T09:00:00Z", createdBy: "Jane Doe" }, "Thanks")).toEqual(
      {
        id: "c9",
        content: "Thanks",
        type: "comment",
        createdOn: "2026-11-10T09:00:00Z",
        createdBy: "Jane Doe",
        isEscalated: "false",
        hasInlineAttachments: false,
        inlineAttachments: [],
      },
    );
  });
});
