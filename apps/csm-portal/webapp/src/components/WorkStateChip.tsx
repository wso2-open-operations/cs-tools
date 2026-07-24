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

import { Chip } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { CaseWorkState } from "@features/csm-dashboard/types/abtDashboard";
import { WORK_STATE_LABEL } from "@features/csm-cases/utils/caseWorkState";

interface WorkStateChipProps {
  workState: CaseWorkState;
  size?: "small" | "medium";
}

/**
 * The single source of truth for the work sub-state badge (Ongoing/Paused)
 * shown beside a `work_in_progress` case's state chip — same "single source
 * of truth" role `SeverityChip`/`StateChip` play for their own badges.
 */
export default function WorkStateChip({
  workState,
  size = "small",
}: WorkStateChipProps): JSX.Element {
  return (
    <Chip
      size={size}
      variant="outlined"
      color={workState === "paused" ? "warning" : "default"}
      label={WORK_STATE_LABEL[workState]}
      sx={{ fontWeight: 600 }}
    />
  );
}
