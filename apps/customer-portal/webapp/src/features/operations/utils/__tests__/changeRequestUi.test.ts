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
import { ChangeRequestStates } from "@features/operations/constants/operationsConstants";
import {
  formatImpactLabel,
  getChangeRequestStateColor,
  resolveChangeRequestCanonicalState,
} from "@features/operations/utils/changeRequestUi";

describe("resolveChangeRequestCanonicalState", () => {
  it("maps API id to canonical label", () => {
    expect(resolveChangeRequestCanonicalState({ id: "-5", label: "x" })).toBe(
      ChangeRequestStates.NEW,
    );
  });

  it("normalizes Cancelled label to Canceled", () => {
    expect(resolveChangeRequestCanonicalState({ label: "Cancelled" })).toBe(
      ChangeRequestStates.CANCELED,
    );
  });
});

describe("formatImpactLabel", () => {
  it("strips numeric prefix", () => {
    expect(formatImpactLabel("1 - High")).toBe("High");
  });

  it("returns Not Available when undefined", () => {
    expect(formatImpactLabel(undefined)).toBe("Not Available");
  });
});

describe("getChangeRequestStateColor", () => {
  it("returns a hex-like string for known state id", () => {
    const c = getChangeRequestStateColor({ id: "-5" });
    expect(c).toMatch(/^#/);
  });
});
