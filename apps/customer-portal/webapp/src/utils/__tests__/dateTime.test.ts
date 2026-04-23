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

import { afterEach, describe, expect, it } from "vitest";
import {
  clearUserPreferredTimeZone,
  formatBackendTimestampForDisplay,
  resolveDisplayTimeZone,
  setUserPreferredTimeZone,
} from "@utils/dateTime";

describe("dateTime timezone resolution", () => {
  afterEach(() => {
    clearUserPreferredTimeZone();
  });

  it("should format backend UTC timestamp using users/me timezone", () => {
    setUserPreferredTimeZone("Asia/Colombo");

    const formatted = formatBackendTimestampForDisplay(
      "2026-04-21 02:14:46",
      {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "numeric",
        minute: "2-digit",
        second: "2-digit",
        hour12: true,
      },
      undefined,
      "en-US",
    );

    expect(formatted).toContain("7:44:46");
    expect(formatted).toContain("AM");
  });

  it("should fall back to browser timezone when users/me timezone is null", () => {
    setUserPreferredTimeZone(null);

    const browserTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    expect(resolveDisplayTimeZone()).toBe(browserTimeZone);
  });
});

