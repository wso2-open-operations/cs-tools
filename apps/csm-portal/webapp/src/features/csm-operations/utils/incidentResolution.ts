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

import type { BeIncidentResolutionCode } from "@api/backend/types";

/** All backend incident resolution codes, in the order declared in openapi.yaml. */
export const INCIDENT_RESOLUTION_CODES: BeIncidentResolutionCode[] = [
  "SOLVED_WORKAROUND",
  "SOLVED_PERMANENTLY",
  "NOT_SOLVED_NOT_REPRODUCIBLE",
  "FALSE_ALARM",
  "DUPLICATE",
  "NOT_ACTIONABLE",
];

/**
 * Display text for each incident resolution code. "Duplicate" is deliberately
 * not "Duplicate Alert" (the backing data source's own UI label for that
 * choice) — this platform isn't an alerting feature.
 */
export const INCIDENT_RESOLUTION_CODE_LABELS: Record<BeIncidentResolutionCode, string> = {
  SOLVED_WORKAROUND: "Solved (Work Around)",
  SOLVED_PERMANENTLY: "Solved (Permanently)",
  NOT_SOLVED_NOT_REPRODUCIBLE: "Not Solved (Not Reproducible)",
  FALSE_ALARM: "False Alarm",
  DUPLICATE: "Duplicate",
  NOT_ACTIONABLE: "Not Actionable Alert",
};
