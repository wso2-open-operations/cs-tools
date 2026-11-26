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

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  buildScheduleSearchRequest,
  toSearchAbsencesResult,
  toSearchRotaResult,
} from "@features/csm-custom-dashboards/api/codeDashboardTeamSchedule";

const ME = "jane.doe@example.com";

describe("buildScheduleSearchRequest", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("sends the window, the team keys in lower case and the viewer's email for mine", () => {
    expect(
      buildScheduleSearchRequest("searchRota", { startDate: "2026-11-16", endDate: "2026-11-22", teams: ["Rigel", " apollo "], mine: true }, ME),
    ).toEqual({ from: "2026-11-16", to: "2026-11-22", teamKeys: ["rigel", "apollo"], userEmail: ME });
  });

  it("sends only the window when nothing else is asked, and nothing for mine: false", () => {
    expect(buildScheduleSearchRequest("searchAbsences", { startDate: "2026-11-16", endDate: "2026-11-16", mine: false }, ME)).toEqual({
      from: "2026-11-16",
      to: "2026-11-16",
    });
  });

  it("reads tokens as the viewer's local dates", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 10, 18, 12, 0));
    expect(buildScheduleSearchRequest("searchRota", { startDate: "__startOfWeek:0__", endDate: "__endOfWeek:0__" }, ME)).toEqual({
      from: "2026-11-16",
      to: "2026-11-22",
    });
  });

  it("takes a window of exactly 70 days", () => {
    expect(buildScheduleSearchRequest("searchRota", { startDate: "2026-11-01", endDate: "2027-01-10" }, ME)).toEqual({
      from: "2026-11-01",
      to: "2027-01-10",
    });
  });

  it.each([
    [{ startDate: "2026-11-16" }, "searchRota: startDate and endDate are both needed"],
    [{ endDate: "2026-11-16" }, "searchRota: startDate and endDate are both needed"],
    [{ startDate: "2026-11-16", endDate: "2026-11-22", team: "rigel" }, 'searchRota: unknown argument "team"'],
    [{ startDate: "16/11/2026", endDate: "2026-11-22" }, "searchRota: startDate must be a date (YYYY-MM-DD) or a date token"],
    [{ startDate: "2026-11-16", endDate: 20261122 }, "searchRota: endDate must be a date (YYYY-MM-DD) or a date token"],
    [{ startDate: "2026-11-16", endDate: "2026-11-15" }, "searchRota: endDate must not be before startDate"],
    [{ startDate: "2026-11-01", endDate: "2027-01-11" }, "searchRota: the window must be at most 70 days"],
    [{ startDate: "2026-11-16", endDate: "2026-11-22", teams: [] }, "searchRota: teams must be a non-empty list of team keys"],
    [{ startDate: "2026-11-16", endDate: "2026-11-22", teams: "rigel" }, "teams must be a non-empty list"],
    [{ startDate: "2026-11-16", endDate: "2026-11-22", teams: ["rigel", " "] }, "teams must be a non-empty list"],
    [{ startDate: "2026-11-16", endDate: "2026-11-22", mine: "yes" }, "searchRota: mine must be true or false"],
  ])("refuses %j", (args, message) => {
    expect(() => buildScheduleSearchRequest("searchRota", args, ME)).toThrow(message);
  });

  it("needs the signed-in user for mine", () => {
    expect(() => buildScheduleSearchRequest("searchAbsences", { startDate: "2026-11-16", endDate: "2026-11-22", mine: true }, undefined)).toThrow(
      "searchAbsences: the signed-in user is not loaded",
    );
  });
});

describe("toSearchRotaResult", () => {
  it("passes each shift on with the engineer by name, no email, id or note", () => {
    expect(
      toSearchRotaResult({
        count: 9,
        assignments: [
          {
            id: "a1",
            engineer: { userId: "u1", name: "Phoenix Engineer 03", email: "p3@example.com", isLead: true },
            teamKey: "phoenix",
            shiftCode: "CRE_MORNING_OC",
            rotaDate: "2026-11-16",
            startsAt: "2026-11-16T00:30:00Z",
            endsAt: "2026-11-16T03:30:00Z",
            isOnCall: true,
            source: "GENERATED",
            note: "swap with Atlas",
          } as never,
        ],
      }),
    ).toEqual({
      total: 9,
      items: [
        {
          date: "2026-11-16",
          engineer: "Phoenix Engineer 03",
          lead: true,
          team: "phoenix",
          shift: "CRE_MORNING_OC",
          startsAt: "2026-11-16T00:30:00Z",
          endsAt: "2026-11-16T03:30:00Z",
          onCall: true,
        },
      ],
    });
  });

  it("keeps the lead and on-call flags apart", () => {
    const [shift] = toSearchRotaResult({ assignments: [{ engineer: { name: "Rigel Lead", isLead: true }, isOnCall: false }] }).items;
    expect(shift).toMatchObject({ lead: true, onCall: false });
  });

  it("reads missing values as null or false, and an empty response as empty", () => {
    expect(toSearchRotaResult({ assignments: [{}] })).toEqual({
      total: 1,
      items: [{ date: null, engineer: null, lead: false, team: null, shift: null, startsAt: null, endsAt: null, onCall: false }],
    });
    expect(toSearchRotaResult({})).toEqual({ total: 0, items: [] });
  });
});

describe("toSearchAbsencesResult", () => {
  it("passes each absence on with its kind, the engineer by name, no email, id or note", () => {
    expect(
      toSearchAbsencesResult({
        count: 4,
        absences: [
          {
            id: "b1",
            engineer: { userId: "u2", name: "Americas Engineer 05", email: "a5@example.com", isLead: false },
            teamKey: "americas",
            kindCode: "CUSTOMER_ONSITE",
            startsOn: "2026-11-10",
            endsOn: "2026-11-20",
            allocatedTo: "Example Corp",
            note: "standing allocation",
            homeTeamKey: "rigel",
          } as never,
        ],
      }),
    ).toEqual({
      total: 4,
      items: [
        {
          engineer: "Americas Engineer 05",
          team: "americas",
          kind: "CUSTOMER_ONSITE",
          startsOn: "2026-11-10",
          endsOn: "2026-11-20",
          allocatedTo: "Example Corp",
        },
      ],
    });
  });

  it("reads an open-ended absence's end, and an empty response, as null / empty", () => {
    expect(toSearchAbsencesResult({ absences: [{ kindCode: "RND", startsOn: "2026-09-07" }] })).toEqual({
      total: 1,
      items: [{ engineer: null, team: null, kind: "RND", startsOn: "2026-09-07", endsOn: null, allocatedTo: null }],
    });
    expect(toSearchAbsencesResult({})).toEqual({ total: 0, items: [] });
  });
});
