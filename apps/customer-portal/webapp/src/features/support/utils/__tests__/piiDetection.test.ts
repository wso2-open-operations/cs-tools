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
      const matches = detectPii("My CPR is 010203-0001");
      expect(matches.some((m) => m.type === PiiType.DANISH_CPR)).toBe(true);
    });

    it("detects a CPR number without hyphen", () => {
      const matches = detectPii("CPR 0102030001 on file");
      expect(matches.some((m) => m.type === PiiType.DANISH_CPR)).toBe(true);
    });

    it("does not flag an invalid day/month", () => {
      const matches = detectPii("Ticket 991399-1234 raised");
      expect(matches.some((m) => m.type === PiiType.DANISH_CPR)).toBe(false);
    });

    it("does not flag a date-shaped order ref that fails the mod-11 check", () => {
      const matches = detectPii("Order 100200-3004 was created");
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

    it("does not flag a long generic number (>15 digits) as a phone", () => {
      const matches = detectPii("Reference 20260415123456789012 was logged");
      expect(matches.some((m) => m.type === PiiType.PHONE)).toBe(false);
    });

    it("does not flag an IPv4 address as a phone", () => {
      const matches = detectPii("Client connected from 192.168.1.100 at startup");
      expect(matches.some((m) => m.type === PiiType.PHONE)).toBe(false);
    });

    it("does not flag a unix timestamp or bare id as a phone", () => {
      expect(
        detectPii("event at 1737382920 UTC").some((m) => m.type === PiiType.PHONE),
      ).toBe(false);
      expect(
        detectPii("Order 1002003004 refunded").some((m) => m.type === PiiType.PHONE),
      ).toBe(false);
    });

    it("still detects a formatted phone number with grouping", () => {
      expect(
        detectPii("call (415) 555-2671").some((m) => m.type === PiiType.PHONE),
      ).toBe(true);
    });
  });

  describe("detectPii - IBAN", () => {
    it("detects an IBAN", () => {
      const matches = detectPii("Transfer to DK5000400440116243 today");
      expect(matches.some((m) => m.type === PiiType.IBAN)).toBe(true);
    });

    it("does not flag a two-letter-prefixed code that fails the mod-97 check", () => {
      const matches = detectPii("part number GB01ACME00012345678 shipped");
      expect(matches.some((m) => m.type === PiiType.IBAN)).toBe(false);
    });
  });

  describe("detectPii - secrets and credentials", () => {
    it("detects a PEM private key header", () => {
      const matches = detectPii(
        "Here is my key:\n-----BEGIN RSA PRIVATE KEY-----\nMIIEow...",
      );
      expect(matches.some((m) => m.type === PiiType.PRIVATE_KEY)).toBe(true);
    });

    it("detects credentials in a JDBC connection string", () => {
      const matches = detectPii("jdbc:mysql://admin:s3cretPw@db.internal:3306/wso2");
      expect(matches.some((m) => m.type === PiiType.CREDENTIALS_IN_URI)).toBe(true);
    });

    it("detects credentials in an https URL without also flagging an email", () => {
      const matches = detectPii("clone from https://user:token@github.com/org/repo");
      expect(matches.some((m) => m.type === PiiType.CREDENTIALS_IN_URI)).toBe(true);
      expect(matches.some((m) => m.type === PiiType.EMAIL)).toBe(false);
    });

    it("detects an AWS access key id", () => {
      const matches = detectPii("key AKIAIOSFODNN7EXAMPLE in the config");
      expect(matches.some((m) => m.type === PiiType.ACCESS_TOKEN)).toBe(true);
    });

    it("detects a GitHub token", () => {
      const matches = detectPii(
        "token ghp_1234567890abcdefghijklmnopqrstuvwxyz set",
      );
      expect(matches.some((m) => m.type === PiiType.ACCESS_TOKEN)).toBe(true);
    });

    it("detects a JWT", () => {
      const matches = detectPii(
        "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.abc123-_",
      );
      expect(matches.some((m) => m.type === PiiType.JWT)).toBe(true);
    });

    it("detects a password assigned in a config line", () => {
      const matches = detectPii('keystore password="wso2carbon123"');
      expect(matches.some((m) => m.type === PiiType.PASSWORD)).toBe(true);
    });

    it("does not flag prose mentioning a password with no value", () => {
      const matches = detectPii("The user forgot their password and cannot log in");
      expect(matches.some((m) => m.type === PiiType.PASSWORD)).toBe(false);
    });

    it("detects short secret keyword variants (pass, pin, token)", () => {
      expect(detectPii("pass: hunter2").some((m) => m.type === PiiType.PASSWORD)).toBe(
        true,
      );
      expect(detectPii("pin=4821").some((m) => m.type === PiiType.PASSWORD)).toBe(true);
      expect(
        detectPii("token: Xk9223LmQpR8vTnW4bLz").some(
          (m) => m.type === PiiType.PASSWORD,
        ),
      ).toBe(true);
    });

    it("detects an Azure-style AccountKey", () => {
      const matches = detectPii(
        "DefaultEndpointsProtocol=https;AccountKey=abc123def456==",
      );
      expect(matches.some((m) => m.type === PiiType.PASSWORD)).toBe(true);
    });

    it("detects Anthropic, OpenAI and Google OAuth tokens", () => {
      expect(
        detectPii("ANTHROPIC_API_KEY=sk-ant-api03-AbC123dEf456GhI789xyz").some(
          (m) => m.type === PiiType.ACCESS_TOKEN,
        ),
      ).toBe(true);
      expect(
        detectPii("OPENAI_API_KEY=sk-proj-AbC123dEf456GhI789jkl012mno").some(
          (m) => m.type === PiiType.ACCESS_TOKEN,
        ),
      ).toBe(true);
      expect(
        detectPii("token ya29.a0Af_longvalueherexyz12345").some(
          (m) => m.type === PiiType.ACCESS_TOKEN,
        ),
      ).toBe(true);
    });

    it("detects an opaque Authorization header token", () => {
      const matches = detectPii("Authorization: Bearer abc123def456ghi789jkl");
      expect(matches.some((m) => m.type === PiiType.ACCESS_TOKEN)).toBe(true);
    });
  });

  describe("detectPii - national identifiers", () => {
    it("detects a UK National Insurance number", () => {
      const matches = detectPii("My NINO is AB 12 34 56 C");
      expect(matches.some((m) => m.type === PiiType.UK_NINO)).toBe(true);
    });

    it("does not flag a NINO with an administratively invalid prefix", () => {
      // "QQ" is HMRC's reserved test prefix; "BG" is never allocated.
      expect(
        detectPii("code QQ 12 34 56 C").some((m) => m.type === PiiType.UK_NINO),
      ).toBe(false);
      expect(
        detectPii("code BG 12 34 56 C").some((m) => m.type === PiiType.UK_NINO),
      ).toBe(false);
    });

    it("does not flag an SSN with an invalid area number", () => {
      expect(
        detectPii("id 000-45-6789").some((m) => m.type === PiiType.NATIONAL_ID),
      ).toBe(false);
      expect(
        detectPii("id 666-45-6789").some((m) => m.type === PiiType.NATIONAL_ID),
      ).toBe(false);
    });

    it("detects a valid SSN", () => {
      expect(
        detectPii("ssn 123-45-6789").some((m) => m.type === PiiType.NATIONAL_ID),
      ).toBe(true);
    });

    it("detects a passport number when labelled", () => {
      const matches = detectPii("German passport: C01X00T47");
      expect(matches.some((m) => m.type === PiiType.PASSPORT)).toBe(true);
    });

    it("does not flag a bare alphanumeric code without the word passport", () => {
      const matches = detectPii("Build artifact C01X00T47 completed");
      expect(matches.some((m) => m.type === PiiType.PASSPORT)).toBe(false);
    });
  });

  describe("detectPii - rich-text HTML input", () => {
    it("detects PII in link attributes that plain-text extraction would drop", () => {
      const emailMatches = detectPii(
        '<p><a href="mailto:john@example.com">contact us</a></p>',
      );
      expect(emailMatches.some((m) => m.type === PiiType.EMAIL)).toBe(true);

      const credMatches = detectPii(
        '<a href="https://user:token123@github.com/org/repo">clone</a>',
      );
      expect(credMatches.some((m) => m.type === PiiType.CREDENTIALS_IN_URI)).toBe(true);
    });

    it("detects visible PII inside markup", () => {
      const matches = detectPii("<p>My card is 4242 4242 4242 4242</p>");
      expect(matches.some((m) => m.type === PiiType.CREDIT_CARD)).toBe(true);
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
