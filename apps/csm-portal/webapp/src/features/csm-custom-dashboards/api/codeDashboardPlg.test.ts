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
  buildCountPlgRequest,
  buildPlgSummaryPath,
  buildPlgTrendRequest,
  buildSearchPlgRequest,
  createPlaybookCodeChecker,
  toCountPlgResult,
  toPlgSummaryResult,
  toPlgTrendResult,
  toSearchPlgResult,
} from "@features/csm-custom-dashboards/api/codeDashboardPlg";

const me = "user-1";

describe("buildCountPlgRequest", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("counts with one record of the kind's search when nothing is grouped", () => {
    expect(buildCountPlgRequest({ kind: "organization", productCodes: ["IAM"], subscriptionTiers: ["TRIAL", "TRIAL_EXTENDED"] }, me)).toEqual({
      path: "/plg/organizations/search",
      body: { filters: { productCodes: ["IAM"], subscriptionTiers: ["TRIAL", "TRIAL_EXTENDED"] }, pagination: { offset: 0, limit: 1 } },
      grouped: false,
    });
  });

  it("groups on the kind's aggregate", () => {
    expect(buildCountPlgRequest({ kind: "organization", groupBy: "owner", maxGroups: 10 }, me)).toEqual({
      path: "/plg/organizations/aggregate",
      body: { filters: {}, groupBy: "owner", maxGroups: 10 },
      grouped: true,
    });
    expect(buildCountPlgRequest({ kind: "registration", groupBy: "country", unacknowledgedOnly: true }, me)).toEqual({
      path: "/plg/registrations/aggregate",
      body: { filters: {}, unacknowledgedOnly: true, groupBy: "country" },
      grouped: true,
    });
  });

  it("turns mine, owners, countries and flags into the PLG filters", () => {
    expect(
      buildCountPlgRequest(
        { kind: "registration", mine: true, ownerEmails: ["person@example.com"], countryNames: ["Sri Lanka"], lifecycleStages: ["ACTIVATED"] },
        me,
      ).body.filters,
    ).toEqual({ lifecycleStages: ["ACTIVATED"], ownerIds: [me], ownerEmails: ["person@example.com"], countryNames: ["Sri Lanka"] });
    expect(buildCountPlgRequest({ kind: "organization", unowned: true, countryUnknown: true }, me).body.filters).toEqual({
      unowned: true,
      countryUnknown: true,
    });
  });

  it("sends a playbook task's answers, its unanswered ones, or a grouping by answer", () => {
    expect(buildCountPlgRequest({ kind: "organization", playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED", "MONITOR"] }, me).body.filters).toEqual({
      playbookTask: "OUTCOME",
      playbookAnswers: ["EXCLUDED", "MONITOR"],
    });
    expect(buildCountPlgRequest({ kind: "registration", playbookTask: "OUTCOME", playbookUnanswered: true }, me).body.filters).toEqual({
      playbookTask: "OUTCOME",
      playbookUnanswered: true,
    });
    expect(buildCountPlgRequest({ kind: "registration", playbookTask: "OUTCOME", groupBy: "playbookAnswer", maxGroups: 10 }, me)).toEqual({
      path: "/plg/registrations/aggregate",
      body: { filters: { playbookTask: "OUTCOME" }, groupBy: "playbookAnswer", maxGroups: 10 },
      grouped: true,
    });
    expect(buildPlgTrendRequest({ playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED"], days: 90 }, me)).toEqual({
      filters: { playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED"] },
      days: 90,
    });
  });

  it("sends the sign-up dates as the UTC start and end of those days, tokens read as UTC days", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(Date.UTC(2026, 10, 12, 1, 0)));
    expect(buildCountPlgRequest({ kind: "registration", registeredFrom: "__startOfMonth:0__", registeredTo: "2026-11-12" }, me).body.filters).toEqual({
      registeredFrom: "2026-11-01T00:00:00Z",
      registeredTo: "2026-11-12T23:59:59.999999Z",
    });
  });

  it.each([
    [{}, /kind must be "organization" or "registration"/],
    [{ kind: "account" }, /kind must be/],
    [{ kind: "organization", unacknowledgedOnly: true }, /an? organization call takes no "unacknowledgedOnly"/],
    [{ kind: "organization", ownerIds: ["someone"] }, /takes no "ownerIds"/],
    [{ kind: "organization", productCodes: ["Choreo"] }, /productCodes must be a non-empty list of IAM/],
    [{ kind: "organization", lifecycleStages: ["activated"] }, /lifecycleStages must be a non-empty list/],
    [{ kind: "organization", subscriptionTiers: [] }, /subscriptionTiers must be a non-empty list/],
    [{ kind: "registration", registeredFrom: "November" }, /registeredFrom must be a date/],
    [{ kind: "organization", unowned: "yes" }, /unowned must be true or false/],
    [{ kind: "organization", ownerEmails: ["Alex"] }, /ownerEmails must be a non-empty list of email addresses/],
    [{ kind: "organization", unowned: true, mine: true }, /unowned cannot go with mine or ownerEmails/],
    [{ kind: "organization", countryNames: [""] }, /countryNames must be a non-empty list of country names/],
    [{ kind: "organization", countryNames: ["India"], countryUnknown: true }, /countryUnknown cannot go with countryNames/],
    [{ kind: "registration", groupBy: "owner" }, /a registration groupBy must be "country" or "playbookAnswer"/],
    [{ kind: "organization", groupBy: "playbookAnswer", playbookTask: "OUTCOME" }, /an? organization groupBy must be "owner" or "country"/],
    [{ kind: "registration", groupBy: "playbookAnswer" }, /groupBy playbookAnswer needs a playbookTask/],
    [{ kind: "registration", playbookAnswers: ["EXCLUDED"] }, /need a playbookTask/],
    [{ kind: "registration", playbookUnanswered: true }, /need a playbookTask/],
    [{ kind: "registration", playbookTask: ["OUTCOME"] }, /playbookTask must be one task code/],
    [{ kind: "registration", playbookTask: "OUTCOME", playbookAnswers: [] }, /playbookAnswers must be a non-empty list of answer codes/],
    [{ kind: "registration", playbookTask: "OUTCOME", playbookUnanswered: "yes" }, /playbookUnanswered must be true or false/],
    [
      { kind: "registration", playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED"], playbookUnanswered: true },
      /playbookUnanswered cannot go with playbookAnswers/,
    ],
    [{ kind: "organization", groupBy: "product" }, /an? organization groupBy must be "owner" or "country"/],
    [{ kind: "organization", maxGroups: 5 }, /maxGroups goes only with groupBy/],
    [{ kind: "organization", groupBy: "owner", maxGroups: 0 }, /maxGroups must be a positive integer/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountPlgRequest(args, me)).toThrow(message);
  });

  it("refuses mine before the viewer is loaded", () => {
    expect(() => buildCountPlgRequest({ kind: "organization", mine: true }, undefined)).toThrow(/not loaded/);
  });
});

describe("toCountPlgResult", () => {
  it("reads a search's total, or an aggregate's groups with Others", () => {
    expect(toCountPlgResult({ total: 8 }, false)).toEqual({ total: 8 });
    expect(
      toCountPlgResult({ totalRecords: 8, groups: [{ label: "Sri Lanka", count: 4 }, { label: "Unknown", count: 1 }], othersCount: 3 }, true),
    ).toEqual({ total: 8, groups: [{ label: "Sri Lanka", count: 4 }, { label: "Unknown", count: 1 }, { label: "Others", count: 3 }] });
    expect(toCountPlgResult({ totalRecords: 0, groups: null }, true)).toEqual({ total: 0, groups: [] });
  });
});

describe("buildSearchPlgRequest", () => {
  it("lists organisations newest first by default, or by name", () => {
    expect(buildSearchPlgRequest({ kind: "organization", sortBy: { field: "name" } }, me)).toEqual({
      path: "/plg/organizations/search",
      itemsKey: "organizations",
      body: { filters: {}, pagination: { offset: 0, limit: 10 }, sortBy: "name", sortOrder: "asc" },
    });
    expect(buildSearchPlgRequest({ kind: "organization", sortBy: { field: "registeredOn" }, limit: 5 }, me).body).toMatchObject({
      sortBy: "registeredOn",
      sortOrder: "desc",
      pagination: { offset: 0, limit: 5 },
    });
  });

  it.each([
    [{ kind: "registration", sortBy: { field: "registeredOn" } }, /a registration list takes no sortBy/],
    [{ kind: "organization", sortBy: { field: "owner" } }, /sortBy.field must be "name" or "registeredOn"/],
    [{ kind: "organization", limit: 51 }, /limit must be an integer from 1 to 50/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchPlgRequest(args, me)).toThrow(message);
  });
});

describe("toSearchPlgResult", () => {
  it("keeps the organisation, date, owner and products, never the sign-up email or name", () => {
    const organization = {
      id: "o1",
      organizationName: "Harbor Media",
      registeredEmail: "hal@harbormedia.example.test",
      registeredName: "Harbor Test",
      registeredOn: "2026-11-03T09:00:00Z",
      platformStages: [{ product: { code: "API_PLATFORM", name: "API Platform" }, lifecycleStage: "REGISTRATION", stageName: "Registration" }],
      owner: { name: "Jane Doe", email: "jane.doe@example.com" },
    };
    expect(toSearchPlgResult({ total: 8, organizations: [organization] }, "organizations")).toEqual({
      total: 8,
      items: [
        {
          id: "o1",
          organization: "Harbor Media",
          registeredOn: "2026-11-03T09:00:00Z",
          owner: "Jane Doe",
          products: [{ code: "API_PLATFORM", name: "API Platform", stage: "Registration" }],
        },
      ],
    });
    const registration = {
      orgPlatformId: "r1",
      organizationName: "Harbor Media",
      registeredEmail: "hal@harbormedia.example.test",
      product: { code: "API_PLATFORM", name: "API Platform" },
      lifecycleStage: "REGISTRATION",
      registeredOn: "2026-11-03T09:00:00Z",
      owner: null,
    };
    expect(toSearchPlgResult({ registrations: [registration] }, "registrations").items).toEqual([
      {
        id: "r1",
        organization: "Harbor Media",
        registeredOn: "2026-11-03T09:00:00Z",
        owner: null,
        products: [{ code: "API_PLATFORM", name: "API Platform", stage: "REGISTRATION" }],
      },
    ]);
  });
});

describe("buildPlgTrendRequest", () => {
  it("sends the registration filters beside the window", () => {
    expect(buildPlgTrendRequest({ productCodes: ["IAM"], startDate: "2026-01-01", endDate: "2026-11-12", bucket: "month" }, me)).toEqual({
      filters: { productCodes: ["IAM"] },
      startDate: "2026-01-01",
      endDate: "2026-11-12",
      bucket: "month",
    });
    expect(buildPlgTrendRequest({ unacknowledgedOnly: true }, me)).toEqual({ filters: {}, unacknowledgedOnly: true, days: 30 });
  });

  it.each([
    [{ days: 91 }, /plgTrend: days must be an integer from 1 to 90/],
    [{ days: 30, startDate: "2026-10-01", endDate: "2026-10-31" }, /plgTrend: use days or startDate and endDate, not both/],
    [{ groupBy: "country" }, /takes no "groupBy"/],
  ])("refuses %j", (args, message) => {
    expect(() => buildPlgTrendRequest(args, me)).toThrow(message);
  });
});

describe("toPlgTrendResult", () => {
  it("gives one point per period, and none without a series", () => {
    expect(
      toPlgTrendResult({
        series: [
          {
            label: "Registered",
            points: [
              { date: "2026-09-01", endDate: "2026-09-30", count: 1 },
              { date: "2026-10-01", endDate: "2026-10-31", count: 2 },
            ],
          },
        ],
      }),
    ).toEqual({
      points: [
        { date: "2026-09-01", endDate: "2026-09-30", count: 1 },
        { date: "2026-10-01", endDate: "2026-10-31", count: 2 },
      ],
    });
    expect(toPlgTrendResult({ series: null })).toEqual({ points: [] });
  });
});

describe("createPlaybookCodeChecker", () => {
  it("checks a task and its answers once, and nothing without a task", async () => {
    const post = vi.fn().mockResolvedValue({});
    const check = createPlaybookCodeChecker(post);
    await check({ playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED"] });
    await check({ playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED"] });
    await check({ playbookTask: "OUTCOME" });
    await check({ productCodes: ["IAM"] });
    expect(post.mock.calls).toEqual([[{ playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED"] }], [{ playbookTask: "OUTCOME" }]]);
  });

  it("passes a failed check's reason on and does not keep it", async () => {
    const post = vi.fn().mockRejectedValueOnce(new Error('Unknown playbook task "OUTCOME"')).mockResolvedValueOnce({});
    const check = createPlaybookCodeChecker(post);
    await expect(check({ playbookTask: "OUTCOME" })).rejects.toThrow(/Unknown playbook task/);
    await expect(check({ playbookTask: "OUTCOME" })).resolves.toBeUndefined();
    expect(post).toHaveBeenCalledTimes(2);
  });
});

describe("buildPlgSummaryPath", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("reads the endpoint's own window when no date is given", () => {
    expect(buildPlgSummaryPath(undefined)).toBe("/plg/analytics/dashboard");
    expect(buildPlgSummaryPath({})).toBe("/plg/analytics/dashboard");
  });

  it("sends the dates as days, the end as the day after it, tokens read as UTC days", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(Date.UTC(2026, 10, 12, 1, 0)));
    expect(buildPlgSummaryPath({ startDate: "__startOfMonth:0__", endDate: "__today__" })).toBe(
      "/plg/analytics/dashboard?from=2026-11-01&to=2026-11-13",
    );
  });

  it("takes one side alone", () => {
    expect(buildPlgSummaryPath({ startDate: "2026-10-01" })).toBe("/plg/analytics/dashboard?from=2026-10-01");
    expect(buildPlgSummaryPath({ endDate: "2026-10-31" })).toBe("/plg/analytics/dashboard?to=2026-11-01");
  });

  it("takes the same day as both ends, and ends a year on the next one", () => {
    expect(buildPlgSummaryPath({ startDate: "2026-10-01", endDate: "2026-10-01" })).toBe(
      "/plg/analytics/dashboard?from=2026-10-01&to=2026-10-02",
    );
    expect(buildPlgSummaryPath({ endDate: "2026-12-31" })).toBe("/plg/analytics/dashboard?to=2027-01-01");
  });

  it.each([
    [{ days: 30 }, 'plgSummary: unknown argument "days"'],
    [{ startDate: "2026/10/01" }, "plgSummary: startDate must be a date (YYYY-MM-DD) or a date token"],
    [{ endDate: 20261001 }, "plgSummary: endDate must be a date (YYYY-MM-DD) or a date token"],
    [{ startDate: "2026-11-02", endDate: "2026-11-01" }, "plgSummary: endDate must not be before startDate"],
  ])("refuses %j", (args, message) => {
    expect(() => buildPlgSummaryPath(args)).toThrow(message);
  });
});

describe("toPlgSummaryResult", () => {
  it("names each figure and keeps each breakdown", () => {
    expect(
      toPlgSummaryResult({
        summary: {
          totalOrganizations: 5,
          totalRegistrations: 6,
          newRegistrations: 7,
          pairingsNeedingAttention: 1,
          trialsEndingSoon: 2,
          unresolvedIngestFailures: 3,
        },
        registrationsByProduct: [{ label: "API Platform", count: 2 }],
        registrationsByLifecycleStage: [{ label: "REGISTRATION", count: 3 }],
        subscriptionMix: [{ label: "NOT_RECORDED", count: 1 }],
        registrationsOverTime: { buckets: ["2026-10-19", "2026-11-02"], series: [{ label: "API Platform", counts: [1, 0] }] },
      }),
    ).toEqual({
      organizations: 5,
      registrations: 6,
      unacknowledged: 7,
      needingAttention: 1,
      trialsEndingSoon: 2,
      ingestFailures: 3,
      byProduct: [{ label: "API Platform", count: 2 }],
      byStage: [{ label: "REGISTRATION", count: 3 }],
      byTier: [{ label: "NOT_RECORDED", count: 1 }],
      weekly: { weeks: ["2026-10-19", "2026-11-02"], series: [{ label: "API Platform", counts: [1, 0] }] },
    });
  });

  it("reads an empty response as zeros and empty lists", () => {
    expect(toPlgSummaryResult({})).toEqual({
      organizations: 0,
      registrations: 0,
      unacknowledged: 0,
      needingAttention: 0,
      trialsEndingSoon: 0,
      ingestFailures: 0,
      byProduct: [],
      byStage: [],
      byTier: [],
      weekly: { weeks: [], series: [] },
    });
  });
});
