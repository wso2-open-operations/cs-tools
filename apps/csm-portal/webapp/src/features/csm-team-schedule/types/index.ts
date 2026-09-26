/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/** An SRE time zone. `weekendZoneCode` names the zone that absorbs this one
 *  at the weekend, when three weekday zones collapse into two. */
export interface ScheduleZone {
  id: string;
  code: string;
  label: string;
  weekendZoneCode?: string;
  sortOrder: number;
}

/**
 * A named window of the working day. Minutes are counted from midnight in
 * `authoringTimeZone`; an end past 1440 runs into the next day, so the night
 * block 21:00-06:00 arrives as one window (1260 -> 1800).
 */
export interface ScheduleShift {
  id: string;
  code: string;
  shortCode: string;
  label: string;
  family: "CRE" | "SRE";
  zoneCode?: string;
  tier?: ScheduleTier;
  dayScope: "WEEKDAY" | "WEEKEND" | "ANY";
  startMinute: number;
  endMinute: number;
  authoringTimeZone: string;
  isOnCall: boolean;
  isEscalation: boolean;
  /** False for a window that is simply when a team works -- regular hours,
   *  or the Americas night -- rather than a turn on the rota. */
  isRotation: boolean;
  crossesMidnight: boolean;
  colourToken: string;
  sortOrder: number;
}

export type ScheduleTier = "L1" | "L2" | "L3";

/** Why someone is out of the rota. */
export interface ScheduleAbsenceKind {
  id: string;
  code: string;
  shortCode: string;
  label: string;
  bucket: "LEAVE" | "ALLOCATION" | "EXCLUDED";
  colourToken: string;
  sortOrder: number;
}

export interface ScheduleCatalogue {
  zones: ScheduleZone[];
  shifts: ScheduleShift[];
  absenceKinds: ScheduleAbsenceKind[];
}

export interface ScheduleEngineer {
  userId: string;
  name: string;
  email: string;
  isLead: boolean;
}

/**
 * One engineer, one rota day, one window.
 *
 * `rotaDate` is the day the CREW is rostered for, not the calendar date of
 * every hour worked: a Monday 21:00-06:00 block carries the Monday even though
 * six of its hours fall on Tuesday. `startsAt`/`endsAt` are absolute instants,
 * so rendering them in the reader's clock needs no further arithmetic.
 */
export interface ScheduleAssignment {
  id: string;
  engineer: ScheduleEngineer;
  teamKey: string;
  shiftCode: string;
  zoneCode?: string;
  tier?: ScheduleTier;
  rotaDate: string;
  startsAt: string;
  endsAt: string;
  isOnCall: boolean;
  source: string;
  note?: string;
}

/** Whole days out of the rota. `endsOn` absent means "until further notice". */
export interface ScheduleAbsence {
  id: string;
  engineer: ScheduleEngineer;
  teamKey: string;
  kindCode: string;
  startsOn: string;
  endsOn?: string;
  note?: string;
}

export interface SearchScheduleAssignmentsPayload {
  from: string;
  to: string;
  teamKeys?: string[];
  family?: "CRE" | "SRE";
  userId?: string;
  /** Find one engineer's own rota. The portal knows its users by email, so
   *  entity-service resolves that rather than every client doing it. */
  userEmail?: string;
  /** Widen the window to catch a block that began before `from` and is still
   *  running into it -- the night crew a day view shows at the top. */
  includeOvernight?: boolean;
}

export interface SearchScheduleAbsencesPayload {
  from: string;
  to: string;
  teamKeys?: string[];
  userId?: string;
  userEmail?: string;
}

export interface ScheduleAssignmentsResponse {
  assignments: ScheduleAssignment[];
  count: number;
}

export interface ScheduleAbsencesResponse {
  absences: ScheduleAbsence[];
  count: number;
}
