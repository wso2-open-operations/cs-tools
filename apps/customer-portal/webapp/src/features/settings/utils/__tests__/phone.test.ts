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
  formatPhoneForDisplay,
  parseE164,
  parseE164ToCountryCode,
  toE164FromCountryCode,
  validatePhoneE164,
} from "@features/settings/utils/phone";

describe("phone utils", () => {
  it("formats and validates phone numbers", () => {
    expect(formatPhoneForDisplay("+14155552671")).toContain("+1415");
    expect(validatePhoneE164("+14155552671")).toBe("");
    expect(validatePhoneE164("14155552671")).toContain("valid phone number");
  });

  it("parses and builds E.164", () => {
    expect(parseE164("+14155552671").nationalNumber.length).toBeGreaterThan(5);
    expect(parseE164ToCountryCode("+14155552671").countryCode).toBeTruthy();
    expect(toE164FromCountryCode("US", "4155552671")).toContain("+1");
  });
});

