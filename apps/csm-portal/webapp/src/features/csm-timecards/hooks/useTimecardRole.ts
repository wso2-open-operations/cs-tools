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

import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { resolveUserInfo } from "@utils/userClaims";
import {
  TIMECARD_ADMIN_GROUP,
  TIMECARD_APPROVER_GROUP,
} from "@features/csm-timecards/constants/timeCardConstants";

export interface TimecardRole {
  /** May approve/reject/recall time cards (approver group, or admin). */
  isApprover: boolean;
  /** Time-card admin: edit any user's editable cards, approve by exception. */
  isAdmin: boolean;
}

/**
 * The signed-in user's time-card role, derived from the Asgardeo `groups` claim.
 * Phase 1 / FE-first interim: role flags should come from `GET /users/me` once
 * the backend exposes `isTimecardApprover` / `isTimecardAdmin` (ISSU-009 Phase 2).
 * Admins are a superset of approvers. Client-side affordance only — the backend
 * must enforce the same gates.
 */
export function useTimecardRole(): TimecardRole {
  const { groups } = resolveUserInfo(useIdTokenClaims());
  const lower = groups.map((g) => g.toLowerCase());
  const isAdmin = lower.includes(TIMECARD_ADMIN_GROUP.toLowerCase());
  const isApprover = isAdmin || lower.includes(TIMECARD_APPROVER_GROUP.toLowerCase());
  return { isApprover, isAdmin };
}
