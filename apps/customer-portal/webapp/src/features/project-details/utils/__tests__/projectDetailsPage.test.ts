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
import { PROJECT_DETAILS_TABS } from "@features/project-details/constants/projectDetailsConstants";
import { ProjectDetailsTabId } from "@features/project-details/types/projectDetails";
import {
  filterProjectDetailsTabsByPermissions,
  getProjectDetailsLoadingState,
} from "@features/project-details/utils/projectDetailsPage";

describe("getProjectDetailsLoadingState", () => {
  it("returns true while auth is loading", () => {
    expect(
      getProjectDetailsLoadingState({
        isAuthLoading: true,
        isProjectLoading: false,
        isStatsLoading: false,
        project: { id: "1" },
        projectError: null,
        stats: {},
        statsError: null,
      }),
    ).toBe(true);
  });

  it("returns false when project and stats are resolved", () => {
    expect(
      getProjectDetailsLoadingState({
        isAuthLoading: false,
        isProjectLoading: false,
        isStatsLoading: false,
        project: { id: "1" },
        projectError: null,
        stats: { projectStats: {} },
        statsError: null,
      }),
    ).toBe(false);
  });
});

describe("filterProjectDetailsTabsByPermissions", () => {
  it("hides deployments and time tracking when not permitted", () => {
    const tabs = filterProjectDetailsTabsByPermissions(PROJECT_DETAILS_TABS, {
      hasDeployments: false,
      hasTimeLogs: false,
    } as never);

    expect(tabs.map((t) => t.id)).toEqual([ProjectDetailsTabId.OVERVIEW]);
  });

  it("keeps all tabs when permitted", () => {
    const tabs = filterProjectDetailsTabsByPermissions(PROJECT_DETAILS_TABS, {
      hasDeployments: true,
      hasTimeLogs: true,
    } as never);

    expect(tabs.map((t) => t.id)).toEqual([
      ProjectDetailsTabId.OVERVIEW,
      ProjectDetailsTabId.DEPLOYMENTS,
      ProjectDetailsTabId.TIME_TRACKING,
    ]);
  });
});
