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

import type { Severity } from "@features/csm-dashboard/types/abtDashboard";

export type CsmSearchHitKind = "case" | "project" | "account";

interface CsmSearchHitBase {
  kind: CsmSearchHitKind;
  /** Stable id used for the route + react key. */
  id: string;
  /** Headline rendered as the primary label. */
  title: string;
  /** Optional secondary line (e.g. customer · project). */
  subtitle?: string;
  /** Optional badge text (e.g. severity, tier). */
  badge?: string;
  /** Route to navigate to on click. */
  href: string;
}

export interface CsmSearchCaseHit extends CsmSearchHitBase {
  kind: "case";
  caseNumber: string;
  severity: Severity;
}

export interface CsmSearchProjectHit extends CsmSearchHitBase {
  kind: "project";
}

export interface CsmSearchAccountHit extends CsmSearchHitBase {
  kind: "account";
}

export type CsmSearchHit =
  | CsmSearchCaseHit
  | CsmSearchProjectHit
  | CsmSearchAccountHit;

export interface CsmSearchResults {
  query: string;
  cases: CsmSearchCaseHit[];
  projects: CsmSearchProjectHit[];
  accounts: CsmSearchAccountHit[];
}
