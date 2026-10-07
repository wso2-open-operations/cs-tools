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

/** The rota families: CRE's ABT rota, SRE, and SME -- the product special
 *  rotations. Within SRE and SME the work is split further into rotas. */
export type RotaFamily = "CRE" | "SRE" | "SME";

/** A block of the day a rota is worked in: SaaS SRE's time zones, or a
 *  rotation's Day and Night. `weekendZoneCode` names the zone that absorbs
 *  this one at the weekend, when three weekday zones collapse into two.
 *  `rotaCode` is the rota the zone belongs to, absent for one no rota claims. */
export interface ScheduleZone {
  id: string;
  code: string;
  label: string;
  weekendZoneCode?: string;
  sortOrder: number;
  rotaCode?: string;
}

/** A named rotation inside a family -- SRE's SaaS and IaaS, one per SME
 *  product -- with the rules its own sheet states. */
export interface ScheduleRota {
  code: string;
  label: string;
  family: RotaFamily;
  rotates: "DAILY" | "WEEKLY" | "IRREGULAR";
  /** Informational: the escalation ladder keeps its own timing. */
  escalationMinutes?: number;
  sourceSheet?: string;
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
  family: RotaFamily;
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
  /** A tag a lead added from the portal, which a lead may also delete. The
   *  catalogue's own kinds are never custom. */
  custom?: boolean;
  /** The rota the kind is offered on; absent for a kind every rota uses,
   *  which is every kind of leave. SRE allocates RnD, CRE allocates Migration. */
  family?: RotaFamily;
  /** No longer offered. Still served so the days already marked with it keep
   *  their label, but a picker must not offer it. */
  retired?: boolean;
  /** The team a span of this kind is spent working for -- the Brazil
   *  rotation moves someone to the Americas team. Such a span is filed under
   *  that team, so the roster shows the person there for its dates. */
  movesToTeamKey?: string;
  /** That stint is rota work on the other team, so the person is not listed
   *  as off the rota on its days. */
  worksRotaThere?: boolean;
  /** The standing window a span of this kind is drawn as on the roster: on
   *  the team it moves someone to, they work its normal hours (NLK, LK). */
  showsAsShiftCode?: string;
}

/** One team the rota is run for, served so no client holds the list. */
export interface ScheduleTeam {
  key: string;
  name: string;
  family: RotaFamily;
  /** Display order, and what gives a team a stable colour. */
  sortOrder: number;
  /** The rota this team's type belongs to; absent for a team on no named
   *  rota (CRE's teams, Americas). */
  rotaCode?: string;
  /** Everyone on the team, with their role. Older servers do not send it. */
  members?: ScheduleTeamMember[];
  /** The standing window a member's ordinary weekday is (Americas cover for
   *  the Americas team). Absent means Regular hours. */
  defaultShiftCode?: string;
}

/** One member of a rota team. role is as stored: engineer (or member), lead,
 *  americas_team_lead, ... */
export interface ScheduleTeamMember extends ScheduleEngineer {
  role: string;
}

export interface ScheduleCatalogue {
  zones: ScheduleZone[];
  shifts: ScheduleShift[];
  absenceKinds: ScheduleAbsenceKind[];
  teams: ScheduleTeam[];
  /** Absent from a server older than the rotas; read as "no named rotas". */
  rotas?: ScheduleRota[];
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
  /** Who an allocation is for -- the customer, or the product team for RnD.
   *  The kind says what sort of time it is; this says for whom. */
  allocatedTo?: string;
  /** The person's own team, on a span filed under the team a kind moved them
   *  to (teamKey). Its lead still owns the span. */
  homeTeamKey?: string;
}

/** One leave or allocation span as a roster cell hands it to the picker, so
 *  the picker can remove the whole span rather than the day that was clicked. */
export interface CellAbsence {
  id: string;
  kindCode: string;
  startsOn: string;
  /** Absent for a span that runs until further notice. */
  endsOn?: string;
  allocatedTo?: string;
  /** The person's own team, on a span that moved them to another one. */
  homeTeamKey?: string;
}

export interface SearchScheduleAssignmentsPayload {
  from: string;
  to: string;
  teamKeys?: string[];
  family?: RotaFamily;
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

/** One recorded change to a team's rota or leave, as the history keeps it. */
export interface ScheduleActivity {
  id: string;
  assignmentId: string;
  userId: string;
  teamKey: string;
  /** The day changed; for leave, the first day of the span. */
  rotaDate: string;
  subject: "rota" | "leave";
  /** For leave, the last day of the span. */
  endsOn?: string;
  /** The window's code, or for leave the absence kind's code. */
  shiftCode: string;
  action: "CREATED" | "UPDATED" | "DELETED" | "TRIMMED";
  actorEmail: string;
  note?: string;
  /** For an UPDATED row, which field moved and what it moved between. The
   *  service has always sent these; the panel needs them to say what an edit
   *  actually was rather than guessing. */
  fieldName?: string;
  oldValue?: string;
  newValue?: string;
  createdOn: string;
}

/** One roster cell that a person has changed, and who changed it last. */
export interface ScheduleEditMarker {
  userId: string;
  rotaDate: string;
  actor: string;
  changedAt: string;
  action: "CREATED" | "UPDATED" | "DELETED";
}

export interface ScheduleEditMarkersResponse {
  markers: ScheduleEditMarker[];
  count: number;
}
