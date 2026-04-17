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
import { getCaseTypeChipConfig } from "@features/dashboard/utils/dashboard";

describe("getCaseTypeChipConfig", () => {
  it("returns null for empty or undefined label", () => {
    expect(getCaseTypeChipConfig(undefined)).toBeNull();
    expect(getCaseTypeChipConfig(null)).toBeNull();
    expect(getCaseTypeChipConfig("")).toBeNull();
  });

  it("maps Incident to Case with FileText icon", () => {
    const config = getCaseTypeChipConfig("Incident");
    expect(config).not.toBeNull();
    expect(config!.displayLabel).toBe("Case");
    expect(config!.Icon).toBeDefined();
  });

  it("maps Query to Case with FileText icon", () => {
    const config = getCaseTypeChipConfig("Query");
    expect(config).not.toBeNull();
    expect(config!.displayLabel).toBe("Case");
  });

  it("maps Security Report Analysis correctly", () => {
    const config = getCaseTypeChipConfig("Security Report Analysis");
    expect(config).not.toBeNull();
    expect(config!.displayLabel).toBe("Security Report Analysis");
  });

  it("maps Service Request correctly", () => {
    const config = getCaseTypeChipConfig("Service Request");
    expect(config).not.toBeNull();
    expect(config!.displayLabel).toBe("Service Request");
  });

  it("maps Change Request correctly", () => {
    const config = getCaseTypeChipConfig("Change Request");
    expect(config).not.toBeNull();
    expect(config!.displayLabel).toBe("Change Request");
  });

  it("returns fallback config for unknown type", () => {
    const config = getCaseTypeChipConfig("Other Type");
    expect(config).not.toBeNull();
    expect(config!.displayLabel).toBe("Other Type");
  });
});
