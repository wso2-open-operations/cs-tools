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
  outageStatusColor,
  outageStatusLabel,
  outageTypeColor,
  outageTypeLabel,
} from "@features/csm-operations/utils/outages";

describe("outageTypeLabel", () => {
  it("labels the three documented types", () => {
    expect(outageTypeLabel("outage")).toBe("Outage");
    expect(outageTypeLabel("degradation")).toBe("Degradation");
    expect(outageTypeLabel("planned")).toBe("Planned");
  });

  it("falls back to a title-cased raw value for an uncurated type", () => {
    // The backing choice list is configurable independently of this portal —
    // an unrecognized type must still render something sane, not "—".
    expect(outageTypeLabel("maintenance")).toBe("Maintenance");
  });

  it("renders — for a missing type", () => {
    expect(outageTypeLabel(null)).toBe("—");
    expect(outageTypeLabel(undefined)).toBe("—");
  });
});

describe("outageTypeColor", () => {
  it("maps each documented type to a distinct color", () => {
    expect(outageTypeColor("outage")).toBe("error");
    expect(outageTypeColor("degradation")).toBe("warning");
    expect(outageTypeColor("planned")).toBe("info");
  });

  it("defaults an unknown type to default", () => {
    expect(outageTypeColor("maintenance")).toBe("default");
    expect(outageTypeColor(null)).toBe("default");
  });
});

describe("outageStatusLabel / outageStatusColor", () => {
  it("labels and colors in_progress as an active/error state", () => {
    expect(outageStatusLabel("in_progress")).toBe("In progress");
    expect(outageStatusColor("in_progress")).toBe("error");
  });

  it("labels and colors resolved as a settled/success state", () => {
    expect(outageStatusLabel("resolved")).toBe("Resolved");
    expect(outageStatusColor("resolved")).toBe("success");
  });

  it("handles a missing status", () => {
    expect(outageStatusLabel(null)).toBe("—");
    expect(outageStatusColor(null)).toBe("default");
  });
});
