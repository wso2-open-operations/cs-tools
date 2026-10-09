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
import {
  formatCasesTableCaseIdentifier,
  formatCaseStatusLabel,
  getSeverityColor,
  getStatusColor,
} from "@features/dashboard/utils/casesTable";
import { CaseSeverityLevel, CaseStatus } from "@features/support/constants/supportConstants";

describe("casesTable utils", () => {
  describe("getSeverityColor", () => {
    it("should return correct MUI color paths for severity levels", () => {
      expect(getSeverityColor(CaseSeverityLevel.S0)).toBe("error.main");
      expect(getSeverityColor(CaseSeverityLevel.S1)).toBe("warning.main");
      expect(getSeverityColor(CaseSeverityLevel.S2)).toBe("info.main");
      expect(getSeverityColor(CaseSeverityLevel.S3)).toBe("secondary.main");
      expect(getSeverityColor(CaseSeverityLevel.S4)).toBe("success.main");
      expect(getSeverityColor("S4")).toBe("success.main");
      expect(getSeverityColor("Unknown")).toBe("text.secondary");
      expect(getSeverityColor(undefined)).toBe("text.secondary");
    });
  });

  describe("getStatusColor", () => {
    it("should return 'info.main' for open status", () => {
      expect(getStatusColor(CaseStatus.OPEN)).toBe("info.main");
    });

    it("should return 'primary.main' for awaiting response", () => {
      expect(getStatusColor(CaseStatus.AWAITING_INFO)).toBe("primary.main");
    });

    it("should return 'warning.main' for in progress", () => {
      expect(getStatusColor(CaseStatus.WORK_IN_PROGRESS)).toBe("warning.main");
    });

    it("should return 'success.main' for resolved/closed status", () => {
      expect(getStatusColor("Resolved")).toBe("success.main");
      expect(getStatusColor(CaseStatus.CLOSED)).toBe("success.main");
    });

    it("should return 'text.secondary' for unknown status", () => {
      expect(getStatusColor("Unknown")).toBe("text.secondary");
    });

    it("should match raw UPPER_SNAKE_CASE labels from the global search endpoint", () => {
      // Regression: the global search/project-hub cases table renders a raw
      // Postgres-sourced enum label ("WORK_IN_PROGRESS") rather than the
      // humanized form ("Work In Progress"), which used to fail every
      // .includes() check here and silently fall through to the default color.
      expect(getStatusColor("OPEN")).toBe("info.main");
      expect(getStatusColor("AWAITING_INFO")).toBe("primary.main");
      expect(getStatusColor("WORK_IN_PROGRESS")).toBe("warning.main");
      expect(getStatusColor("CLOSED")).toBe("success.main");
    });
  });

  describe("formatCaseStatusLabel", () => {
    it("converts a raw UPPER_SNAKE_CASE backend label to a human-readable one", () => {
      expect(formatCaseStatusLabel("WORK_IN_PROGRESS")).toBe("Work In Progress");
      expect(formatCaseStatusLabel("OPEN")).toBe("Open");
      expect(formatCaseStatusLabel("AWAITING_INFO")).toBe("Awaiting Info");
    });

    it("leaves an already human-readable label untouched", () => {
      expect(formatCaseStatusLabel("Work In Progress")).toBe("Work In Progress");
      expect(formatCaseStatusLabel("Open")).toBe("Open");
    });

    it("does not mangle a label that isn't a raw enum, even with an acronym or punctuation", () => {
      // A blind lower-case/re-title-case pass would turn these into
      // "Waiting On Wso2" and "On Hold (customer)" -- only a raw
      // UPPER_SNAKE_CASE value should ever be normalized.
      expect(formatCaseStatusLabel("Waiting On WSO2")).toBe("Waiting On WSO2");
      expect(formatCaseStatusLabel("On Hold (Customer)")).toBe("On Hold (Customer)");
    });

    it("returns '--' for an absent label", () => {
      expect(formatCaseStatusLabel(undefined)).toBe("--");
      expect(formatCaseStatusLabel(null)).toBe("--");
      expect(formatCaseStatusLabel("")).toBe("--");
    });

    it("returns '--' for a label that is whitespace-only or becomes blank after normalizing", () => {
      expect(formatCaseStatusLabel("   ")).toBe("--");
      // All-underscore input passes the raw-enum check but turns into pure
      // whitespace once underscores become spaces.
      expect(formatCaseStatusLabel("___")).toBe("--");
    });
  });

  describe("formatCasesTableCaseIdentifier", () => {
    it("should join number and internal id with a pipe after ID prefix", () => {
      expect(formatCasesTableCaseIdentifier("CS-001", "INT-1")).toBe(
        "ID: CS-001 | INT-1",
      );
    });

    it("should return ID and number when internal id is missing", () => {
      expect(formatCasesTableCaseIdentifier("CS-001", undefined)).toBe(
        "ID: CS-001",
      );
    });

    it("should return ID and internal id when number is missing", () => {
      expect(formatCasesTableCaseIdentifier(undefined, "INT-1")).toBe(
        "ID: INT-1",
      );
    });

    it("should return ID placeholder when both are missing", () => {
      expect(formatCasesTableCaseIdentifier(undefined, undefined)).toBe(
        "ID: --",
      );
    });
  });
});
