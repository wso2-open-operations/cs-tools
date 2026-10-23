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
  /** Each tier of this window is one person for the whole rota, not one per
   *  team: SaaS TZ3, the night cover for Apollo and Artemis alike. Absent
   *  from an older entity-service, which reads as false. */
  sharedAcrossTeams?: boolean;
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
  /** An ABT: the teams the CRE rota and its regular hours are run by, as
   *  against Americas, Migration and the like. Older servers do not send it. */
  isAbt?: boolean;
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
  /** Leave out anybody on leave that day: who a reminder would reach. */
  excludeAbsent?: boolean;
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

/* ── Case Paging ────────────────────────────────────────────────────────────
 * The system pages people about an incident tier by tier until someone
 * acknowledges. Every person on a tier is an existing team membership. */

/** How a paging number's last test call went. */
export type PagingTestStatus =
  "pending" | "completed" | "no-answer" | "busy" | "failed";

/** A person's paging-only phone number: kept by entity-service, used only
 *  when their profile has none, never written to their profile. */
export interface PagingPhone {
  /** "+94•••••123" -- the only form the page shows. */
  masked: string;
  /** The full number, sent only to a reader who may edit it (to prefill). */
  phone?: string;
  setBy: string;
  setAt: string;
  lastTestAt?: string;
  lastTestStatus?: PagingTestStatus;
}

/** The number a test call rings for a person -- the one Case Alerts dials
 *  from CSM -- and its last test. Sent only to a reader who may test it. */
export interface PagingTestCall {
  /** "+94•••••123" -- the only form the page shows. */
  masked: string;
  /** "profile": the number on their own profile; "paging": their paging-only number. */
  source: "profile" | "paging";
  lastTestAt?: string;
  lastTestStatus?: PagingTestStatus;
}

/** One membership as the Case Paging tab shows it. */
export interface PagingChainMember {
  membershipId: string;
  teamKey: string;
  teamName: string;
  teamType: string;
  family: string;
  userId: string;
  name: string;
  email: string;
  /** engineer, lead, americas_team_lead (the America lead), cre_head or cs_head. */
  role: string;
  /** 1, 2 or 3 for the team's 1st, 2nd or 3rd responder (Tier 1); 0 for none. */
  responderRank: number;
  /** Whether the reader may change or remove this person's paging number. */
  canEditPhone?: boolean;
  pagingPhone?: PagingPhone | null;
  /** Whether their profile has a mobile number; absent when it could not be checked. */
  hasProfilePhone?: boolean;
  /** Whether the reader may place a test call to this person (a portal admin,
   *  or a rota admin of the person's family). */
  canTestCall?: boolean;
  /** What a test call would ring; absent when the reader may not test, or
   *  CSM holds no number for the person. */
  testCall?: PagingTestCall | null;
}

/** What the reader may change; the server checks every edit again. */
export interface PagingChainPermissions {
  responderTeams: string[];
  teamLeadTeams: string[];
  americasTeamLead: boolean;
  heads: boolean;
}

export interface PagingChainResponse {
  family: string;
  members: PagingChainMember[];
  count: number;
  canEdit: PagingChainPermissions;
}

/** The roles a paging change may give a membership. */
export type PagingRole =
  "lead" | "engineer" | "americas_team_lead" | "cre_head" | "cs_head";

/** Exactly one change to one membership. */
export type PagingMemberChange =
  | { membershipId: string; responderRank: number }
  | { membershipId: string; role: PagingRole };

/** The paging chains the readiness check answers for. */
export type PagingChainCode = "CRE" | "SRE_SAAS" | "SRE_IAAS" | "SME";

/** What it takes to close a readiness gap, which decides the action offered. */
export type PagingGapFix =
  "responders" | "rota" | "profile" | "config" | "data";

/** One thing standing between a chain and being ready. */
export interface PagingReadinessGap {
  code: string;
  severity: "error" | "warning";
  message: string;
  fix: PagingGapFix;
  teamKey?: string;
  /** The day the gap is on, YYYY-MM-DD, where it is on one. */
  date?: string;
  zoneCode?: string;
  tier?: string;
  shiftCode?: string;
  /** The person a phone gap (NO_PHONE, PHONE_UNTESTED, PHONE_TEST_FAILED) is about. */
  userId?: string;
  /** On a phone gap: the Case Paging tier that first calls the person
   *  (1 = first). The server lists phone gaps in this order. */
  pagingTier?: number;
}

export interface PagingReadinessChain {
  chain: PagingChainCode;
  label: string;
  ready: boolean;
  gaps: PagingReadinessGap[];
}

export interface PagingReadinessResponse {
  generatedAt: string;
  from: string;
  to: string;
  chains: PagingReadinessChain[];
}

/**
 * The Rota access tab: who may change the rotas beyond the team leads.
 *
 * A rota admin edits every team in one family. A rota marker edits one team's
 * rota -- rotations, leave and allocations -- and nothing else. canManage is
 * what the reader may change: rota admins for a CSM admin, and the markers of
 * markerFamilies (every family for a CSM admin, their own for a rota admin).
 * Somebody who may change nothing gets empty lists, and the tab is not shown.
 */
export interface RotaAccess {
  /** markerTeams: the teams whose Rota Managers the reader may change as their
   *  lead. Absent from an older server. */
  canManage: { rotaAdmins: boolean; markerFamilies: RotaFamily[]; markerTeams?: string[] };
  rotaAdmins: RotaAdminGrant[];
  markers: RotaMarkerGrant[];
}

interface RotaGrant {
  userId: string;
  name: string;
  email: string;
  grantedBy: string;
  grantedOn: string;
  /** False when the grant does nothing: the holder is not internal staff,
   *  or (a marker) has left the team. Listed so it can be removed. */
  active: boolean;
}

export interface RotaAdminGrant extends RotaGrant {
  family: RotaFamily;
}

export interface RotaMarkerGrant extends RotaGrant {
  teamKey: string;
  family: RotaFamily;
}

/**
 * The settings section of the Alert Settings tab: the chat rooms, ServiceNow
 * group routes and ladder values the paging ladders use. Changed here instead
 * of a Choreo secret and csm-notification-service's file mount; the alert
 * service uses them only once it runs with PAGING_CONFIG_SOURCE set, and only
 * after a restart.
 * canManage.view is false for anybody who may not see them, and the section is
 * not shown.
 */
export type CaseAlertLadder = "CRE" | "SRE";

/** A ladder with chat rooms: CRE's and SRE's, and SME's, which has only its
 *  teams' rooms here (no shared room, routes or ladder values). */
export type CaseAlertRoomLadder = CaseAlertLadder | "SME";

export interface CaseAlertSettings {
  canManage: {
    view: boolean;
    roomLadders: CaseAlertRoomLadder[];
    routeLadders: CaseAlertLadder[];
  };
  /** e.g. "staging" or "production"; "" when the server has no label. */
  environment: string;
  /** False only from a server older than 0237 with no encryption key. */
  webhooksEnabled: boolean;
  revision: {
    saved: number;
    changedAt?: string;
    /** What the alert service last reported it loaded. */
    applied?: number;
    appliedAt?: string;
    appliedMode?: "off" | "shadow" | "db";
  };
  rooms: CaseAlertChatRoom[];
  routes: CaseAlertGroupRoute[];
  /** The ladder values saved here: the rest of the paging file's keys. */
  values: CaseAlertLadderValue[];
  /** What the alert service last reported it runs with. Absent until it has
   *  started in shadow or db mode; off never reports. */
  running?: CaseAlertRunning;
  /** The lists a group route is made from; absent for somebody who may see nothing. */
  routeOptions?: CaseAlertRouteOptions;
  /** SRE viewers only: active assignment groups no enabled SRE redirect
   *  covers and no Team Schedule team is named after. Their incident cards go
   *  to the shared SRE room. Absent when there are none. */
  unmappedGroups?: CaseAlertAssignmentGroup[];
}

/** A ServiceNow assignment group (a "group" row that is a service's support group). */
export interface CaseAlertAssignmentGroup {
  id: string;
  name: string;
  /** The services it is the support group of. Absent from an older server. */
  services?: string[];
}

/** A team as a route option: CRE, SRE or SME. */
export interface CaseAlertTeamOption {
  key: string;
  name: string;
  family: "CRE" | "SRE" | "SME";
}

/**
 * What a group route is made of: the CSM Portal teams a case or incident can
 * be assigned to (matched by name), the Team Schedule teams a route can send
 * it to, and the active SRE rotas.
 */
export interface CaseAlertRouteOptions {
  groups: CaseAlertTeamOption[];
  teams: CaseAlertTeamOption[];
  rotas: { code: string; label: string }[];
  /** The active assignment groups a redirect can be picked for. Absent from
   *  an older server. */
  assignmentGroups?: CaseAlertAssignmentGroup[];
}

/** A chat room: a ladder's shared room (no teamKey) or an SRE or SME team's
 *  own. The webhook URL itself never reaches the page, only a hint. */
export interface CaseAlertChatRoom {
  id: string;
  ladder: CaseAlertRoomLadder;
  teamKey?: string;
  /** An SRE assignment group's own room: the group, and its current name. */
  groupId?: string;
  groupName?: string;
  label: string;
  enabled: boolean;
  /** Also gets the cards of an incident with no SRE team. */
  teamless: boolean;
  /** A team room's own channel. Absent: the same as the ladder's. The shared
   *  room never has one. */
  channel?: CaseAlertChannel;
  hasWebhook: boolean;
  webhookHint?: string;
  webhookSetAt?: string;
  webhookSetBy?: string;
  lastTestAt?: string;
  lastTestStatus?: "delivered" | "failed";
  updatedBy: string;
  updatedOn: string;
}

/** How a ladder, or one team on it, is alerted. A team can do less than its
 *  ladder, never more: under chat, only chat or log. */
export type CaseAlertChannel = "log" | "chat" | "call" | "both";

/** A ServiceNow group's alerts go to a team, or (SRE) to a default rota. */
export interface CaseAlertGroupRoute {
  id: string;
  ladder: CaseAlertLadder;
  groupName: string;
  /** The assignment group the redirect was picked for; absent for one saved
   *  by name. Its groupName is then the group's current name. */
  groupId?: string;
  teamKey?: string;
  rotaCode?: string;
  enabled: boolean;
  updatedBy: string;
  updatedOn: string;
}

/** One key of the paging file's cre: or sre: section, saved here. A saved
 *  value replaces that key of the file; removing it goes back to the file. The
 *  value is the file's own shape for the key: a list, a number, a head's name
 *  and email, and so on. */
export interface CaseAlertLadderValue {
  id: string;
  ladder: CaseAlertRoomLadder;
  key: string;
  value: unknown;
  updatedBy: string;
  updatedOn: string;
}

/** Where a running value came from: the file, or a value saved here. */
export type CaseAlertValueSource = "file" | "saved";

export interface CaseAlertRunningValue {
  value: unknown;
  source: CaseAlertValueSource;
}

export interface CaseAlertRunning {
  /** When the alert service reported; absent when the server has no time. */
  reportedAt?: string;
  mode: "off" | "shadow" | "db";
  /** Per ladder, each key's value in use. */
  ladders: Partial<
    Record<CaseAlertRoomLadder, Record<string, CaseAlertRunningValue>>
  >;
  /** Per ladder, the settings only the file can set. Absent from an alert
   *  service that doesn't report them yet. */
  fileOnly?: Partial<Record<CaseAlertRoomLadder, CaseAlertFileOnly>>;
  /** Per ladder, whether it runs, its channel, and each team's channel as
   *  the alert service runs it. Absent from an alert service that doesn't
   *  report them yet. */
  channels?: Partial<Record<CaseAlertRoomLadder, CaseAlertLadderChannels>>;
  /** Per ladder, how the running config routes an incident's assignment
   *  group with no redirect saved here. Absent from an alert service that
   *  doesn't report it yet. */
  groups?: Partial<Record<CaseAlertLadder, CaseAlertGroupRouting>>;
}

/** How a ladder's running config routes assignment groups. */
export interface CaseAlertGroupRouting {
  /** Group name, lower-cased, to the team key it is alerted as. */
  aliases?: Record<string, string>;
  /** SRE: the groups paged on the default rota. */
  defaultGroups?: string[];
  /** SRE: teams with a chat space but no rota of their own. */
  teamlessRooms?: string[];
  /** SRE: the groups (lower-cased) with a chat space of their own in use. */
  groupRooms?: string[];
  /** SRE: whether the ladder has its shared space, where a group that maps
   *  to no space gets its card. Absent from an older report. */
  sharedRoom?: boolean;
}

/** One team's channel as running: the ladder's narrowed by the team's own. */
export interface CaseAlertTeamChannel {
  team: string;
  channel: string;
  /** The team's own saved channel, when it has one. */
  teamChannel?: string;
  /** Whether the team has a chat room of its own. */
  room: boolean;
}

export interface CaseAlertLadderChannels {
  /** The ladder runs: its own switch and the file's master switch are on. */
  enabled: boolean;
  channel: string;
  teams: CaseAlertTeamChannel[];
}

/** What decides whether a ladder pages and what it may cost: read here, set
 *  only in the alert service's file. */
export interface CaseAlertFileOnly {
  enabled?: boolean;
  channel?: string;
  "safety.maxCallsPerLadder"?: number;
  /** Absent when there is no cap. */
  "safety.maxLevel"?: number;
  /** How many numbers are allowed; the numbers themselves are never sent. */
  "safety.allowedNumbers"?: number;
  "safety.callWithoutVerifiedLeads"?: boolean;
  "safety.callHeadsWithoutLowerTiers"?: boolean;
  "phones.source"?: string;
  /** Whether a stopped chain replies in its thread. */
  ackChat?: boolean;
  /** CRE: one email per step. The lists are counted or flagged, never sent. */
  "email.enabled"?: boolean;
  "email.teamLists"?: number;
  "email.creTeamList"?: boolean;
  "email.csTeamList"?: boolean;
  /** CRE: true when the file replaces the built-in rule table. */
  "rules.custom"?: boolean;
  /** CRE: minutes after the case is created that each step (L0 to L4) is
   *  called, by severity (S0 to S4). From the code, not the file. */
  timing?: Partial<Record<string, number[][]>>;
}

/** A rotation's reminder window: "HH:MM" on the 24-hour clock of an IANA zone. */
export type ReminderWindow = { start: string; end: string; timeZone: string };

/**
 * What a group's rota reminder emails may be changed to: whether the weekday
 * and weekend reminders and the calendar invitations go out, how many people a
 * weekend rotation needs each day before the weekend is announced, the hours of
 * the rotations kept on a fixed clock, and who is copied on every reminder.
 * Keyed by rotation code; a code this form does not show is kept as saved.
 */
export type SaveRotaReminderSettings = {
  weekdayEnabled: boolean;
  weekendEnabled: boolean;
  invitesEnabled: boolean;
  weekendRequired: Record<string, number>;
  windows: Record<string, ReminderWindow>;
  copyTo: string[];
};

/** The saved settings of one group, with who last changed them. */
export type RotaReminderSettings = SaveRotaReminderSettings & {
  family: RotaFamily;
  updatedBy: string;
  updatedOn: string;
};

/** Which reminder is asked for: the next working day's, or the weekend's. */
export type RotaReminderKind = "weekday" | "weekend";

/** Where a request is: waiting for the scheduled task, being sent, or done. */
export type RotaReminderStatus = "QUEUED" | "SENDING" | "SENT" | "FAILED";

/**
 * One ask to send a group's rota reminders now, picked up by the scheduled task.
 * `test` is true for a test copy, which goes only to the person who asked.
 */
export type RotaReminderRequest = {
  id: string;
  family: RotaFamily;
  kind: RotaReminderKind;
  test: boolean;
  status: RotaReminderStatus;
  requestedBy: string;
  requestedOn: string;
  claimedOn?: string;
  handledOn?: string;
  result?: string;
};

/**
 * What is posted to ask for a reminder now. With `test` the reminder goes only
 * to the person asking (marked "[Test copy]"), never to the rota, and is not
 * recorded as a real reminder.
 */
export type SendRotaReminder = { kind: RotaReminderKind; test: boolean };
