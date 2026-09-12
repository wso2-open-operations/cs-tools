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
  canRequestCaseUpdate,
  deriveCaseUpdateRequestCategory,
} from "@features/csm-cases/utils/caseUpdateRequests";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

describe("canRequestCaseUpdate", () => {
  it.each<CaseState>(["awaiting_info", "solution_proposed"])(
    "is true while the case is %s",
    (state) => {
      expect(canRequestCaseUpdate({ state } as CsmCaseDetail)).toBe(true);
    },
  );

  it.each<CaseState>(["open", "work_in_progress", "waiting_on_wso2", "closed"])(
    "is false while the case is %s",
    (state) => {
      expect(canRequestCaseUpdate({ state } as CsmCaseDetail)).toBe(false);
    },
  );
});

describe("deriveCaseUpdateRequestCategory", () => {
  it("is migration only for an engagement case with engagementType migration", () => {
    expect(
      deriveCaseUpdateRequestCategory({
        caseType: "engagement",
        engagementType: "Migration",
      }),
    ).toBe("migration");
  });

  it("is case-insensitive on engagementType, mirroring the backend's strings.EqualFold check", () => {
    expect(
      deriveCaseUpdateRequestCategory({
        caseType: "engagement",
        engagementType: "MIGRATION",
      }),
    ).toBe("migration");
    expect(
      deriveCaseUpdateRequestCategory({
        caseType: "engagement",
        engagementType: "migration",
      }),
    ).toBe("migration");
  });

  it("is generic for a non-migration engagement type", () => {
    expect(
      deriveCaseUpdateRequestCategory({
        caseType: "engagement",
        engagementType: "onboarding",
      }),
    ).toBe("generic");
  });

  it("is generic for a non-engagement case even when engagementType is somehow set", () => {
    expect(
      deriveCaseUpdateRequestCategory({
        caseType: "case",
        engagementType: "Migration",
      }),
    ).toBe("generic");
  });

  it("is generic when caseType/engagementType are both absent", () => {
    expect(deriveCaseUpdateRequestCategory({})).toBe("generic");
  });
});
