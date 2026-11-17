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
import { getSendBackToWso2Action, isInternalUserRoles } from "./caseActions";

describe("isInternalUserRoles", () => {
  it("is true for staff roles, including the ServiceNow spellings", () => {
    for (const role of ["internal", "snc_internal", "agent", "wso2_agent"]) {
      expect(isInternalUserRoles([role])).toBe(true);
    }
  });

  it("is false for customers, partners, an admin of a customer, or nothing loaded", () => {
    expect(isInternalUserRoles(["customer_user"])).toBe(false);
    expect(isInternalUserRoles(["sn_customerservice.customer_admin", "partner_user"])).toBe(false);
    expect(isInternalUserRoles([])).toBe(false);
    expect(isInternalUserRoles(undefined)).toBe(false);
  });
});

describe("getSendBackToWso2Action", () => {
  it("is Reject Solution for everyone on a Solution Proposed case", () => {
    expect(getSendBackToWso2Action(true, false, false)).toEqual({ label: "Reject Solution", hidden: false });
    expect(getSendBackToWso2Action(true, false, true)).toEqual({ label: "Reject Solution", hidden: false });
  });

  it("offers Wait on WSO2 on an Awaiting Info case to staff only", () => {
    expect(getSendBackToWso2Action(false, true, true)).toEqual({ label: "Wait on WSO2", hidden: false });
    expect(getSendBackToWso2Action(false, true, false).hidden).toBe(true);
  });

  it("is hidden in every other state", () => {
    expect(getSendBackToWso2Action(false, false, true).hidden).toBe(true);
    expect(getSendBackToWso2Action(false, false, false).hidden).toBe(true);
  });
});
