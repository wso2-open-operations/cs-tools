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

import { describe, expect, it } from "vitest";
import {
  buildCountOperationsRequest,
  buildSearchOperationsRequest,
  toCountOperationsResult,
  toSearchOperationsResult,
} from "@features/csm-custom-dashboards/api/codeDashboardOperations";

const me = "user-1";
const openIncidents = [{ field: "state", op: "in", values: ["NEW", "IN_PROGRESS", "ON_HOLD"] }];

describe("buildCountOperationsRequest", () => {
  it("counts each kind on its own endpoint, by state when nothing is asked", () => {
    expect(buildCountOperationsRequest({ kind: "incident" }, me)).toEqual({
      path: "/incidents/aggregate",
      body: { filters: {}, groupBy: "state" },
      grouped: false,
    });
    expect(buildCountOperationsRequest({ kind: "change_request" }, me).path).toBe("/change-requests/aggregate");
    expect(buildCountOperationsRequest({ kind: "problem" }, me).path).toBe("/problems/aggregate");
  });

  it("passes the kind's lists and filters through, and mine as the viewer's own id", () => {
    expect(
      buildCountOperationsRequest(
        {
          kind: "incident",
          priorities: ["CRITICAL", "HIGH"],
          filters: [...openIncidents, { field: "createdOn", op: "gte", values: ["__startOfMonth:0__"] }],
          mine: true,
          groupBy: "state",
          maxGroups: 8,
        },
        me,
      ).body,
    ).toEqual({
      filters: {
        priorities: ["CRITICAL", "HIGH"],
        filters: [
          ...openIncidents,
          { field: "createdOn", op: "gte", values: ["__startOfMonth:0__"] },
          { field: "assignedUserId", op: "in", values: [me] },
        ],
      },
      groupBy: "state",
      maxGroups: 8,
    });
    expect(
      buildCountOperationsRequest(
        { kind: "change_request", states: ["new", "assess"], impacts: ["high"], filters: [{ field: "approval", op: "eq", values: ["requested"] }] },
        me,
      ).body.filters,
    ).toEqual({ states: ["new", "assess"], impacts: ["high"], filters: [{ field: "approval", op: "eq", values: ["requested"] }] });
  });

  it.each([
    [{}, /kind must be one of incident, change_request, problem/],
    [{ kind: "case" }, /kind must be one of/],
    [{ kind: "incident", states: ["new"] }, /incident takes no "states"/],
    [{ kind: "problem", priorities: ["HIGH"] }, /problem takes no "priorities"/],
    [{ kind: "incident", priorities: ["P1"] }, /priorities must be a non-empty list of CRITICAL/],
    [{ kind: "change_request", states: ["NEW"] }, /states must be a non-empty list of new/],
    [{ kind: "change_request", impacts: [] }, /impacts must be a non-empty list/],
    [{ kind: "incident", filters: { field: "state" } }, /filters must be an array/],
    [{ kind: "incident", filters: [{ field: "state", op: "in", values: ["new"] }] }, /state filter's values must be from NEW/],
    [{ kind: "incident", filters: [{ field: "priority", op: "in", values: ["HIGH"] }] }, /field must be one of state, createdOn, slaViolated, madeSla/],
    [{ kind: "change_request", filters: [{ field: "state", op: "in", values: ["new"] }] }, /field must be one of approval, createdOn/],
    [{ kind: "problem", filters: [{ field: "state", op: "notIn", values: ["NEW"] }] }, /state filter's op must be in/],
    [{ kind: "incident", filters: [{ field: "slaViolated", op: "eq", values: ["true", "false"] }] }, /values must be one of true, false/],
    [{ kind: "incident", filters: [{ field: "createdOn", op: "gte", values: [] }] }, /takes one date/],
    [{ kind: "incident", filters: [{ field: "assignedUserId", op: "in", values: ["someone"] }] }, /field must be one of/],
    [{ kind: "incident", mine: "yes" }, /mine must be true or false/],
    [{ kind: "incident", groupBy: "priority" }, /groupBy must be "state"/],
    [{ kind: "incident", maxGroups: 5 }, /maxGroups must be a positive integer, with groupBy/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountOperationsRequest(args, me)).toThrow(message);
  });

  it("refuses mine before the viewer is loaded", () => {
    expect(() => buildCountOperationsRequest({ kind: "problem", mine: true }, undefined)).toThrow(/not loaded/);
  });
});

describe("toCountOperationsResult", () => {
  it("keeps the groups, adds Others, and reads no groups as none", () => {
    expect(
      toCountOperationsResult({ totalRecords: 3, groups: [{ key: "on_hold", label: "on_hold", count: 2 }], othersCount: 1 }, true),
    ).toEqual({ total: 3, groups: [{ label: "on_hold", count: 2 }, { label: "Others", count: 1 }] });
    expect(toCountOperationsResult({ totalRecords: 0, groups: null }, true)).toEqual({ total: 0, groups: [] });
    expect(toCountOperationsResult({ totalRecords: 3 }, false)).toEqual({ total: 3 });
  });
});

describe("buildSearchOperationsRequest", () => {
  it("reads the kind's first page, with its own sort", () => {
    expect(buildSearchOperationsRequest({ kind: "incident", sortBy: { field: "openedOn" }, limit: 5 }, me)).toEqual({
      path: "/incidents/search",
      itemsKey: "incidents",
      body: { filters: {}, sortBy: { field: "openedOn", order: "desc" }, pagination: { offset: 0, limit: 5 } },
    });
    expect(buildSearchOperationsRequest({ kind: "problem" }, me)).toEqual({
      path: "/problems/search",
      itemsKey: "problems",
      body: { filters: {}, pagination: { offset: 0, limit: 10 } },
    });
  });

  it.each([
    [{ kind: "problem", sortBy: { field: "createdOn" } }, /a problem list takes no sortBy/],
    [{ kind: "change_request", sortBy: { field: "openedOn" } }, /sortBy.field must be one of createdOn, updatedOn/],
    [{ kind: "incident", sortBy: { field: "createdOn", order: "newest" } }, /sortBy.order must be/],
    [{ kind: "incident", limit: 51 }, /limit must be an integer from 1 to 50/],
    [{ kind: "incident", groupBy: "state" }, /incident takes no "groupBy"/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchOperationsRequest(args, me)).toThrow(message);
  });
});

describe("toSearchOperationsResult", () => {
  it("keeps plain values and the assignee's name, from either field", () => {
    expect(
      toSearchOperationsResult(
        {
          total: 11,
          changeRequests: [
            { id: "c1", number: "CHG-1", subject: "Patch", state: "new", impact: "low", assignedEngineer: { name: "Nico Rivera" } },
          ],
        },
        "changeRequests",
      ),
    ).toEqual({
      total: 11,
      items: [
        {
          id: "c1",
          number: "CHG-1",
          subject: "Patch",
          state: "new",
          priority: null,
          impact: "low",
          createdOn: null,
          updatedOn: null,
          openedOn: null,
          assignee: "Nico Rivera",
        },
      ],
    });
    expect(
      toSearchOperationsResult({ incidents: [{ id: "i1", priority: "CRITICAL", assignedTo: { name: "Avery Patel" } }] }, "incidents").items[0],
    ).toMatchObject({ priority: "CRITICAL", assignee: "Avery Patel" });
    expect(toSearchOperationsResult({ problems: null }, "problems")).toEqual({ total: 0, items: [] });
  });
});
