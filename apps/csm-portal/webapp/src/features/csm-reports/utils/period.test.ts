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
import { quarterStart, toDateInputValue } from "@features/csm-reports/utils/period";

describe("toDateInputValue", () => {
  it("pads the month and day", () => {
    expect(toDateInputValue(new Date(2026, 0, 5))).toBe("2026-01-05");
    expect(toDateInputValue(new Date(2026, 10, 11))).toBe("2026-11-11");
  });
});

describe("quarterStart", () => {
  it("is the first day of the quarter the date is in", () => {
    expect(toDateInputValue(quarterStart(new Date(2026, 0, 31)))).toBe("2026-01-01");
    expect(toDateInputValue(quarterStart(new Date(2026, 2, 31)))).toBe("2026-01-01");
    expect(toDateInputValue(quarterStart(new Date(2026, 3, 1)))).toBe("2026-04-01");
    expect(toDateInputValue(quarterStart(new Date(2026, 8, 30)))).toBe("2026-07-01");
    expect(toDateInputValue(quarterStart(new Date(2026, 10, 11)))).toBe("2026-10-01");
    expect(toDateInputValue(quarterStart(new Date(2026, 11, 31)))).toBe("2026-10-01");
  });
});
