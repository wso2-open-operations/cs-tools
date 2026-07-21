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

import type { BeCaseCause, BeCaseResolutionCode } from "@api/backend/types";

/** All backend resolution codes, in the order declared in openapi.yaml. */
export const RESOLUTION_CODES: BeCaseResolutionCode[] = [
  "SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED",
  "SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT",
  "SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET",
  "SOLVED_WORKAROUND_PROVIDED",
  "SOLVED_BY_CUSTOMER",
  "CONSIDERED_FOR_ROADMAP",
  "INCONCLUSIVE_OUT_OF_SCOPE",
  "INCONCLUSIVE_CANNOT_REPRODUCE",
  "INCONCLUSIVE_NO_WORKAROUND",
  "DUPLICATE_ISSUE",
  "VOIDED_CANCELED",
  "ON_HOLD",
  "CONSIDERED_FOR_ROADMAP_ALT",
  "SOLVED_FIXED_THE_ISSUE",
  "SOLVED_WORKAROUND_PROVIDED_ALT",
  "SOLVED_BY_CONTRIBUTOR",
  "SOLVED_BY_NOVERA",
  "ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS",
];

/** All backend root-cause categories, in the order declared in openapi.yaml. */
export const CASE_CAUSES: BeCaseCause[] = [
  "SOLUTION_ARCHITECTURE",
  "DEPLOYMENT_ARCHITECTURE",
  "USER_ERROR_CONFIGURATION",
  "USER_ERROR_PRODUCT_CONCEPT",
  "USER_ERROR_RUNTIME",
  "USER_ERROR_RECOMMENDATION_BEST_PRACTICES",
  "CUSTOMIZATION_LIMITATION",
  "CUSTOMIZATION_BUG",
  "DOCUMENTATION_GAP",
  "DOCUMENTATION_ERROR",
  "PRODUCT_LIMITATION",
  "PRODUCT_BUG",
  "PRODUCT_REGRESSION",
  "PRODUCT_MIGRATION",
  "INFRASTRUCTURE_DATABASE",
  "INFRASTRUCTURE_OS",
  "INFRASTRUCTURE_NETWORK",
  "INFRASTRUCTURE_JDK",
  "INFRASTRUCTURE_LDAP",
  "INFRASTRUCTURE_LOAD_BALANCER",
  "INFRASTRUCTURE_IAAS",
  "INFRASTRUCTURE_EXTERNAL_PRODUCT",
  "INFRASTRUCTURE_PROXY",
  "INFRASTRUCTURE_OTHER",
  "UNKNOWN",
];

/**
 * Exact display text for each resolution code, matching the backing data
 * source's picklist verbatim (punctuation, capitalization, and duplicate
 * wording included — e.g. `CONSIDERED_FOR_ROADMAP` and
 * `CONSIDERED_FOR_ROADMAP_ALT` both read "Considered for roadmap" because
 * that's what the source shows for both underlying choice-list entries).
 * Mechanical humanization can't reproduce this text, so it's listed
 * explicitly rather than derived from the enum token.
 */
export const RESOLUTION_CODE_LABELS: Record<BeCaseResolutionCode, string> = {
  SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED:
    "Solved – Fixed by support/guidance provided",
  SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT:
    "Solved – Fixed by closing related Incident",
  SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET:
    "Solved - Fixed by closing related R&D ticket",
  SOLVED_WORKAROUND_PROVIDED: "Solved – Workaround provided",
  SOLVED_BY_CUSTOMER: "Solved by customer",
  CONSIDERED_FOR_ROADMAP: "Considered for roadmap",
  INCONCLUSIVE_OUT_OF_SCOPE: "Inconclusive – Out of scope",
  INCONCLUSIVE_CANNOT_REPRODUCE: "Inconclusive – Cannot reproduce",
  INCONCLUSIVE_NO_WORKAROUND: "Inconclusive – No workaround",
  DUPLICATE_ISSUE: "Duplicate issue",
  VOIDED_CANCELED: "Voided/Canceled",
  ON_HOLD: "On Hold",
  CONSIDERED_FOR_ROADMAP_ALT: "Considered for roadmap",
  SOLVED_FIXED_THE_ISSUE: "Solved - Fixed the issue",
  SOLVED_WORKAROUND_PROVIDED_ALT: "Solved - Workaround provided",
  SOLVED_BY_CONTRIBUTOR: "Solved by Contributor",
  SOLVED_BY_NOVERA: "Solved : By Novera",
  ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS:
    "Abruptly closed due to non responsiveness through auto closure",
};

/**
 * Exact display text for each root-cause category, matching the backing
 * data source's "cause" picklist verbatim. Listed explicitly rather than
 * derived from the enum token — mechanical humanization can't reproduce the
 * source's `/`-separated wording or acronym casing (`IAAS`, `JDK`, `LDAP`,
 * `OS`).
 */
export const CASE_CAUSE_LABELS: Record<BeCaseCause, string> = {
  SOLUTION_ARCHITECTURE: "Solution Architecture",
  DEPLOYMENT_ARCHITECTURE: "Deployment Architecture",
  USER_ERROR_CONFIGURATION: "User Error/Configuration",
  USER_ERROR_PRODUCT_CONCEPT: "User Error/Product Concept",
  USER_ERROR_RUNTIME: "User Error/Runtime",
  USER_ERROR_RECOMMENDATION_BEST_PRACTICES:
    "User Error/Recommandation/Best practices",
  CUSTOMIZATION_LIMITATION: "Customization/Limitation",
  CUSTOMIZATION_BUG: "Customization/Bug",
  DOCUMENTATION_GAP: "Documentation/Gap",
  DOCUMENTATION_ERROR: "Documentation/Error",
  PRODUCT_LIMITATION: "Product/Limitation",
  PRODUCT_BUG: "Product/Bug",
  PRODUCT_REGRESSION: "Product/Regression",
  PRODUCT_MIGRATION: "Product/Migration",
  INFRASTRUCTURE_DATABASE: "Infrastructure/Database",
  INFRASTRUCTURE_OS: "Infrastructure/OS",
  INFRASTRUCTURE_NETWORK: "Infrastructure/Network",
  INFRASTRUCTURE_JDK: "Infrastructure/JDK",
  INFRASTRUCTURE_LDAP: "Infrastructure/LDAP",
  INFRASTRUCTURE_LOAD_BALANCER: "Infrastructure/Load Balancer",
  INFRASTRUCTURE_IAAS: "Infrastructure/IAAS",
  INFRASTRUCTURE_EXTERNAL_PRODUCT: "Infrastructure/External Product",
  INFRASTRUCTURE_PROXY: "Infrastructure/Proxy",
  INFRASTRUCTURE_OTHER: "Infrastructure/Other",
  UNKNOWN: "Unknown",
};
