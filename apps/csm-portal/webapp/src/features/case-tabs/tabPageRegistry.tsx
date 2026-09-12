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

import type { ComponentType } from "react";
import type { CaseRouteKind } from "@context/case-tabs/caseTabsTypes";
import CsmCaseDetailPage from "@features/csm-cases/pages/CsmCaseDetailPage";
import CsmIncidentDetailPage from "@features/csm-operations/pages/CsmIncidentDetailPage";
import CsmChangeRequestDetailPage from "@features/csm-operations/pages/CsmChangeRequestDetailPage";

/**
 * Which page component renders for which open-tab kind. The five case-like
 * kinds all share `CsmCaseDetailPage`; `incident` and `change_request` each
 * have their own dedicated page. All three are plain eager imports — same
 * module instance `App.tsx` also imports directly, so this registry adds no
 * extra copy of any of them.
 */
export function pageComponentForKind(kind: CaseRouteKind): ComponentType {
  switch (kind) {
    case "incident":
      return CsmIncidentDetailPage;
    case "change_request":
      return CsmChangeRequestDetailPage;
    case "case":
    case "service_request":
    case "engagement":
    case "announcement":
    case "security_report_analysis":
      return CsmCaseDetailPage;
  }
}
