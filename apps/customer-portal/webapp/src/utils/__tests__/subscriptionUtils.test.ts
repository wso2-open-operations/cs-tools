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
import { PROJECT_TYPE_LABELS } from "@constants/projectTypeConstants";
import {
  calculateProjectStats,
  getProjectPermissions,
  getRecentActivityItems,
  shouldExcludeS0,
} from "@utils/subscriptionUtils";

describe("getProjectPermissions", () => {
  it("should enable full features for Managed Cloud Subscription", () => {
    const p = getProjectPermissions(
      PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION,
    );
    expect(p.hasOperations).toBe(true);
    expect(p.hasSR).toBe(true);
    expect(p.hasCR).toBe(true);
    expect(p.hasDeployments).toBe(true);
    expect(p.hasTimeLogs).toBe(true);
    expect(p.hasQueryHours).toBe(true);
    expect(p.showOutstandingOpsChart).toBe(true);
    expect(p.includeChangeRequestsInDashboardTotals).toBe(true);
    expect(p.includeS0InSupportMetrics).toBe(true);
    expect(p.showServiceHoursAllocationsCard).toBe(true);
  });

  it("should enable SR-only operations for Cloud Support", () => {
    const p = getProjectPermissions(PROJECT_TYPE_LABELS.CLOUD_SUPPORT);
    expect(p.hasOperations).toBe(true);
    expect(p.hasSR).toBe(true);
    expect(p.hasCR).toBe(false);
    expect(p.hasDeployments).toBe(false);
    expect(p.hasTimeLogs).toBe(false);
    expect(p.showOutstandingOpsChart).toBe(true);
    expect(p.includeChangeRequestsInDashboardTotals).toBe(false);
    expect(p.includeS0InSupportMetrics).toBe(false);
  });

  it("should be restrictive for Cloud Evaluation Support", () => {
    const p = getProjectPermissions(
      PROJECT_TYPE_LABELS.CLOUD_EVALUATION_SUPPORT,
    );
    expect(p.hasOperations).toBe(false);
    expect(p.hasSR).toBe(false);
    expect(p.showOutstandingOpsChart).toBe(false);
    expect(p.hasDeployments).toBe(false);
    expect(p.hasQueryHours).toBe(false);
  });

  it("should be restrictive for Evaluation Subscription and Subscription", () => {
    for (const label of [
      PROJECT_TYPE_LABELS.EVALUATION_SUBSCRIPTION,
      PROJECT_TYPE_LABELS.SUBSCRIPTION,
    ]) {
      const p = getProjectPermissions(label);
      expect(p.hasOperations).toBe(false);
      expect(p.showOutstandingOpsChart).toBe(false);
    }
  });

  it("should be restrictive for unknown labels", () => {
    const p = getProjectPermissions("Unknown Product");
    expect(p.hasOperations).toBe(false);
    expect(p.includeS0InSupportMetrics).toBe(false);
  });

  it("should be restrictive for null/undefined", () => {
    expect(getProjectPermissions(null).hasOperations).toBe(false);
    expect(getProjectPermissions(undefined).hasOperations).toBe(false);
  });
});

describe("shouldExcludeS0", () => {
  it("should return false only for Managed Cloud Subscription", () => {
    expect(
      shouldExcludeS0(PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION),
    ).toBe(false);
    expect(shouldExcludeS0(PROJECT_TYPE_LABELS.CLOUD_SUPPORT)).toBe(true);
  });
});

describe("calculateProjectStats", () => {
  it("should sum SR and CR when hasCR", () => {
    const permissions = getProjectPermissions(
      PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION,
    );
    const r = calculateProjectStats(permissions, 3, 5);
    expect(r.serviceRequests).toBe(3);
    expect(r.changeRequests).toBe(5);
    expect(r.total).toBe(8);
  });

  it("should ignore CR count when !hasCR", () => {
    const permissions = getProjectPermissions(
      PROJECT_TYPE_LABELS.CLOUD_SUPPORT,
    );
    const r = calculateProjectStats(permissions, 3, 5);
    expect(r.serviceRequests).toBe(3);
    expect(r.changeRequests).toBe(0);
    expect(r.total).toBe(3);
  });
});

describe("getRecentActivityItems", () => {
  it("should omit time rows when hasTimeLogs is false", () => {
    const items = getRecentActivityItems(
      { totalHours: 60, billableHours: 30, lastDeploymentOn: "2025-01-01" },
      PROJECT_TYPE_LABELS.CLOUD_SUPPORT,
    );
    expect(items.some((i) => i.label === "Total Time Logged")).toBe(false);
    expect(items.some((i) => i.label === "Last Deployment")).toBe(false);
  });

  it("should include time and deployment rows for Managed Cloud", () => {
    const items = getRecentActivityItems(
      { totalHours: 60, billableHours: 30, lastDeploymentOn: "2025-01-01" },
      PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION,
    );
    expect(items.some((i) => i.label === "Total Time Logged")).toBe(true);
    expect(items.some((i) => i.label === "Last Deployment")).toBe(true);
  });
});
