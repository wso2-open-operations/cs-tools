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
import { ProjectHubContentView } from "@features/project-hub/types/projectHub";
import {
  resolveProjectHubContentView,
  resolveProjectHubHeaderSubtitle,
  resolveProjectHubHeaderTitle,
  shouldHideProjectHubHeaderBlock,
  shouldShowProjectHubSearchBar,
} from "@features/project-hub/utils/projectHub";

describe("resolveProjectHubContentView", () => {
  it("returns redirect loader when redirecting to single project", () => {
    expect(
      resolveProjectHubContentView(true, false, false, false, 1, false),
    ).toBe(ProjectHubContentView.REDIRECT_LOADER);
  });

  it("returns project list when data is ready", () => {
    expect(
      resolveProjectHubContentView(false, false, false, false, 3, false),
    ).toBe(ProjectHubContentView.PROJECT_LIST);
  });

  it("returns error view when query failed", () => {
    expect(
      resolveProjectHubContentView(false, false, false, true, 0, false),
    ).toBe(ProjectHubContentView.ERROR);
  });
});

describe("resolveProjectHubHeaderTitle", () => {
  it("formats title with total count", () => {
    expect(resolveProjectHubHeaderTitle(12)).toBe("Your Projects (12)");
  });
});

describe("resolveProjectHubHeaderSubtitle", () => {
  it("returns select prompt", () => {
    expect(resolveProjectHubHeaderSubtitle()).toContain("Choose a project");
  });
});

describe("shouldShowProjectHubSearchBar", () => {
  it("shows when more than minimum projects", () => {
    expect(shouldShowProjectHubSearchBar(6, "")).toBe(true);
  });

  it("shows when search query is non-empty", () => {
    expect(shouldShowProjectHubSearchBar(1, "  acme ")).toBe(true);
  });
});

describe("shouldHideProjectHubHeaderBlock", () => {
  it("hides on error", () => {
    expect(shouldHideProjectHubHeaderBlock(true, false, false, 5, "")).toBe(true);
  });

  it("hides when empty and no search", () => {
    expect(shouldHideProjectHubHeaderBlock(false, false, false, 0, "")).toBe(true);
  });
});
