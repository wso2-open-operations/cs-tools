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

import { describe, expect, it, vi, afterEach, beforeEach } from "vitest";
import {
  formatProjectDate,
  getSLAStatusColor,
  getSupportTierColor,
  getProjectTypeColor,
  getSystemHealthColor,
  getSubscriptionStatus,
  getSubscriptionColor,
  calculateProgress,
} from "../projectStats";

describe("projectStats utils", () => {
  describe("formatProjectDate", () => {
    it("should format valid date strings correctly", () => {
      expect(formatProjectDate("2024-01-15")).toBe("Jan 15, 2024");
      expect(formatProjectDate("2023-12-31")).toBe("Dec 31, 2023");
    });

    it("should handle empty or null input", () => {
      expect(formatProjectDate("")).toBe("");
      // @ts-ignore - testing runtime behavior for null
      expect(formatProjectDate(null)).toBe("");
    });

    it("should handle full ISO strings", () => {
      expect(formatProjectDate("2024-01-15T12:00:00Z")).toBe("Jan 15, 2024");
    });
  });

  describe("getSLAStatusColor", () => {
    it("should return 'success' for 'All Good' or 'Good'", () => {
      expect(getSLAStatusColor("All Good")).toBe("success");
      expect(getSLAStatusColor("all good")).toBe("success");
      expect(getSLAStatusColor("Good")).toBe("success");
      expect(getSLAStatusColor("good")).toBe("success");
    });

    it("should return 'error' for 'Bad'", () => {
      expect(getSLAStatusColor("Bad")).toBe("error");
      expect(getSLAStatusColor("bad")).toBe("error");
    });

    it("should return 'default' for unknown values", () => {
      expect(getSLAStatusColor("Met")).toBe("default");
      expect(getSLAStatusColor("Unknown")).toBe("default");
      // @ts-ignore
      expect(getSLAStatusColor(null)).toBe("default");
    });
  });

  describe("getSupportTierColor", () => {
    it("should return 'warning' for 'Enterprise'", () => {
      expect(getSupportTierColor("Enterprise")).toBe("warning");
      expect(getSupportTierColor("enterprise")).toBe("warning");
    });

    it("should return 'info' for 'Standard'", () => {
      expect(getSupportTierColor("Standard")).toBe("info");
      expect(getSupportTierColor("standard")).toBe("info");
    });

    it("should return 'default' for unknown values", () => {
      expect(getSupportTierColor("Pro")).toBe("default");
      expect(getSupportTierColor("Basic")).toBe("default");
    });
  });

  describe("getProjectTypeColor", () => {
    it("should return 'info' for 'Subscription'", () => {
      expect(getProjectTypeColor("Subscription")).toBe("info");
      expect(getProjectTypeColor("subscription")).toBe("info");
    });

    it("should return 'warning' for 'Free'", () => {
      expect(getProjectTypeColor("Free")).toBe("warning");
      expect(getProjectTypeColor("free")).toBe("warning");
    });

    it("should return 'default' for unknown values", () => {
      expect(getProjectTypeColor("Trial")).toBe("default");
    });
  });

  describe("getSystemHealthColor", () => {
    it("should return 'success' for 'Healthy'", () => {
      expect(getSystemHealthColor("Healthy")).toBe("success");
      expect(getSystemHealthColor("healthy")).toBe("success");
    });

    it("should return 'error' for 'Critical'", () => {
      expect(getSystemHealthColor("Critical")).toBe("error");
      expect(getSystemHealthColor("critical")).toBe("error");
    });

    it("should return 'default' for unknown values", () => {
      expect(getSystemHealthColor("Maintenance")).toBe("default");
    });
  });

  describe("getSubscriptionStatus", () => {
    beforeEach(() => {
      vi.useFakeTimers();
      // Set fixed date: Jan 15, 2024
      vi.setSystemTime(new Date("2024-01-15T00:00:00Z"));
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("should return 'Active' for empty date", () => {
      expect(getSubscriptionStatus("")).toBe("Active");
    });

    it("should return 'Expired' for past dates", () => {
      // Jan 14, 2024 (1 day ago)
      expect(getSubscriptionStatus("2024-01-14T00:00:00Z")).toBe("Expired");
    });

    it("should return 'Expiring Soon' for dates within 30 days", () => {
      // Feb 14, 2024 (30 days from now)
      expect(getSubscriptionStatus("2024-02-14T00:00:00Z")).toBe(
        "Expiring Soon",
      );
    });

    it("should return 'Active' for dates more than 30 days in future", () => {
      // Feb 15, 2024 (31 days from now)
      expect(getSubscriptionStatus("2024-02-15T00:00:00Z")).toBe("Active");
    });
  });

  describe("getSubscriptionColor", () => {
    it("should return 'error' for 'Expired'", () => {
      expect(getSubscriptionColor("Expired")).toBe("error");
      expect(getSubscriptionColor("expired")).toBe("error");
    });

    it("should return 'warning' for 'Expiring Soon'", () => {
      expect(getSubscriptionColor("Expiring Soon")).toBe("warning");
      expect(getSubscriptionColor("expiring soon")).toBe("warning");
    });

    it("should return 'success' for 'Active'", () => {
      expect(getSubscriptionColor("Active")).toBe("success");
      expect(getSubscriptionColor("active")).toBe("success");
    });

    it("should return 'default' for unknown values", () => {
      expect(getSubscriptionColor("Pending")).toBe("default");
    });
  });

  describe("calculateProgress", () => {
    beforeEach(() => {
      vi.useFakeTimers();
      // Set fixed date: Jan 15, 2024
      vi.setSystemTime(new Date("2024-01-15T00:00:00Z"));
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("should return 0 if start or end date is missing", () => {
      expect(calculateProgress("", "2024-01-15")).toBe(0);
      expect(calculateProgress("2024-01-01", "")).toBe(0);
    });

    it("should return 0 if start or end date is invalid", () => {
      expect(calculateProgress("invalid", "2024-01-15")).toBe(0);
      expect(calculateProgress("2024-01-01", "invalid")).toBe(0);
    });

    it("should return 100 if total duration is <= 0", () => {
      expect(calculateProgress("2024-01-15", "2024-01-15")).toBe(100);
      expect(calculateProgress("2024-01-16", "2024-01-15")).toBe(100);
    });

    it("should calculate correct progress percentage", () => {
      // Start: Jan 1, 2024, End: Jan 31, 2024, Today: Jan 15, 2024
      // Total days: 30, Elapsed: 14. Progress: ~46.6%
      const start = "2024-01-01T00:00:00Z";
      const end = "2024-01-31T00:00:00Z";
      const progress = calculateProgress(start, end);
      expect(progress).toBeCloseTo(46.66, 1);
    });

    it("should clamp progress between 0 and 100", () => {
      // Future start date
      expect(
        calculateProgress("2024-02-01T00:00:00Z", "2024-02-28T00:00:00Z"),
      ).toBe(0);

      // Past end date (already verified via other checks but good to be explicit)
      // If end date is before today, it means elapsed > total, so > 100, clamped to 100.
      expect(
        calculateProgress("2023-01-01T00:00:00Z", "2023-12-31T00:00:00Z"),
      ).toBe(100);
    });
  });
});
