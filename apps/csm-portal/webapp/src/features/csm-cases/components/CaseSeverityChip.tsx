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

import type { JSX, ReactNode } from "react";
import type { BeCaseType } from "@api/backend/types";
import type { SeverityOrUnset } from "@features/csm-dashboard/types/abtDashboard";
import SeverityChip from "@components/SeverityChip";
import { caseTypeHasSeverity } from "@features/csm-cases/utils/caseType";

interface CaseSeverityChipProps {
  caseType: BeCaseType | undefined;
  severity: SeverityOrUnset;
  withLabel?: boolean;
  size?: "small" | "medium";
  clickable?: boolean;
  /** Rendered when there is no badge to show. Defaults to a dash (a table cell); pass `null` to render nothing. */
  emptyFallback?: ReactNode;
}

/**
 * Severity badge for a case-family row, applying the per-type rules in one
 * place. Types with no severity concept render a dash. A service request's
 * severity is never unset (the backend always supplies S1-S4), so a missing
 * value there renders a dash too, never the "Unset" badge that is reserved
 * for support cases whose severity is genuinely absent.
 */
export default function CaseSeverityChip({
  caseType,
  severity,
  withLabel,
  size,
  clickable,
  emptyFallback = "—",
}: CaseSeverityChipProps): JSX.Element {
  if (!caseTypeHasSeverity(caseType) || (caseType === "service_request" && severity === "unset")) {
    return <>{emptyFallback}</>;
  }
  return (
    <SeverityChip severity={severity} withLabel={withLabel} size={size} clickable={clickable} />
  );
}
