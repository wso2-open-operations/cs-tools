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

import type { CsmContact } from "@features/csm-projects/types/csmProjects";

/**
 * Account-level contacts. Two roles are WSO2-internal assignments:
 *   - "Account Manager" — WSO2 sales / account owner
 *   - "Technical Owner" — WSO2 sales engineering lead
 * The rest ("Primary Contact", "Billing Contact", "Security Contact") are
 * customer-side. Project-level overrides in {@link PROJECT_CONTACTS} extend
 * (and can supersede) these for a specific project view.
 */
const ACCOUNT_CONTACTS: Record<string, CsmContact[]> = {
  "acc-001": [
    {
      id: "c-acc-001-am",
      name: "Erin Walters",
      email: "erin.walters@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      phone: "+14155550101",
      scope: "account",
    },
    {
      id: "c-acc-001-to",
      name: "Naveen P.",
      email: "naveen.p@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-001-primary",
      name: "Renee Park",
      email: "renee.park@acmefinancial.com",
      roles: ["Primary Contact"],
      status: "Active",
      phone: "+14155550102",
      scope: "account",
    },
    {
      id: "c-acc-001-billing",
      name: "Jordan Vega",
      email: "billing@acmefinancial.com",
      roles: ["Billing Contact"],
      status: "Active",
      scope: "account",
    },
  ],
  "acc-002": [
    {
      id: "c-acc-002-am",
      name: "Bilal R.",
      email: "bilal.r@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-002-to",
      name: "Ramesh M.",
      email: "ramesh.m@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-002-primary",
      name: "Helena Voss",
      email: "h.voss@globex.com",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-002-sec",
      name: "Marcus Liang",
      email: "secops@globex.com",
      roles: ["Security Contact"],
      status: "Invited",
      scope: "account",
    },
  ],
  "acc-003": [
    {
      id: "c-acc-003-am",
      name: "Sashika H.",
      email: "sashika.h@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-003-to",
      name: "Chathura D.",
      email: "chathura.d@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-003-primary",
      name: "Peter Gibbons",
      email: "peter@initech.io",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "account",
    },
  ],
  "acc-004": [
    {
      id: "c-acc-004-am",
      name: "Tom Marvolo",
      email: "tom.m@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-004-to",
      name: "Ishara K.",
      email: "ishara.k@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-004-primary",
      name: "Marie Sandwitch",
      email: "marie@soylent.com",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "account",
    },
  ],
  "acc-005": [
    {
      id: "c-acc-005-am",
      name: "Anya Kovac",
      email: "anya.k@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-005-to",
      name: "Dilshan A.",
      email: "dilshan.a@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-005-primary",
      name: "Albert Wesker",
      email: "a.wesker@umbrella.health",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-005-billing",
      name: "Imelda Brooks",
      email: "billing@umbrella.health",
      roles: ["Billing Contact"],
      status: "Inactive",
      scope: "account",
    },
  ],
  "acc-101": [
    {
      id: "c-acc-101-am",
      name: "Erin Walters",
      email: "erin.walters@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-101-to",
      name: "Naveen P.",
      email: "naveen.p@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-101-primary",
      name: "Lucius Fox",
      email: "lfox@wayne.com",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "account",
    },
  ],
  "acc-102": [
    {
      id: "c-acc-102-am",
      name: "Sashika H.",
      email: "sashika.h@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-102-to",
      name: "Asanka R.",
      email: "asanka.r@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-102-primary",
      name: "Pepper Potts",
      email: "pepper@stark.com",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "account",
    },
  ],
  "acc-103": [
    {
      id: "c-acc-103-am",
      name: "Tom Marvolo",
      email: "tom.m@wso2.com",
      roles: ["Account Manager"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-103-to",
      name: "Ishara K.",
      email: "ishara.k@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "account",
    },
    {
      id: "c-acc-103-primary",
      name: "Eldon Tyrell",
      email: "eldon@tyrell.corp",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "account",
    },
  ],
};

/**
 * Project-level overrides / additions. Most projects just inherit from the
 * account; the project-specific entries here typically swap in a different
 * WSO2 sales engineer for a single project, or add an extra customer
 * tech lead.
 */
const PROJECT_CONTACTS: Record<string, CsmContact[]> = {
  "prj-acme-iam-prod": [
    {
      id: "c-prj-acme-iam-to",
      name: "Sanjay K.",
      email: "sanjay.k@wso2.com",
      roles: ["Technical Owner"],
      status: "Active",
      scope: "project",
      projectId: "prj-acme-iam-prod",
    },
  ],
  "prj-initech-mi": [
    {
      id: "c-prj-initech-mi-primary",
      name: "Bill Lumbergh",
      email: "bill@initech.io",
      roles: ["Primary Contact"],
      status: "Active",
      scope: "project",
      projectId: "prj-initech-mi",
    },
  ],
};

export function getMockAccountContacts(accountId: string): CsmContact[] {
  return ACCOUNT_CONTACTS[accountId] ?? [];
}

export function getMockProjectContacts(
  projectId: string,
  accountId: string | undefined,
): CsmContact[] {
  const projectExtras = PROJECT_CONTACTS[projectId] ?? [];
  const accountInherited = accountId ? getMockAccountContacts(accountId) : [];
  // Project-specific entries first; same email won't appear twice.
  const seen = new Set(projectExtras.map((c) => c.email));
  const merged = [...projectExtras];
  for (const c of accountInherited) {
    if (!seen.has(c.email)) merged.push(c);
  }
  return merged;
}
