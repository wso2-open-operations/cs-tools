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

import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
import type {
  CsmProjectRow,
  CsmProjectsListResponse,
  CsmProjectStatus,
  CsmProjectTier,
} from "@features/csm-projects/types/csmProjects";

interface ProjectMetaSeed {
  tier: CsmProjectTier;
  productType: string;
  status: CsmProjectStatus;
  updateLevel: string;
  /** WSO2-side sales / account-management contact (the "Account Manager"). */
  accountManager: string;
  /** WSO2-side sales-engineering contact (the "Technical Owner"). */
  technicalOwner: string;
}

const PROJECT_META: Record<string, ProjectMetaSeed> = {
  "prj-acme-iam-prod": {
    tier: "Platinum",
    productType: "Identity Server",
    status: "Active",
    updateLevel: "IS-7.0.0 / U231",
    accountManager: "Erin Walters",
    technicalOwner: "Sanjay K.",
    // NOTE: Sanjay K. is a project-level override (see contactsMocks). Other
    // Acme projects fall back to the account-level TO (Naveen P.).
  },
  "prj-acme-openbanking": {
    tier: "Platinum",
    productType: "Open Banking",
    status: "Active",
    updateLevel: "OB-4.0.0 / U118",
    accountManager: "Erin Walters",
    technicalOwner: "Naveen P.",
  },
  "prj-initech-apim": {
    tier: "Platinum",
    productType: "API Manager",
    status: "Active",
    updateLevel: "APIM-4.3.0 / U92",
    accountManager: "Sashika H.",
    technicalOwner: "Chathura D.",
  },
  "prj-initech-mi": {
    tier: "Platinum",
    productType: "Micro Integrator",
    status: "Active",
    updateLevel: "MI-4.3.0 / U67",
    accountManager: "Sashika H.",
    technicalOwner: "Chathura D.",
  },
  "prj-initech-si": {
    tier: "Platinum",
    productType: "Streaming Integrator",
    status: "Active",
    updateLevel: "SI-4.2.0 / U43",
    accountManager: "Sashika H.",
    technicalOwner: "Chathura D.",
  },
  "prj-initech-choreo": {
    tier: "Platinum",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
    accountManager: "Sashika H.",
    technicalOwner: "Chathura D.",
  },
  "prj-umbrella-choreo": {
    tier: "Gold",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
    accountManager: "Anya Kovac",
    technicalOwner: "Dilshan A.",
  },
  "prj-umbrella-asgardeo": {
    tier: "Gold",
    productType: "Asgardeo",
    status: "Active",
    updateLevel: "Asgardeo Cloud",
    accountManager: "Anya Kovac",
    technicalOwner: "Dilshan A.",
  },
  "prj-globex-choreo": {
    tier: "Gold",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
    accountManager: "Bilal R.",
    technicalOwner: "Ramesh M.",
  },
  "prj-globex-iam": {
    tier: "Gold",
    productType: "Identity Server",
    status: "Active",
    updateLevel: "IS-6.1.0 / U195",
    accountManager: "Bilal R.",
    technicalOwner: "Ramesh M.",
  },
  "prj-soylent-apim": {
    tier: "Silver",
    productType: "API Manager",
    status: "Active",
    updateLevel: "APIM-4.2.0 / U88",
    accountManager: "Tom Marvolo",
    technicalOwner: "Ishara K.",
  },
  "prj-soylent-iam": {
    tier: "Silver",
    productType: "Identity Server",
    status: "Onboarding",
    updateLevel: "IS-7.0.0 / U231",
    accountManager: "Tom Marvolo",
    technicalOwner: "Ishara K.",
  },
  // All-customer extras (only visible at scope=all_customers via the cases mocks)
  "prj-wayne-openbanking": {
    tier: "Platinum",
    productType: "Open Banking",
    status: "Active",
    updateLevel: "OB-4.0.0 / U118",
    accountManager: "Erin Walters",
    technicalOwner: "Naveen P.",
  },
  "prj-stark-choreo": {
    tier: "Gold",
    productType: "Choreo",
    status: "Active",
    updateLevel: "Choreo Cloud",
    accountManager: "Sashika H.",
    technicalOwner: "Asanka R.",
  },
  "prj-stark-iam": {
    tier: "Gold",
    productType: "Identity Server",
    status: "Active",
    updateLevel: "IS-7.0.0 / U231",
    accountManager: "Sashika H.",
    technicalOwner: "Asanka R.",
  },
  "prj-tyrell-apim": {
    tier: "Silver",
    productType: "API Manager",
    status: "Suspended",
    updateLevel: "APIM-4.1.0 / U54",
    accountManager: "Tom Marvolo",
    technicalOwner: "Ishara K.",
  },
};

const FALLBACK_META: ProjectMetaSeed = {
  tier: "Silver",
  productType: "—",
  status: "Active",
  updateLevel: "—",
  accountManager: "—",
  technicalOwner: "—",
};

interface Aggregates {
  openCaseCount: number;
  s0s1Count: number;
  breachedCount: number;
  lastActivityAt: string;
}

function projectAggregatesFromCases(
  scope: DashboardScope,
): Map<string, Aggregates & { name: string; customer: string; accountId: string }> {
  const cases = getMockCsmCases(scope).cases;
  const byProject = new Map<
    string,
    Aggregates & { name: string; customer: string; accountId: string }
  >();

  for (const c of cases) {
    let agg = byProject.get(c.projectId);
    if (!agg) {
      agg = {
        name: c.projectName,
        customer: c.customer,
        accountId: c.accountId,
        openCaseCount: 0,
        s0s1Count: 0,
        breachedCount: 0,
        lastActivityAt: c.updatedAt,
      };
      byProject.set(c.projectId, agg);
    }
    if (c.state !== "closed") agg.openCaseCount += 1;
    if ((c.severity === "S0" || c.severity === "S1") && c.state !== "closed") {
      agg.s0s1Count += 1;
    }
    if (c.minutesToBreach < 0 && c.state !== "closed") {
      agg.breachedCount += 1;
    }
    if (c.updatedAt > agg.lastActivityAt) {
      agg.lastActivityAt = c.updatedAt;
    }
  }

  return byProject;
}

/**
 * Look up a single project by id. Uses the broadest scope so direct URL
 * navigation works even when the project sits outside the engineer's ABT.
 */
export function getMockCsmProjectById(
  projectId: string,
): CsmProjectRow | undefined {
  return getMockCsmProjects("all_customers").projects.find(
    (p) => p.id === projectId,
  );
}

export function getMockCsmProjects(
  scope: DashboardScope,
): CsmProjectsListResponse {
  const aggregates = projectAggregatesFromCases(scope);
  const projects: CsmProjectRow[] = [];

  for (const [projectId, agg] of aggregates) {
    const meta = PROJECT_META[projectId] ?? FALLBACK_META;
    projects.push({
      id: projectId,
      name: agg.name,
      customer: agg.customer,
      accountId: agg.accountId,
      tier: meta.tier,
      productType: meta.productType,
      status: meta.status,
      updateLevel: meta.updateLevel,
      accountManager: meta.accountManager,
      technicalOwner: meta.technicalOwner,
      openCaseCount: agg.openCaseCount,
      s0s1Count: agg.s0s1Count,
      breachedCount: agg.breachedCount,
      lastActivityAt: agg.lastActivityAt,
    });
  }

  // Customer A → Z, then project name A → Z for stable display order.
  projects.sort((a, b) => {
    const byCustomer = a.customer.localeCompare(b.customer);
    return byCustomer !== 0 ? byCustomer : a.name.localeCompare(b.name);
  });

  return { scope, projects };
}
