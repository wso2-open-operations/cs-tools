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
  getVulnerabilitySeverityColor,
  getVulnerabilityStatusColor,
} from "@features/security/utils/vulnerabilities";

describe("vulnerabilities utils", () => {
  it("maps severity labels to theme colors", () => {
    expect(getVulnerabilitySeverityColor(" Critical ")).toBe("error.main");
    expect(getVulnerabilitySeverityColor("HIGH")).toBe("warning.main");
    expect(getVulnerabilitySeverityColor("unknown")).toBe("text.secondary");
  });

  it("maps status labels to theme colors with strict matching", () => {
    expect(getVulnerabilityStatusColor("in progress")).toBe("warning.main");
    expect(getVulnerabilityStatusColor("resolved")).toBe("success.main");
    expect(getVulnerabilityStatusColor("unresolved")).toBe("text.secondary");
  });
});
