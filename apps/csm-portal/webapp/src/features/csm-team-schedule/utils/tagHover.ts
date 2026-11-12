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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import type { ScheduleAbsenceKind } from "../types";

/** Said wherever a tag that takes people off the timesheet is explained. */
export const OFF_TIMESHEET_NOTE = "Not on the timesheet";

/** A tag added from the portal is its short name: its chip and its name are
 *  the same text. */
function nameIsChip(kind: Pick<ScheduleAbsenceKind, "label" | "shortCode">): boolean {
  return kind.label.trim().toLowerCase() === kind.shortCode.trim().toLowerCase();
}

/** What sits beside a tag's chip in the picker and the legend: its name, or
 *  for a tag whose name is the chip, its description (nothing when it has
 *  none), so the same text is never shown twice. */
export function tagSideText(
  kind: Pick<ScheduleAbsenceKind, "label" | "shortCode" | "description">,
): string {
  if (!nameIsChip(kind)) return kind.label;
  return kind.description?.trim() ?? "";
}

/** What hovering a tag shows: its name, its description when it has one,
 *  and whether its days take the person off the timesheet. */
export function tagHoverText(
  kind: Pick<ScheduleAbsenceKind, "label" | "description" | "excludesFromTimesheet">,
): string {
  const parts = [kind.label];
  const description = kind.description?.trim();
  if (description) parts.push(description);
  if (kind.excludesFromTimesheet) parts.push(OFF_TIMESHEET_NOTE);
  return parts.join(" — ");
}
