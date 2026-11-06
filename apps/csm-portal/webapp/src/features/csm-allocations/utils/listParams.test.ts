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
import { buildSearchPayload, readListParams, writeListParams } from "./listParams";

const read = (qs: string) => readListParams(new URLSearchParams(qs));

describe("list params", () => {
  it("defaults to newest start date first, page 0, 20 rows", () => {
    const p = read("");
    expect(p).toMatchObject({ sort: "startDate", order: "desc", page: 0, rows: 20, states: [] });
    expect(writeListParams(p).toString()).toBe("");
  });

  it("round-trips filters, sort and paging through the URL", () => {
    const qs = "q=abc&account=a-1&accountLabel=Example+Corp&engineer=u-1&engineerLabel=Jane+Doe&state=CONFIRMED&state=TENTATIVE&from=2026-09-01&to=2026-09-30&sort=utilization&order=asc&page=2&rows=50";
    expect(new URLSearchParams(writeListParams(read(qs))).toString()).toBe(qs);
  });

  it("drops unknown states, malformed dates, unknown sort fields and odd paging", () => {
    const p = read("state=NOPE&state=CONFIRMED&from=yesterday&sort=bogus&order=sideways&page=-3&rows=7");
    expect(p).toMatchObject({ states: ["CONFIRMED"], from: "", sort: "startDate", order: "desc", page: 0, rows: 20 });
  });

  it("builds the request from the params, leaving empty filters out", () => {
    expect(buildSearchPayload(read("page=2&rows=10"))).toEqual({
      pagination: { limit: 10, offset: 20 },
      filters: {},
      sortBy: { field: "startDate", order: "desc" },
    });
    expect(
      buildSearchPayload(read("q=%20abc%20&account=a-1&engineer=u-1&state=CONFIRMED&from=2026-09-01&to=2026-09-30&sort=state&order=asc")),
    ).toEqual({
      pagination: { limit: 20, offset: 0 },
      filters: {
        searchQuery: "abc",
        accountId: "a-1",
        engineerId: "u-1",
        states: ["CONFIRMED"],
        from: "2026-09-01",
        to: "2026-09-30",
      },
      sortBy: { field: "state", order: "asc" },
    });
  });

  it("does not send a backwards date range", () => {
    const f = buildSearchPayload(read("from=2026-10-01&to=2026-09-01")).filters;
    expect(f.from).toBeUndefined();
    expect(f.to).toBeUndefined();
  });
});
