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

import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import {
  TIMECARD_ADMIN_GROUP,
  TIMECARD_APPROVER_GROUP,
} from "@features/csm-timecards/constants/timeCardConstants";

export interface TimecardRole {
  /** May approve/reject/recall time cards — the dedicated approver group only. */
  isApprover: boolean;
  /** Time-card admin: edit any user's editable cards, approve by exception. */
  isAdmin: boolean;
}

/**
 * The signed-in user's time-card role, resolved from `GET /users/me` roles
 * (platform-owned data from the entity service). `isApprover` and `isAdmin`
 * are deliberately independent — being a general portal admin no longer
 * implies time-card approval rights, and doesn't put the Approvals tab in
 * front of someone who isn't actually a time-card approver. Client-side
 * affordance only — the backend must enforce the same gates.
 */
export function useTimecardRole(): TimecardRole {
  const { data: me } = useGetUsersMe();
  const roles = (me?.roles ?? []).map((r) => r.toLowerCase());
  const isAdmin = roles.includes(TIMECARD_ADMIN_GROUP.toLowerCase());
  const isApprover = roles.includes(TIMECARD_APPROVER_GROUP.toLowerCase());
  return { isApprover, isAdmin };
}
