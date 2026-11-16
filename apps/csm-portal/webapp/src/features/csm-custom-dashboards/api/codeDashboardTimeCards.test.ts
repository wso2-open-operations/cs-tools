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
  buildCountTimeCardsRequest,
  buildSearchTimeCardsRequest,
  createTeamListResolver,
  toCountTimeCardsResult,
  toSearchTimeCardsResult,
} from "@features/csm-custom-dashboards/api/codeDashboardTimeCards";

const me = "user-1";
const october = { startDate: "2026-10-01", endDate: "2026-10-31" };

describe("buildCountTimeCardsRequest", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("counts every card by state when nothing is asked", () => {
    expect(buildCountTimeCardsRequest(undefined, me)).toEqual({ body: { filters: {}, groupBy: "state" }, grouped: false });
  });

  it("turns mine into the viewer's own id and hours into minutes", () => {
    expect(
      buildCountTimeCardsRequest({ mine: true, states: ["submitted"], groupBy: "project", measure: "hours", maxGroups: 5 }, me),
    ).toEqual({
      body: { filters: { states: ["submitted"], userIds: [me] }, groupBy: "project", measure: "minutes", maxGroups: 5 },
      grouped: true,
    });
    expect(buildCountTimeCardsRequest({ mine: false, groupBy: "user", measure: "count" }, me).body).toEqual({
      filters: {},
      groupBy: "user",
      measure: "count",
    });
  });

  it("resolves date tokens to the dates the endpoint takes", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 10, 12, 12));
    expect(buildCountTimeCardsRequest({ startDate: "__startOfMonth:0__", endDate: "__today__" }, me).body.filters).toEqual({
      startDate: "2026-11-01",
      endDate: "2026-11-12",
    });
  });

  it("sends teams as teamIds, for a team grouping or any other", () => {
    expect(buildCountTimeCardsRequest({ teams: ["rigel", "sirius"], groupBy: "team", measure: "hours" }, me).body).toEqual({
      filters: { teamIds: ["rigel", "sirius"] },
      groupBy: "team",
      measure: "minutes",
    });
    expect(buildSearchTimeCardsRequest({ teams: ["rigel"] }, me).filters).toEqual({ teamIds: ["rigel"] });
  });

  it("takes a day grouping over a window of up to 31 days", () => {
    expect(buildCountTimeCardsRequest({ ...october, groupBy: "day", measure: "hours" }, me).body).toEqual({
      filters: october,
      groupBy: "day",
      measure: "minutes",
    });
  });

  it.each([
    [{ filters: [] }, /unknown argument "filters"/],
    [{ userIds: ["someone"] }, /unknown argument "userIds"/],
    [{ states: [] }, /states must be a non-empty list/],
    [{ states: ["closed"] }, /states must be a non-empty list/],
    [{ mine: "yes" }, /mine must be true or false/],
    [{ startDate: "October" }, /startDate must be a date/],
    [{ endDate: "__daysAgo:x__" }, /endDate must be a date/],
    [{ groupBy: "team" }, /groupBy team needs teams/],
    [{ groupBy: "ABT" }, /groupBy must be one of state, user, project, day, team/],
    [{ teams: [] }, /teams must be a list of 1 to 100 team keys/],
    [{ teams: "rigel" }, /teams must be a list of 1 to 100 team keys/],
    [{ teams: ["rigel", ""] }, /teams must be a list of 1 to 100 team keys/],
    [{ teams: Array.from({ length: 101 }, (_, i) => `t${i}`) }, /teams must be a list of 1 to 100 team keys/],
    [{ groupBy: "user", measure: "minutes" }, /measure must be "count" or "hours"/],
    [{ maxGroups: 5 }, /maxGroups goes only with a groupBy other than day/],
    [{ ...october, groupBy: "day", maxGroups: 5 }, /maxGroups goes only with a groupBy other than day/],
    [{ groupBy: "user", maxGroups: 0 }, /maxGroups must be a positive integer/],
    [{ groupBy: "day", startDate: "2026-10-01" }, /needs both startDate and endDate/],
    [{ groupBy: "day", startDate: "2026-09-01", endDate: "2026-10-31" }, /window of 1 to 31 days/],
    [{ groupBy: "day", startDate: "2026-10-31", endDate: "2026-10-01" }, /window of 1 to 31 days/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountTimeCardsRequest(args, me)).toThrow(message);
  });

  it("refuses mine before the viewer is loaded", () => {
    expect(() => buildCountTimeCardsRequest({ mine: true }, undefined)).toThrow(/signed-in user is not loaded/);
  });
});

describe("toCountTimeCardsResult", () => {
  const response = {
    groups: [
      { key: "p1", label: "Example Corp Production", count: 5, totalMinutes: 420 },
      { key: "p2", label: "Summit Enterprises Production", count: 2, totalMinutes: 159 },
    ],
    othersCount: 1,
    othersMinutes: 94,
    totalRecords: 8,
    totalMinutes: 673,
  };

  it("gives cards and hours per group, with an Others group", () => {
    expect(toCountTimeCardsResult(response, true)).toEqual({
      total: 8,
      hours: 11.22,
      groups: [
        { label: "Example Corp Production", count: 5, hours: 7 },
        { label: "Summit Enterprises Production", count: 2, hours: 2.65 },
        { label: "Others", count: 1, hours: 1.57 },
      ],
    });
  });

  it("gives only the totals for an ungrouped count", () => {
    expect(toCountTimeCardsResult(response, false)).toEqual({ total: 8, hours: 11.22 });
    expect(toCountTimeCardsResult({}, false)).toEqual({ total: 0, hours: 0 });
  });
});

describe("buildSearchTimeCardsRequest", () => {
  it("reads the first 10 cards by default", () => {
    expect(buildSearchTimeCardsRequest(undefined, me)).toEqual({ filters: {}, pagination: { offset: 0, limit: 10 } });
  });

  it("passes the filters, a sort and a limit through", () => {
    expect(buildSearchTimeCardsRequest({ mine: true, ...october, sortBy: { field: "workDate" }, limit: 50 }, me)).toEqual({
      filters: { userIds: [me], ...october },
      sortBy: { field: "workDate", order: "desc" },
      pagination: { offset: 0, limit: 50 },
    });
  });

  it.each([
    [{ groupBy: "user" }, /unknown argument "groupBy"/],
    [{ limit: 0 }, /limit must be an integer from 1 to 50/],
    [{ limit: 51 }, /limit must be an integer from 1 to 50/],
    [{ sortBy: { field: "createdOn" } }, /sortBy.field must be one of workDate, updatedOn/],
    [{ sortBy: { field: "workDate", order: "up" } }, /sortBy.order must be/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchTimeCardsRequest(args, me)).toThrow(message);
  });
});

describe("toSearchTimeCardsResult", () => {
  it("keeps plain values and names, the time in hours", () => {
    expect(
      toSearchTimeCardsResult({
        total: 23,
        timeCards: [
          {
            id: "t1",
            workDate: "2026-10-09",
            state: "processed",
            totalTime: 120,
            user: { name: "Sirius Engineer 02" },
            project: { name: "Example Corp Production" },
            case: { number: "CS0001" },
          },
          { id: "t2" },
        ],
      }),
    ).toEqual({
      total: 23,
      items: [
        {
          id: "t1",
          workDate: "2026-10-09",
          state: "processed",
          hours: 2,
          user: "Sirius Engineer 02",
          project: "Example Corp Production",
          caseNumber: "CS0001",
        },
        { id: "t2", workDate: null, state: null, hours: 0, user: null, project: null, caseNumber: null },
      ],
    });
  });
});

describe("createTeamListResolver", () => {
  it("looks a team list up once and returns the group ids", async () => {
    const post = vi.fn().mockResolvedValue({ teamIds: ["group-rigel"] });
    const resolve = createTeamListResolver(post);
    expect(await resolve(["rigel"])).toEqual(["group-rigel"]);
    expect(await resolve(["rigel"])).toEqual(["group-rigel"]);
    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith({ teamIds: ["rigel"] });
  });

  it("passes a failed lookup's error on and does not keep it", async () => {
    const post = vi
      .fn()
      .mockRejectedValueOnce(new Error('Unknown team "orion"'))
      .mockResolvedValueOnce({ teamIds: ["group-orion"] });
    const resolve = createTeamListResolver(post);
    await expect(resolve(["orion"])).rejects.toThrow(/orion/);
    expect(await resolve(["orion"])).toEqual(["group-orion"]);
    expect(post).toHaveBeenCalledTimes(2);
  });
});
