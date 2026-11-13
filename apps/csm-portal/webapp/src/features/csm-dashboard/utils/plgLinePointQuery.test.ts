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
import { plgLinePointFilters } from "@features/csm-dashboard/utils/plgLinePointQuery";

describe("plgLinePointFilters", () => {
  const now = new Date(2026, 2, 15, 12, 0, 0);

  it("puts a day point's day on the widget's own filters", () => {
    expect(plgLinePointFilters({ productCodes: ["IAM"], unacknowledgedOnly: true }, "2026-03-02", undefined, now)).toEqual({
      productCodes: ["IAM"],
      unacknowledgedOnly: true,
      registeredFrom: "2026-03-02",
      registeredTo: "2026-03-02",
    });
  });

  it("covers a week or month point from its first day to its last", () => {
    expect(plgLinePointFilters({}, "2026-02-01", "2026-02-28", now)).toEqual({
      registeredFrom: "2026-02-01",
      registeredTo: "2026-02-28",
    });
  });

  it("keeps the widget's own bound where it is tighter, and the point's where it is not", () => {
    expect(
      plgLinePointFilters({ registeredFrom: "2026-02-10", registeredTo: "2026-02-20" }, "2026-02-01", "2026-02-28", now),
    ).toEqual({ registeredFrom: "2026-02-10", registeredTo: "2026-02-20" });
    expect(
      plgLinePointFilters({ registeredFrom: "2026-01-10", registeredTo: "2026-03-20" }, "2026-02-01", "2026-02-28", now),
    ).toEqual({ registeredFrom: "2026-02-01", registeredTo: "2026-02-28" });
  });

  it("resolves the widget's relative bounds before comparing", () => {
    // Last month starts 2026-02-01; this year's January point starts earlier.
    expect(plgLinePointFilters({ registeredFrom: "__startOfMonth:-1__" }, "2026-01-01", "2026-01-31", now)).toEqual({
      registeredFrom: "2026-02-01",
      registeredTo: "2026-01-31",
    });
  });

  it("leaves the point's bound for a widget bound that is not a plain date", () => {
    expect(plgLinePointFilters({ registeredTo: "2026-02-20T10:00:00Z" }, "2026-02-01", "2026-02-28", now)).toEqual({
      registeredFrom: "2026-02-01",
      registeredTo: "2026-02-28",
    });
  });
});
