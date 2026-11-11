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

/**
 * Stage of a work item SLA record, as returned by the SLA list endpoint. An
 * open enum: the known values below still drive autocomplete, but a stage
 * outside this set (e.g. one added later on the backend) is kept as-is
 * rather than forced into the closed set.
 */
export type SlaStage =
  | "in_progress"
  | "paused"
  | "completed"
  | "cancelled"
  | "breached"
  | (string & {});

/**
 * A single SLA record attached to a work item (a case, an incident, ...). All time fields
 * are pre-formatted server-side (`*Label`) — the frontend renders them as-is
 * rather than recomputing.
 */
export interface WorkItemSla {
  id: string;
  /** SLA definition name (e.g. "S1 - Response"). */
  definition: string;
  /** Target duration as a display string (e.g. "4 Business Hours"); absent for open-ended SLAs. */
  target: string | null;
  stage: SlaStage;
  /** Human-readable stage label from the backend (e.g. "In progress"). */
  stageLabel: string;
  hasBreached: boolean;
  businessTimeLeftLabel: string;
  businessElapsedLabel: string;
  /** Percentage (0-100+) of the target consumed in business time. */
  businessElapsedPercent: number;
  /** ISO-8601 UTC; null when the SLA clock hasn't started. */
  startTime: string | null;
  /** ISO-8601 UTC; null while the SLA is still running. */
  stopTime: string | null;
}

export interface WorkItemSlaList {
  /** The backend's true total, which may exceed the page of rows fetched. */
  count: number;
  slas: WorkItemSla[];
}

/**
 * Raw SLA record shape returned by the task-SLA search endpoint. One record
 * per SLA definition attached to a task (a case or an incident is a task). Kept close to the
 * wire shape; {@link WorkItemSla} is the row model the SLA table actually renders,
 * built from this by {@link useGetWorkItemSlas}.
 */
export interface TaskSlaView {
  id: string;
  slaDefinition: {
    id?: string | null;
    name?: string | null;
    type?: string | null;
    target?: string | null;
  } | null;
  stage: string | null;
  task: {
    id?: string | null;
    number?: string | null;
  } | null;
  businessTimeLeft: string | null;
  businessElapsedTime: string | null;
  businessElapsedPercentage: number | null;
  startTime: string | null;
  endTime: string | null;
}

export interface TaskSlaSearchPayload {
  filters?: {
    taskIds?: string[];
  };
  pagination?: {
    limit?: number;
    offset?: number;
  };
}

export interface TaskSlaSearchResponse {
  slas: TaskSlaView[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * One currently-active clock's own outcome from `POST /cases/{id}/sla/recalculate`
 * (admin-only). `skipReason` is only present when `skipped` is true;
 * `oldBusinessPauseSeconds`/`newBusinessPauseSeconds` are only present when
 * the clock's own schedule is business-days.
 */
export interface CaseSlaRecalcClock {
  target: string;
  schedule: string;
  skipped: boolean;
  skipReason?: string;
  oldPauseSeconds: number;
  newPauseSeconds: number;
  oldBusinessPauseSeconds?: number;
  newBusinessPauseSeconds?: number;
  updated: boolean;
}

/**
 * Response for `POST /cases/{id}/sla/recalculate`. `hasHistory` is false
 * (clocks always empty) when the case has no state-change activity history
 * to replay -- every clock is left exactly as it is, never guessed at.
 */
export interface CaseSlaRecalcResponse {
  hasHistory: boolean;
  clocks: CaseSlaRecalcClock[];
}
