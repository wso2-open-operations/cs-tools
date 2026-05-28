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
  parseSecurityTabQueryParam,
  parseSecurityReportCaseSortField,
  parseSecurityReportViewMode,
} from "@features/security/utils/securityPage";
import {
  SecurityReportCaseSortField,
  SecurityReportViewMode,
  SecurityTabId,
} from "@features/security/types/security";

describe("securityPage utils", () => {
  it("parses tab query param with fallback", () => {
    expect(parseSecurityTabQueryParam(SecurityTabId.COMPONENTS)).toBe(
      SecurityTabId.COMPONENTS,
    );
    expect(parseSecurityTabQueryParam("invalid")).toBe(
      SecurityTabId.VULNERABILITIES,
    );
  });

  it("parses report view mode with fallback to all", () => {
    expect(parseSecurityReportViewMode(SecurityReportViewMode.MY)).toBe(
      SecurityReportViewMode.MY,
    );
    expect(parseSecurityReportViewMode("unknown")).toBe(SecurityReportViewMode.ALL);
  });

  it("parses case sort field with fallback to createdOn", () => {
    expect(parseSecurityReportCaseSortField(SecurityReportCaseSortField.state)).toBe(
      SecurityReportCaseSortField.state,
    );
    expect(parseSecurityReportCaseSortField("bad")).toBe(
      SecurityReportCaseSortField.createdOn,
    );
  });
});
