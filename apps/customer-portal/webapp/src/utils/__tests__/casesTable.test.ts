/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it } from "vitest";
import { getPriorityColor, getStatusColor } from "../casesTable";

describe("casesTable utils", () => {
  describe("getPriorityColor", () => {
    it("should return 'error' for critical severity", () => {
      expect(getPriorityColor("Critical")).toBe("error");
      expect(getPriorityColor("S1")).toBe("error");
      expect(getPriorityColor("critical")).toBe("error");
    });

    it("should return 'warning' for high severity", () => {
      expect(getPriorityColor("High")).toBe("warning");
      expect(getPriorityColor("S2")).toBe("warning");
    });

    it("should return 'info' for medium severity", () => {
      expect(getPriorityColor("Medium")).toBe("info");
      expect(getPriorityColor("S3")).toBe("info");
    });

    it("should return 'success' for low severity", () => {
      expect(getPriorityColor("Low")).toBe("success");
      expect(getPriorityColor("S4")).toBe("success");
    });

    it("should return 'default' for unknown severity", () => {
      expect(getPriorityColor("Unknown")).toBe("default");
      expect(getPriorityColor(undefined)).toBe("default");
    });
  });

  describe("getStatusColor", () => {
    it("should return 'primary.main' for open status", () => {
      expect(getStatusColor("Open")).toBe("primary.main");
    });

    it("should return 'info.main' for awaiting response", () => {
      expect(getStatusColor("Awaiting Response")).toBe("info.main");
    });

    it("should return 'warning.main' for in progress", () => {
      expect(getStatusColor("In Progress")).toBe("warning.main");
    });

    it("should return 'success.main' for resolved/closed status", () => {
      expect(getStatusColor("Resolved")).toBe("success.main");
      expect(getStatusColor("Closed")).toBe("success.main");
    });

    it("should return 'text.secondary' for unknown status", () => {
      expect(getStatusColor("Unknown")).toBe("text.secondary");
    });
  });
});
