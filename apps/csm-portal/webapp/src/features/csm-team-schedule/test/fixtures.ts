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

/**
 * Fixtures for the Team Schedule component tests.
 *
 * Built around a fixed week -- Monday 21 to Sunday 27 September 2026 -- so a
 * test can say "the Saturday" and mean something. The rota is full of
 * weekday/weekend rules, and a fixture anchored to the real clock would pass
 * or fail depending on the day it ran.
 */

import type {
  ScheduleAbsence,
  ScheduleAbsenceKind,
  ScheduleAssignment,
  ScheduleShift,
  ScheduleZone, ScheduleTier } from "../types";

export const MONDAY = new Date(2026, 8, 21);
export const SATURDAY_ISO = "2026-09-26";
export const TZ = "Asia/Colombo";

export function shift(over: Partial<ScheduleShift> & { code: string }): ScheduleShift {
  return {
    id: over.code,
    shortCode: over.code,
    label: over.code,
    family: "CRE",
    dayScope: "WEEKDAY",
    startMinute: 540,
    endMinute: 1080,
    authoringTimeZone: TZ,
    isOnCall: false,
    isEscalation: false,
    isRotation: true,
    crossesMidnight: false,
    colourToken: "LK",
    sortOrder: 10,
    ...over,
  } as ScheduleShift;
}

/** An SME rotation's two windows, as migration 0200 seeds Moesif's: a Day and
 *  a Night zone each, every day of the week, the tier left to the turn. */
export const MOE_DAY = shift({
  code: "SME_MOE_DAY", shortCode: "Day", label: "Moesif day escalation", family: "SME",
  zoneCode: "MOE_D", dayScope: "ANY", startMinute: 600, endMinute: 1320,
  isEscalation: true, colourToken: "TZ1", sortOrder: 910,
});
export const MOE_NIGHT = shift({
  code: "SME_MOE_NIGHT", shortCode: "Night", label: "Moesif night escalation", family: "SME",
  zoneCode: "MOE_N", dayScope: "ANY", startMinute: 1320, endMinute: 2040, crossesMidnight: true,
  isEscalation: true, colourToken: "TZ3", sortOrder: 920,
});

/** The windows these tests lean on, as the catalogue actually has them. */
export const REGULAR = shift({
  code: "CRE_REGULAR",
  shortCode: "LK",
  label: "Regular hours",
  isRotation: false,
  sortOrder: 90,
});
export const REGULAR_IND = shift({
  code: "CRE_REGULAR_IND",
  shortCode: "IND",
  label: "India region shift",
  isRotation: false,
  sortOrder: 91,
});
export const EVENING = shift({
  code: "CRE_EVENING",
  shortCode: "6-9p",
  label: "Evening 6-9pm",
  startMinute: 1080,
  endMinute: 1260,
  sortOrder: 30,
});
export const WEEKEND = shift({
  code: "CRE_WEEKEND",
  shortCode: "WE",
  label: "Weekend rotation",
  dayScope: "WEEKEND",
  sortOrder: 40,
});
export const TZ1 = shift({
  code: "SRE_TZ1",
  shortCode: "L1",
  label: "TZ1 escalation",
  family: "SRE",
  zoneCode: "TZ1",
  isEscalation: true,
  sortOrder: 10,
});
export const TZ2 = shift({
  code: "SRE_TZ2",
  shortCode: "L2",
  label: "TZ2 escalation",
  family: "SRE",
  zoneCode: "TZ2",
  isEscalation: true,
  sortOrder: 20,
});

/** TZ1's own L1 window -- the one escalation window that fixes its tier. */
export const TZ1_L1 = shift({
  code: "SRE_TZ1_L1",
  shortCode: "L1",
  label: "TZ1 L1 support",
  family: "SRE",
  zoneCode: "TZ1",
  tier: "L1",
  isEscalation: true,
  sortOrder: 5,
});
export const TZ3 = shift({
  code: "SRE_TZ3",
  shortCode: "TZ3",
  label: "TZ3 escalation",
  family: "SRE",
  zoneCode: "TZ3",
  isEscalation: true,
  sortOrder: 30,
});

/** The weekend pair. The catalogue has no weekend TZ3, which is why a
 *  weekend earns two zone columns and a weekday three -- and why the roster
 *  only splits a day at all when both scopes have zones to split into. */
export const TZ1_WE = shift({
  code: "SRE_WE_TZ1",
  shortCode: "L1",
  label: "TZ1 weekend",
  family: "SRE",
  zoneCode: "TZ1",
  dayScope: "WEEKEND",
  isEscalation: true,
  sortOrder: 50,
});
export const TZ2_WE = shift({
  code: "SRE_WE_TZ2",
  shortCode: "L2",
  label: "TZ2 weekend",
  family: "SRE",
  zoneCode: "TZ2",
  dayScope: "WEEKEND",
  isEscalation: true,
  sortOrder: 60,
});

export function shiftMap(...list: ScheduleShift[]): Map<string, ScheduleShift> {
  return new Map(list.map((s) => [s.code, s]));
}

let seq = 0;

export function assignment(over: {
  name: string;
  rotaDate: string;
  shiftCode: string;
  teamKey?: string;
  userId?: string;
  zoneCode?: string;
  startsAt?: string;
  endsAt?: string;
  isOnCall?: boolean;
  tier?: ScheduleTier;
}): ScheduleAssignment {
  seq += 1;
  const id = `a${seq}`;
  return {
    id,
    engineer: {
      userId: over.userId ?? `u-${over.name}`,
      name: over.name,
      email: `${over.name.toLowerCase()}@example.test`,
      isLead: false,
    },
    teamKey: over.teamKey ?? "alpha",
    shiftCode: over.shiftCode,
    zoneCode: over.zoneCode,
    rotaDate: over.rotaDate,
    startsAt: over.startsAt ?? `${over.rotaDate}T03:30:00.000Z`,
    endsAt: over.endsAt ?? `${over.rotaDate}T12:30:00.000Z`,
    isOnCall: over.isOnCall ?? false,
    source: "SEED",
    ...(over.tier ? { tier: over.tier } : {}),
  };
}

export const ANNUAL_LEAVE: ScheduleAbsenceKind = {
  id: "k1",
  code: "ANNUAL_LEAVE",
  shortCode: "AL",
  label: "Annual leave",
  bucket: "LEAVE",
  colourToken: "AL",
  sortOrder: 10,
};
export const LIEU_LEAVE: ScheduleAbsenceKind = {
  id: "k2",
  code: "LIEU_LEAVE",
  shortCode: "LL",
  label: "Lieu leave",
  bucket: "LEAVE",
  colourToken: "LL",
  sortOrder: 20,
};
export const RND: ScheduleAbsenceKind = {
  id: "k3",
  code: "RND",
  shortCode: "R&D",
  label: "R&D",
  bucket: "ALLOCATION",
  colourToken: "RND",
  sortOrder: 30,
};

export function absence(over: {
  name: string;
  startsOn: string;
  endsOn?: string;
  kindCode?: string;
  teamKey?: string;
  userId?: string;
}): ScheduleAbsence {
  seq += 1;
  return {
    id: `ab${seq}`,
    engineer: {
      userId: over.userId ?? `u-${over.name}`,
      name: over.name,
      email: `${over.name.toLowerCase()}@example.test`,
      isLead: false,
    },
    teamKey: over.teamKey ?? "alpha",
    kindCode: over.kindCode ?? "ANNUAL_LEAVE",
    startsOn: over.startsOn,
    endsOn: over.endsOn,
  };
}

export const ZONES: ScheduleZone[] = [
  { id: "z1", code: "TZ1", label: "Time zone 1", sortOrder: 1 },
  { id: "z2", code: "TZ2", label: "Time zone 2", sortOrder: 2 },
];

/** The group/team controls every card renders in its own head. */
export function scopeControls() {
  return {
    family: "CRE" as const,
    onFamilyChange: () => {},
    teamKey: "",
    onTeamKeyChange: () => {},
    teams: ["alpha", "bravo"],
    families: ["CRE", "SRE"] as const,
  };
}
