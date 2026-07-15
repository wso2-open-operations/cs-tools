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

import type { TaskSlaViewDto } from "./sla.dto";

export type SlaStage = "in_progress" | "paused" | "completed" | "cancelled" | "breached";

export interface CaseSla {
  id: string;
  /** SLA definition name, e.g. "S1 - Response". */
  definitionName: string;
  stage: string | null;
  hasBreached: boolean;
  businessTimeLeft: string | null;
  businessElapsedTime: string | null;
  /** 0-100+, the percentage of the target already elapsed. */
  businessElapsedPercent: number | null;
}

export function toCaseSla(dto: TaskSlaViewDto): CaseSla {
  return {
    id: dto.id,
    definitionName: dto.slaDefinition?.name || "SLA",
    stage: dto.stage,
    hasBreached: dto.stage === "breached",
    businessTimeLeft: dto.businessTimeLeft,
    businessElapsedTime: dto.businessElapsedTime,
    businessElapsedPercent: dto.businessElapsedPercentage,
  };
}
