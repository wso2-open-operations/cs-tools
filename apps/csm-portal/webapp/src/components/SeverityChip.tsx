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

import type { JSX } from "react";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import {
  SEVERITY_COLOR,
  SEVERITY_LABEL,
} from "@features/csm-dashboard/utils/abtDashboard";
import SemanticChip from "@components/SemanticChip";

interface SeverityChipProps {
  severity: Severity;
  /** Append the descriptive label, e.g. "S1 — Critical" (used on the case header). */
  withLabel?: boolean;
  size?: "small" | "medium";
  /** Render a pointer cursor when the chip is wrapped in a link/clickable row. */
  clickable?: boolean;
}

/**
 * The single source of truth for rendering a case severity badge. Severity is
 * the highest-priority scan signal in the portal, so it renders as a bold,
 * solid {@link SemanticChip} (which guarantees WCAG AA) — visually out-ranking
 * the quieter outlined state chip.
 */
export default function SeverityChip({
  severity,
  withLabel = false,
  size = "small",
  clickable = false,
}: SeverityChipProps): JSX.Element {
  return (
    <SemanticChip
      role={SEVERITY_COLOR[severity]}
      label={
        withLabel ? `${severity} (${SEVERITY_LABEL[severity]})` : severity
      }
      size={size}
      bold
      clickable={clickable}
    />
  );
}
