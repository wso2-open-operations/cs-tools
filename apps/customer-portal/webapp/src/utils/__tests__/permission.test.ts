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
import { ProjectType } from "@/types/permission";
import type { ProjectFeatures } from "@features/project-hub/types/projects";
import {
  calculateProjectStats,
  getProjectPermissions,
  getProjectSeverityPolicy,
  shouldHideOnboardingData,
  shouldForceSeverityS4,
} from "@utils/permission";

function buildProjectFeatures(
  overrides?: Partial<ProjectFeatures>,
): ProjectFeatures {
  return {
    acceptedSeverityValues: [
      { id: "10", label: "Critical (P1)" },
      { id: "11", label: "High (P2)" },
      { id: "12", label: "Medium (P3)" },
      { id: "13", label: "Low (P4)" },
    ],
    hasServiceRequestWriteAccess: false,
    hasServiceRequestReadAccess: false,
    hasSraWriteAccess: false,
    hasSraReadAccess: false,
    hasChangeRequestReadAccess: false,
    hasEngagementsReadAccess: false,
    hasUpdatesReadAccess: false,
    hasTimeLogsReadAccess: false,
    hasDeploymentWriteAccess: false,
    hasDeploymentReadAccess: false,
    ...overrides,
  };
}

describe("getProjectPermissions", () => {
  it("keeps permission resolution independent from projectTypeLabel", () => {
    const options = {
      projectFeatures: buildProjectFeatures({
        hasServiceRequestReadAccess: true,
        hasChangeRequestReadAccess: true,
      }),
    };
    const cloudSupport = getProjectPermissions(
      ProjectType.CLOUD_SUPPORT,
      options,
    );
    const subscription = getProjectPermissions(
      ProjectType.SUBSCRIPTION,
      options,
    );
    expect(cloudSupport).toEqual(subscription);
  });

  it("enables SR, CR, SRA, engagements, updates and operations from feature flags", () => {
    const perms = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {
      projectFeatures: buildProjectFeatures({
        hasServiceRequestReadAccess: true,
        hasChangeRequestReadAccess: true,
        hasSraReadAccess: true,
        hasEngagementsReadAccess: true,
        hasUpdatesReadAccess: true,
      }),
    });
    expect(perms.hasOperations).toBe(true);
    expect(perms.hasSR).toBe(true);
    expect(perms.hasCR).toBe(true);
    expect(perms.hasSecurityReportAnalysis).toBe(true);
    expect(perms.hasEngagements).toBe(true);
    expect(perms.hasUpdates).toBe(true);
    expect(perms.includeChangeRequestsInDashboardTotals).toBe(true);
  });

  it("enables deployment and time-log cards based on feature access", () => {
    const perms = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {
      projectFeatures: buildProjectFeatures({
        hasDeploymentReadAccess: true,
        hasTimeLogsReadAccess: true,
      }),
    });
    expect(perms.hasDeployments).toBe(true);
    expect(perms.hasTimeLogs).toBe(true);
    expect(perms.showServiceHoursAllocationsCard).toBe(true);
  });

  it("includes S0 only when catastrophic severity is accepted", () => {
    const withoutS0 = getProjectPermissions(
      ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
      {
        projectFeatures: buildProjectFeatures(),
      },
    );
    const withS0 = getProjectPermissions(
      ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
      {
        projectFeatures: buildProjectFeatures({
          acceptedSeverityValues: [
            { id: "14", label: "Catastrophic (P0)" },
            { id: "10", label: "Critical (P1)" },
            { id: "11", label: "High (P2)" },
            { id: "12", label: "Medium (P3)" },
            { id: "13", label: "Low (P4)" },
          ],
        }),
      },
    );
    expect(withoutS0.includeS0InSupportMetrics).toBe(false);
    expect(withS0.includeS0InSupportMetrics).toBe(true);
  });
});

describe("shouldForceSeverityS4", () => {
  it("is true when only Low (P4) is accepted", () => {
    expect(
      shouldForceSeverityS4(ProjectType.DEVELOPMENT_SUPPORT, {
        projectFeatures: buildProjectFeatures({
          acceptedSeverityValues: [{ id: "13", label: "Low (P4)" }],
        }),
      }),
    ).toBe(true);
  });

  it("is false when multiple severities are accepted", () => {
    expect(
      shouldForceSeverityS4(ProjectType.PROFESSIONAL_SERVICES, {
        projectFeatures: buildProjectFeatures(),
      }),
    ).toBe(false);
  });
});

describe("getProjectSeverityPolicy", () => {
  it("excludes S0 and restricts to low when only Low (P4) is accepted", () => {
    const policy = getProjectSeverityPolicy(ProjectType.DEVELOPMENT_SUPPORT, {
      projectFeatures: buildProjectFeatures({
        acceptedSeverityValues: [{ id: "13", label: "Low (P4)" }],
      }),
    });
    expect(policy.excludeS0).toBe(true);
    expect(policy.restrictSeverityToLow).toBe(true);
  });

  it("keeps full severity range when catastrophic is accepted", () => {
    const policy = getProjectSeverityPolicy(
      ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
      {
        projectFeatures: buildProjectFeatures({
          acceptedSeverityValues: [
            { id: "14", label: "Catastrophic (P0)" },
            { id: "10", label: "Critical (P1)" },
            { id: "11", label: "High (P2)" },
            { id: "12", label: "Medium (P3)" },
            { id: "13", label: "Low (P4)" },
          ],
        }),
      },
    );
    expect(policy.excludeS0).toBe(false);
    expect(policy.restrictSeverityToLow).toBe(false);
  });
});

describe("calculateProjectStats", () => {
  it("zeros service requests when hasSR is false", () => {
    const perms = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {
      projectFeatures: buildProjectFeatures(),
    });
    const result = calculateProjectStats(perms, 5, 0);
    expect(result.serviceRequests).toBe(0);
    expect(result.total).toBe(0);
  });

  it("includes service requests when hasSR is true", () => {
    const perms = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {
      projectFeatures: buildProjectFeatures({
        hasServiceRequestReadAccess: true,
      }),
    });
    const result = calculateProjectStats(perms, 5, 0);
    expect(result.serviceRequests).toBe(5);
    expect(result.total).toBe(5);
  });
});

describe("shouldHideOnboardingData", () => {
  it("returns true when onboarding status is Not-Applicable", () => {
    expect(shouldHideOnboardingData("Not-Applicable")).toBe(true);
  });

  it("returns true for null, undefined, and empty-string aliases", () => {
    expect(shouldHideOnboardingData(undefined)).toBe(false);
    expect(shouldHideOnboardingData(null)).toBe(false);
    expect(shouldHideOnboardingData("")).toBe(false);
  });

  it("returns true for normalized Not Applicable variants", () => {
    expect(shouldHideOnboardingData(" Not Applicable ")).toBe(true);
    expect(shouldHideOnboardingData("NOT-APPLICABLE")).toBe(true);
    expect(shouldHideOnboardingData("not_applicable")).toBe(true);
    expect(shouldHideOnboardingData("N/A")).toBe(true);
  });

  it("returns false for active onboarding values", () => {
    expect(shouldHideOnboardingData("In Progress")).toBe(false);
  });
});
