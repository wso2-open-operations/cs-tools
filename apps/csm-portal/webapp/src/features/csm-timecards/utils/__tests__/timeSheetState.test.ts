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
import { cardActions } from "@features/csm-timecards/utils/timeSheetState";

const OWNER = { isOwner: true, isApprover: false, isAdmin: false };
const APPROVER = { isOwner: false, isApprover: true, isAdmin: false };
const ADMIN = { isOwner: false, isApprover: false, isAdmin: true };
const NONE = { isOwner: false, isApprover: false, isAdmin: false };

describe("cardActions", () => {
  it("owner never has actions, even on a submitted card", () => {
    expect(cardActions("submitted", OWNER)).toEqual([]);
  });
  it("approver approves/rejects a submitted card that isn't their own", () => {
    expect(cardActions("submitted", APPROVER)).toEqual(["approve", "reject"]);
  });
  it("admin also approves/rejects a submitted card", () => {
    expect(cardActions("submitted", ADMIN)).toEqual(["approve", "reject"]);
  });
  it("no actions for a non-approver, non-owner", () => {
    expect(cardActions("submitted", NONE)).toEqual([]);
  });
  it("no actions once decided (approved/rejected)", () => {
    expect(cardActions("approved", APPROVER)).toEqual([]);
    expect(cardActions("rejected", APPROVER)).toEqual([]);
  });
});
