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
import { formatProjectDate, getStatusColor } from "@features/project-hub/utils/projectCard";

describe("projectCard utilities", () => {
  describe("formatProjectDate", () => {
    it("should format valid date strings correctly", () => {
      expect(formatProjectDate("2025-07-17 09:06:14")).toBe(
        "Created 17 Jul 2025",
      );
      expect(formatProjectDate("2026-01-01 12:00:00")).toBe(
        "Created 1 Jan 2026",
      );
    });

    it("should handle ISO date strings", () => {
      expect(formatProjectDate("2026-01-29T11:28:40+05:30")).toBe(
        "Created 29 Jan 2026",
      );
    });

    it("should return empty string for empty input", () => {
      expect(formatProjectDate("")).toBe("");
    });

    it("should return the original string for invalid dates", () => {
      const invalidDate = "not-a-date";
      expect(formatProjectDate(invalidDate)).toBe(invalidDate);
    });
  });

  describe("getStatusColor", () => {
    it("should return 'success' for 'All Good'", () => {
      expect(getStatusColor("All Good")).toBe("success");
    });

    it("should return 'warning' for 'Need Attention'", () => {
      expect(getStatusColor("Need Attention")).toBe("warning");
    });

    it("should return 'error' for 'Critical Issues'", () => {
      expect(getStatusColor("Critical Issues")).toBe("error");
    });

    it("should return 'default' for unknown statuses", () => {
      expect(getStatusColor("Unknown")).toBe("default");
      expect(getStatusColor("")).toBe("default");
    });
  });
});
