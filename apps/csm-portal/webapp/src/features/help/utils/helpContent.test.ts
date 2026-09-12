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
import { navNodeById } from "@config/csmNavItems";
import { HELP_TOPIC_CONTENT } from "@features/help/utils/helpContent";

describe("HELP_TOPIC_CONTENT", () => {
  it("has a non-empty Markdown source for every topic declared in the nav tree", () => {
    const help = navNodeById("help");
    const topicIds = (help?.children ?? []).map((child) =>
      child.id.replace(/^help\./, ""),
    );
    expect(topicIds.length).toBeGreaterThan(0);

    for (const id of topicIds) {
      expect(HELP_TOPIC_CONTENT[id]).toBeTruthy();
    }
  });

  it("declares no orphaned entries beyond the nav tree's topic list", () => {
    const help = navNodeById("help");
    const topicIds = new Set(
      (help?.children ?? []).map((child) => child.id.replace(/^help\./, "")),
    );
    for (const key of Object.keys(HELP_TOPIC_CONTENT)) {
      expect(topicIds.has(key)).toBe(true);
    }
  });
});
