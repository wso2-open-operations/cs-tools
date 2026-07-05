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
  stateColor,
  stateLabel,
} from "@features/csm-dashboard/utils/abtDashboard";
import SemanticChip from "@components/SemanticChip";

interface StateChipProps {
  state: string;
  size?: "small" | "medium";
  variant?: "filled" | "outlined";
  /** Render a pointer cursor when the chip sits inside a link / clickable row. */
  clickable?: boolean;
}

/**
 * The single source of truth for rendering a case lifecycle-state badge. Renders
 * as a solid {@link SemanticChip} so the state reads as a status pill rather than
 * quiet text — but, unlike {@link SeverityChip}, it is NOT bold, so severity
 * stays the top scan signal in a row. Colour follows the "whose move is it"
 * STATE_COLOR semantics; the neutral `default` states (e.g. awaiting info) have
 * no accessible solid fill and fall back to an outlined chip.
 */
export default function StateChip({
  state,
  size = "small",
  variant,
  clickable = false,
}: StateChipProps): JSX.Element {
  return (
    <SemanticChip
      role={stateColor(state)}
      label={stateLabel(state)}
      size={size}
      variant={variant}
      clickable={clickable}
    />
  );
}
