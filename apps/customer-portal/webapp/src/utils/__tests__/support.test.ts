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
  getChatStatusAction,
  getChatStatusChipSx,
  getChatStatusRowSx,
  formatRelativeTime,
} from "@utils/support";

describe("support utils", () => {
  describe("getChatStatusAction", () => {
    it("should return 'resume' for Still Open", () => {
      expect(getChatStatusAction("Still Open")).toBe("resume");
      expect(getChatStatusAction("still open")).toBe("resume");
    });

    it("should return 'view' for Resolved and Abandoned", () => {
      expect(getChatStatusAction("Resolved")).toBe("view");
      expect(getChatStatusAction("Abandoned")).toBe("view");
      expect(getChatStatusAction("")).toBe("view");
    });
  });

  describe("getChatStatusChipSx", () => {
    it("should return green sx for Resolved", () => {
      const sx = getChatStatusChipSx("Resolved") as any;
      expect(sx.bgcolor).toBeDefined();
      expect(sx.color).toBeDefined();
      expect(sx.borderColor).toBeDefined();
    });

    it("should return blue sx for Still Open", () => {
      const sx = getChatStatusChipSx("Still Open") as any;
      expect(sx.bgcolor).toBeDefined();
      expect(sx.color).toBeDefined();
    });

    it("should return grey sx for Abandoned", () => {
      const sx = getChatStatusChipSx("Abandoned") as any;
      expect(sx.bgcolor).toBeDefined();
    });
  });

  describe("getChatStatusRowSx", () => {
    it("should return green tint for Resolved", () => {
      const sx = getChatStatusRowSx("Resolved") as any;
      expect(sx.bgcolor).toBeDefined();
    });

    it("should return blue tint for Still Open", () => {
      const sx = getChatStatusRowSx("Still Open") as any;
      expect(sx.bgcolor).toBeDefined();
    });
  });

  describe("formatRelativeTime", () => {
    it("should return '--' for undefined date", () => {
      expect(formatRelativeTime(undefined)).toBe("--");
    });

    it("should return 'just now' for very recent dates", () => {
      const now = new Date();
      expect(formatRelativeTime(now)).toBe("just now");
    });

    it("should format minutes ago", () => {
      const fiveMinsAgo = new Date(Date.now() - 5 * 60 * 1000);
      expect(formatRelativeTime(fiveMinsAgo)).toBe("5 minutes ago");
    });

    it("should format hours ago", () => {
      const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000);
      expect(formatRelativeTime(twoHoursAgo)).toBe("2 hours ago");
    });

    it("should format days ago", () => {
      const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000);
      expect(formatRelativeTime(threeDaysAgo)).toBe("3 days ago");
    });
  });
});
