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

import { describe, expect, it } from "vitest";
import type { EngagementAllocation } from "../types";
import {
  allocationLabel,
  allocationTitle,
  allocationsByPersonAndDay,
  dismissedByPersonAndDay,
  chunk,
  distinctUserIds,
  humaniseEnum,
  splitWindow,
} from "./engagementAllocations";

const U1 = "00000000-0000-0000-0000-000000000001";
const U2 = "00000000-0000-0000-0000-000000000002";

function alloc(over: Partial<EngagementAllocation> = {}): EngagementAllocation {
  return {
    id: "a1",
    userId: U1,
    engagementId: "00000000-0000-0000-0000-0000000000e1",
    startDate: "2026-09-21",
    endDate: "2026-09-23",
    ...over,
  };
}

describe("humaniseEnum", () => {
  it("titles the known values", () => {
    expect(humaniseEnum("CONSULTANCY")).toBe("Consultancy");
    expect(humaniseEnum("ONSITE")).toBe("Onsite");
    expect(humaniseEnum("OFFSITE")).toBe("Offsite");
  });

  it("humanises a value it has never seen: underscores out, Title Case", () => {
    expect(humaniseEnum("PROFESSIONAL_SERVICES")).toBe("Professional Services");
    expect(humaniseEnum("hybrid_remote")).toBe("Hybrid Remote");
  });

  it("is empty for nothing", () => {
    expect(humaniseEnum(undefined)).toBe("");
    expect(humaniseEnum("")).toBe("");
  });
});

describe("allocationLabel", () => {
  it("is the account and the engagement id", () => {
    expect(allocationLabel(alloc({ accountName: "Example Corp", engagementDisplayId: "ENG-1001" }))).toBe(
      "Example Corp - ENG-1001",
    );
  });

  it("is the account alone when there is no engagement id", () => {
    expect(allocationLabel(alloc({ accountName: "Example Corp" }))).toBe("Example Corp");
  });

  it("falls back to the engagement id, then to the word Engagement", () => {
    expect(allocationLabel(alloc({ engagementDisplayId: "ENG-1001" }))).toBe("ENG-1001");
    expect(allocationLabel(alloc())).toBe("Engagement");
  });
});

describe("allocationTitle", () => {
  it("joins every part with the bullet, in order", () => {
    expect(
      allocationTitle(
        alloc({
          engagementType: "CONSULTANCY",
          modeOfDelivery: "ONSITE",
          allocationId: "ALLOC-7",
          utilization: 50,
        }),
      ),
    ).toBe("Consultancy ● Onsite ● ALLOC-7 ● 50% ● 2026-09-21 - 2026-09-23");
  });

  it("leaves out what is absent, keeping the dates", () => {
    expect(allocationTitle(alloc())).toBe("2026-09-21 - 2026-09-23");
    expect(allocationTitle(alloc({ modeOfDelivery: "OFFSITE" }))).toBe("Offsite ● 2026-09-21 - 2026-09-23");
  });

  it("keeps a utilization of zero", () => {
    expect(allocationTitle(alloc({ utilization: 0 }))).toBe("0% ● 2026-09-21 - 2026-09-23");
  });
});

describe("splitWindow", () => {
  it("leaves a window within 70 days whole", () => {
    expect(splitWindow("2026-09-21", "2026-09-27")).toEqual([{ from: "2026-09-21", to: "2026-09-27" }]);
    // Exactly 70 days inclusive is still one request.
    expect(splitWindow("2026-01-01", "2026-03-11")).toEqual([{ from: "2026-01-01", to: "2026-03-11" }]);
  });

  it("cuts a longer window into consecutive pieces of at most 70 days, no day twice", () => {
    // 71 days: one full piece and one day.
    expect(splitWindow("2026-01-01", "2026-03-12")).toEqual([
      { from: "2026-01-01", to: "2026-03-11" },
      { from: "2026-03-12", to: "2026-03-12" },
    ]);
    const pieces = splitWindow("2026-01-01", "2026-06-30");
    expect(pieces[0].from).toBe("2026-01-01");
    expect(pieces[pieces.length - 1].to).toBe("2026-06-30");
    for (let i = 1; i < pieces.length; i++) {
      const prev = new Date(`${pieces[i - 1].to}T00:00:00Z`).getTime();
      expect(new Date(`${pieces[i].from}T00:00:00Z`).getTime() - prev).toBe(86_400_000);
    }
  });

  it("gives no pieces for an empty or unreadable window", () => {
    expect(splitWindow("2026-09-27", "2026-09-21")).toEqual([]);
    expect(splitWindow("", "")).toEqual([]);
  });
});

describe("chunk and distinctUserIds", () => {
  it("batches at 500 by default", () => {
    const ids = Array.from({ length: 1201 }, (_, i) => String(i));
    expect(chunk(ids).map((c) => c.length)).toEqual([500, 500, 201]);
    expect(chunk([])).toEqual([]);
  });

  it("deduplicates across sources, sorts, and drops blanks", () => {
    expect(
      distinctUserIds([{ userId: U2 }, { userId: U1 }], [{ userId: U1 }, { userId: "" }, { userId: undefined }]),
    ).toEqual([U1, U2]);
  });
});

describe("allocationsByPersonAndDay", () => {
  const days = ["2026-09-21", "2026-09-22", "2026-09-23", "2026-09-24"];

  it("draws one entry for every day a run covers inside the window", () => {
    const out = allocationsByPersonAndDay([alloc({ startDate: "2026-09-22", endDate: "2026-09-30" })], days);
    expect([...out.get(U1)!.keys()]).toEqual(["2026-09-22", "2026-09-23", "2026-09-24"]);
  });

  it("ignores days outside the window", () => {
    const out = allocationsByPersonAndDay([alloc({ startDate: "2026-08-01", endDate: "2026-08-05" })], days);
    expect(out.size).toBe(0);
  });

  it("keeps every allocation on a day, in label order", () => {
    const out = allocationsByPersonAndDay(
      [
        alloc({ id: "b", accountName: "Zeta Ltd", startDate: "2026-09-21", endDate: "2026-09-21" }),
        alloc({ id: "a", accountName: "Alpha Ltd", startDate: "2026-09-21", endDate: "2026-09-22" }),
      ],
      days,
    );
    expect(out.get(U1)!.get("2026-09-21")!.map((x) => x.id)).toEqual(["a", "b"]);
    expect(out.get(U1)!.get("2026-09-22")!.map((x) => x.id)).toEqual(["a"]);
  });

  it("counts an allocation returned twice once", () => {
    const a = alloc();
    const out = allocationsByPersonAndDay([a, { ...a }], days);
    expect(out.get(U1)!.get("2026-09-21")).toHaveLength(1);
  });
});

describe("days taken off the roster", () => {
  const days = ["2026-09-21", "2026-09-22", "2026-09-23", "2026-09-24"];

  it("leaves a removed day out of the drawn chips", () => {
    const out = allocationsByPersonAndDay(
      [alloc({ startDate: "2026-09-21", endDate: "2026-09-24", dismissedDates: ["2026-09-22"] })],
      days,
    );
    expect([...out.get(U1)!.keys()]).toEqual(["2026-09-21", "2026-09-23", "2026-09-24"]);
  });

  it("lists the removed days on the grid for the picker's Restore", () => {
    const out = dismissedByPersonAndDay(
      [alloc({ startDate: "2026-09-21", endDate: "2026-09-30", dismissedDates: ["2026-09-22", "2026-09-29"] })],
      days,
    );
    expect([...out.get(U1)!.keys()]).toEqual(["2026-09-22"]);
    expect(dismissedByPersonAndDay([alloc()], days).size).toBe(0);
  });
});
