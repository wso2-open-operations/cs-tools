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
import {
  calculateProjectStats,
  getProjectPermissions,
  getProjectSeverityPolicy,
  shouldForceSeverityS4,
} from "@utils/permission";

describe("getProjectPermissions", () => {
  it("enables Cloud Support SR and ops chart only when hasPdpSubscription is true", () => {
    const withoutPdp = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {});
    expect(withoutPdp.hasSR).toBe(false);
    expect(withoutPdp.showOutstandingOpsChart).toBe(false);

    const withPdp = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {
      hasPdpSubscription: true,
    });
    expect(withPdp.hasSR).toBe(true);
    expect(withPdp.showOutstandingOpsChart).toBe(true);
  });

  it("treats Cloud Subscription label like Cloud Support for PDP", () => {
    const perms = getProjectPermissions(ProjectType.CLOUD_SUBSCRIPTION, {
      hasPdpSubscription: true,
    });
    expect(perms.hasSR).toBe(true);
  });

  it("keeps Managed Cloud SR independent of hasPdpSubscription", () => {
    const perms = getProjectPermissions(ProjectType.MANAGED_CLOUD_SUBSCRIPTION, {
      hasPdpSubscription: false,
    });
    expect(perms.hasSR).toBe(true);
  });

  it("disables engagements for Professional Services", () => {
    const perms = getProjectPermissions(ProjectType.PROFESSIONAL_SERVICES);
    expect(perms.hasEngagements).toBe(false);
  });

  it("disables security report analysis for evaluation subscriptions", () => {
    const perms = getProjectPermissions(ProjectType.EVALUATION_SUBSCRIPTION);
    expect(perms.hasSecurityReportAnalysis).toBe(false);
  });

  it("enables security report analysis for professional services", () => {
    const perms = getProjectPermissions(ProjectType.PROFESSIONAL_SERVICES);
    expect(perms.hasSecurityReportAnalysis).toBe(true);
  });
});

describe("shouldForceSeverityS4", () => {
  it("is true for Development Support", () => {
    expect(shouldForceSeverityS4(ProjectType.DEVELOPMENT_SUPPORT)).toBe(true);
  });

  it("is true for Professional Services", () => {
    expect(shouldForceSeverityS4(ProjectType.PROFESSIONAL_SERVICES)).toBe(true);
  });
});

describe("getProjectSeverityPolicy", () => {
  it("excludes S0 and restricts to S4 for Development Support", () => {
    const policy = getProjectSeverityPolicy(ProjectType.DEVELOPMENT_SUPPORT);
    expect(policy.excludeS0).toBe(true);
    expect(policy.restrictSeverityToLow).toBe(true);
  });

  it("excludes S0 and restricts to S4 for Professional Services", () => {
    const policy = getProjectSeverityPolicy(ProjectType.PROFESSIONAL_SERVICES);
    expect(policy.excludeS0).toBe(true);
    expect(policy.restrictSeverityToLow).toBe(true);
  });

  it("keeps full severity range for Managed Cloud Subscription", () => {
    const policy = getProjectSeverityPolicy(
      ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    );
    expect(policy.excludeS0).toBe(false);
    expect(policy.restrictSeverityToLow).toBe(false);
  });
});

describe("calculateProjectStats", () => {
  it("zeros service requests when hasSR is false", () => {
    const perms = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {});
    const result = calculateProjectStats(perms, 5, 0);
    expect(result.serviceRequests).toBe(0);
    expect(result.total).toBe(0);
  });

  it("includes service requests when hasSR is true", () => {
    const perms = getProjectPermissions(ProjectType.CLOUD_SUPPORT, {
      hasPdpSubscription: true,
    });
    const result = calculateProjectStats(perms, 5, 0);
    expect(result.serviceRequests).toBe(5);
    expect(result.total).toBe(5);
  });
});
