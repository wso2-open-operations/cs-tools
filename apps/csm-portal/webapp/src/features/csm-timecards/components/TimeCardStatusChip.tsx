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
import SemanticChip from "@components/SemanticChip";
import { TIME_CARD_STATE_META } from "@features/csm-timecards/constants/timeCardConstants";
import type { TimeCardState } from "@features/csm-timecards/types/timeCards";

/** Status pill for a time card's approval state. Single source of truth for its
 * label + colour (see {@link TIME_CARD_STATE_META}). */
export default function TimeCardStatusChip({
  state,
  size = "small",
}: {
  state: TimeCardState;
  size?: "small" | "medium";
}): JSX.Element {
  const meta = TIME_CARD_STATE_META[state];
  return <SemanticChip role={meta.role} label={meta.label} size={size} variant={meta.variant} />;
}
