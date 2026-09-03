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
import {
  escalationLevelColor,
  escalationLevelLabel,
} from "@features/csm-cases/utils/escalationLevel";
import SemanticChip from "@components/SemanticChip";

interface EscalationLevelChipProps {
  /** Raw escalation-level id ("0"-"5"). */
  level: string;
  /** Compact "EL2" form instead of the full "EL2 — Technology Unit Head". */
  short?: boolean;
  size?: "small" | "medium";
}

/**
 * The single source of truth for rendering a case's escalation-level badge —
 * same role `SeverityChip`/`StateChip` play for their own badges. Colour
 * ramps up (info → warning → error) with the level via
 * `escalationLevelColor`; renders outlined (quieter than the bold, filled
 * severity chip) since escalation level is supplementary context here, not
 * the primary scan signal a case row is triaged on.
 */
export default function EscalationLevelChip({
  level,
  short = false,
  size = "small",
}: EscalationLevelChipProps): JSX.Element {
  return (
    <SemanticChip
      role={escalationLevelColor(level)}
      label={escalationLevelLabel(level, short)}
      variant="outlined"
      size={size}
    />
  );
}
