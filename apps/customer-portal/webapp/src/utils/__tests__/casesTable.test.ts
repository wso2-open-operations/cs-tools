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
import { getStatusColor, getSeverityColor } from "@utils/casesTable";

describe("casesTable utils", () => {
  describe("getSeverityColor", () => {
    it("should return correct MUI color paths for severity levels", () => {
      expect(getSeverityColor("S0")).toBe("error.main");
      expect(getSeverityColor("S1")).toBe("warning.main");
      expect(getSeverityColor("S2")).toBe("text.disabled");
      expect(getSeverityColor("S3")).toBe("info.main");
      expect(getSeverityColor("S4")).toBe("success.main");
      expect(getSeverityColor("Unknown")).toBe("text.secondary");
      expect(getSeverityColor(undefined)).toBe("text.secondary");
    });
  });

  describe("getStatusColor", () => {
    it("should return 'info.main' for open status", () => {
      expect(getStatusColor("Open")).toBe("info.main");
    });

    it("should return 'primary.main' for awaiting response", () => {
      expect(getStatusColor("Awaiting Response")).toBe("primary.main");
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
