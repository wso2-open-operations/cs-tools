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

import { describe, expect, it, vi } from "vitest";
import {
  resolveRelativeDatePlaceholderToDate,
  resolveRelativeDatePlaceholderToUtcDate,
} from "./resolveRelativeDatePlaceholder";

interface ParityCase {
  now: string;
  token: string;
  date: string | null;
}

// entity-service's relative date parity cases, which its resolveRelativeDate
// and the CSM backend's relativeDate run too, so the three agree on every
// token. Read through Vite (this app's TypeScript has no Node types); the
// glob is empty when the file is missing, which fails below.
const parityFiles = import.meta.glob<{ cases: ParityCase[] }>(
  "../../../../../entity-service/internal/service/testdata/relative_date_parity.json",
  { eager: true, import: "default" },
);
const cases = Object.values(parityFiles)[0]?.cases ?? [];

describe("relative date parity with entity-service and the CSM backend", () => {
  it("has the cases", () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  it.each(cases.map((c) => [c.token, c.now, c.date] as const))(
    "%s at %s is %s by the UTC date, from any time zone",
    (token, now, date) => {
      for (const timeZone of ["UTC", "Asia/Colombo", "America/Los_Angeles"]) {
        vi.stubEnv("TZ", timeZone);
        try {
          expect(resolveRelativeDatePlaceholderToUtcDate(token, new Date(now)) ?? null).toBe(date);
        } finally {
          vi.unstubAllEnvs();
        }
      }
    },
  );

  it.each(cases.map((c) => [c.token, c.now, c.date] as const))(
    "%s at %s is %s by the local date for a viewer in UTC",
    (token, now, date) => {
      vi.stubEnv("TZ", "UTC");
      try {
        expect(resolveRelativeDatePlaceholderToDate(token, new Date(now)) ?? null).toBe(date);
      } finally {
        vi.unstubAllEnvs();
      }
    },
  );
});
