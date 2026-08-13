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
  conversationStateChipMeta,
  conversationStateGroup,
  countActiveConversationFilters,
  DEFAULT_CONVERSATION_FILTERS,
} from "@features/csm-projects/utils/conversationState";

describe("conversationStateGroup", () => {
  it("keeps ACTIVE on its own", () => {
    expect(conversationStateGroup("ACTIVE")).toBe("active");
  });

  it("keeps CONVERTED as its own positive group, distinct from CLOSED", () => {
    expect(conversationStateGroup("CONVERTED")).toBe("converted");
  });

  it("collapses RESOLVED/ABANDONED/CLOSED into the closed group", () => {
    expect(conversationStateGroup("RESOLVED")).toBe("closed");
    expect(conversationStateGroup("ABANDONED")).toBe("closed");
    expect(conversationStateGroup("CLOSED")).toBe("closed");
  });
});

describe("conversationStateChipMeta", () => {
  it("gives CONVERTED a success role, distinct from the neutral closed role", () => {
    expect(conversationStateChipMeta("CONVERTED").role).toBe("success");
    expect(conversationStateChipMeta("CLOSED").role).toBe("default");
    expect(conversationStateChipMeta("RESOLVED").role).toBe("default");
    expect(conversationStateChipMeta("ABANDONED").role).toBe("default");
  });

  it("gives ACTIVE an info role", () => {
    expect(conversationStateChipMeta("ACTIVE").role).toBe("info");
  });
});

describe("countActiveConversationFilters", () => {
  it("is 0 for the default filters", () => {
    expect(countActiveConversationFilters(DEFAULT_CONVERSATION_FILTERS)).toBe(0);
  });

  it("counts states and createdByMe, but not search", () => {
    expect(
      countActiveConversationFilters({
        ...DEFAULT_CONVERSATION_FILTERS,
        search: "billing",
        states: ["ACTIVE"],
        createdByMe: true,
      }),
    ).toBe(2);
  });

  it("counts a non-blank number and a non-empty createdBy list", () => {
    expect(
      countActiveConversationFilters({
        ...DEFAULT_CONVERSATION_FILTERS,
        number: "CHAT0000012345",
        createdBy: ["jane.doe@example.com"],
      }),
    ).toBe(2);
  });

  it("does not count a blank/whitespace-only number", () => {
    expect(
      countActiveConversationFilters({
        ...DEFAULT_CONVERSATION_FILTERS,
        number: "   ",
      }),
    ).toBe(0);
  });
});
