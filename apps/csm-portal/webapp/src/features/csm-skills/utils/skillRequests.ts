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


import type { SkillAutoAddNote, SkillHistoryEntry, SkillRequest, SkillRequestStatus } from "@features/csm-skills/types";

/** One line describing a history row, by its recorded action. */
export function describeHistoryEntry(e: SkillHistoryEntry): string {
  const to = e.levelTo?.name ?? "no level";
  const from = e.levelFrom?.name ?? "no level";
  switch (e.action) {
    case "ADDED":
      return `Added at ${to}`;
    case "READDED":
      return `Added again at ${to}`;
    case "REMOVED":
      return `Removed (was ${from})`;
    case "LEVEL_CHANGED":
      return `${from} → ${to}`;
    default:
      // A migrated row without an action: read it from the levels.
      if (!e.levelFrom && e.levelTo) return `Added at ${to}`;
      if (e.levelFrom && !e.levelTo) return `Removed (was ${from})`;
      return `${from} → ${to}`;
  }
}

export const REQUEST_STATUS_LABEL: Record<SkillRequestStatus, string> = {
  PENDING: "Pending",
  APPROVED: "Approved",
  REJECTED: "Rejected",
  WITHDRAWN: "Withdrawn",
};

export const REQUEST_STATUS_COLOR: Record<SkillRequestStatus, "warning" | "success" | "error" | "default"> = {
  PENDING: "warning",
  APPROVED: "success",
  REJECTED: "error",
  WITHDRAWN: "default",
};

/** What approving did to the requester's profile, in their words. */
export function describeAutoAdd(req: Pick<SkillRequest, "autoAddNote" | "autoAddedLevel">): string | null {
  const note: SkillAutoAddNote | null | undefined = req.autoAddNote;
  const level = req.autoAddedLevel?.name;
  switch (note) {
    case "ADDED":
    case "READDED":
      return level ? `Added to your skills at ${level}.` : "Added to your skills.";
    case "LEVEL_ADJUSTED":
      return level ? `Added to your skills at ${level} (the reviewer set the level).` : "Added to your skills.";
    case "ALREADY_HELD":
      return "You already had this skill, so your level was kept.";
    case "SKIPPED_NO_SCALE":
      return "This skill has no level scale, so it wasn't added to your skills.";
    case "SKIPPED_INACTIVE_USER":
      return "It wasn't added to a profile: the requester is no longer active.";
    default:
      return null;
  }
}

/** A person's display name. */
export function personName(p: { name?: string | null; email?: string | null; id: string } | null | undefined): string {
  if (!p) return "";
  return p.name || p.email || p.id;
}
