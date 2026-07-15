// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { describe, expect, it } from "vitest";
import {
  PiiType,
  passesLuhn,
  detectPii,
  containsPii,
  getDetectedPiiLabels,
} from "@features/support/utils/piiDetection";

describe("piiDetection", () => {
  describe("passesLuhn", () => {
    it("accepts valid card numbers", () => {
      expect(passesLuhn("4242424242424242")).toBe(true); // Visa test number
      expect(passesLuhn("5555555555554444")).toBe(true); // Mastercard test number
      expect(passesLuhn("4242 4242 4242 4242")).toBe(true); // with spaces
    });

    it("rejects numbers that fail the checksum", () => {
      expect(passesLuhn("4242424242424241")).toBe(false);
      expect(passesLuhn("1234567890123456")).toBe(false);
    });

    it("rejects sequences of the wrong length", () => {
      expect(passesLuhn("42424242")).toBe(false); // too short
      expect(passesLuhn("42424242424242424242")).toBe(false); // too long
    });
  });

  describe("detectPii - credit cards", () => {
    it("detects a Luhn-valid card number", () => {
      const matches = detectPii("My card is 4242 4242 4242 4242 please charge it");
      expect(matches).toHaveLength(1);
      expect(matches[0].type).toBe(PiiType.CREDIT_CARD);
    });

    it("does not flag a random long digit string that fails Luhn", () => {
      const matches = detectPii("Order reference 1234567890123456 shipped today");
      expect(matches.some((m) => m.type === PiiType.CREDIT_CARD)).toBe(false);
    });
  });

  describe("detectPii - Danish CPR", () => {
    it("detects a CPR number with hyphen", () => {
      const matches = detectPii("My CPR is 010203-1234");
      expect(matches.some((m) => m.type === PiiType.DANISH_CPR)).toBe(true);
    });

    it("detects a CPR number without hyphen", () => {
      const matches = detectPii("CPR 0102031234 on file");
      expect(matches.some((m) => m.type === PiiType.DANISH_CPR)).toBe(true);
    });

    it("does not flag an invalid day/month", () => {
      const matches = detectPii("Ticket 991399-1234 raised");
      expect(matches.some((m) => m.type === PiiType.DANISH_CPR)).toBe(false);
    });
  });

  describe("detectPii - email and phone", () => {
    it("detects an email address", () => {
      const matches = detectPii("Reach me at john.doe@example.com anytime");
      expect(matches.some((m) => m.type === PiiType.EMAIL)).toBe(true);
    });

    it("detects a phone number", () => {
      const matches = detectPii("Call +45 12 34 56 78 for support");
      expect(matches.some((m) => m.type === PiiType.PHONE)).toBe(true);
    });

    it("does not flag a short number as a phone", () => {
      const matches = detectPii("There are 42 open cases");
      expect(matches.some((m) => m.type === PiiType.PHONE)).toBe(false);
    });
  });

  describe("detectPii - IBAN", () => {
    it("detects an IBAN", () => {
      const matches = detectPii("Transfer to DK5000400440116243 today");
      expect(matches.some((m) => m.type === PiiType.IBAN)).toBe(true);
    });
  });

  describe("detectPii - clean input", () => {
    it("returns no matches for benign text", () => {
      expect(detectPii("The deployment failed with a 500 error on startup.")).toEqual([]);
    });

    it("returns empty array for empty input", () => {
      expect(detectPii("")).toEqual([]);
    });
  });

  describe("detectPii - multiple and ordering", () => {
    it("returns matches sorted by position", () => {
      const text = "Email john@example.com then card 4242 4242 4242 4242";
      const matches = detectPii(text);
      expect(matches.length).toBeGreaterThanOrEqual(2);
      for (let i = 1; i < matches.length; i += 1) {
        expect(matches[i].index).toBeGreaterThanOrEqual(matches[i - 1].index);
      }
    });
  });

  describe("containsPii", () => {
    it("returns true when PII is present", () => {
      expect(containsPii("card 4242 4242 4242 4242")).toBe(true);
    });

    it("returns false for clean text", () => {
      expect(containsPii("no sensitive data here")).toBe(false);
    });
  });

  describe("getDetectedPiiLabels", () => {
    it("returns distinct labels", () => {
      const labels = getDetectedPiiLabels(
        "a@b.com and c@d.com and card 4242 4242 4242 4242"
      );
      expect(labels).toContain("Email address");
      expect(labels).toContain("Credit card number");
      // Two emails collapse to a single label.
      expect(labels.filter((l) => l === "Email address")).toHaveLength(1);
    });
  });
});
