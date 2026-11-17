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

import { afterEach, describe, expect, it } from "vitest";
import { DEFAULT_BILLING_EMAIL, billingEmail, billingEmailHref } from "@config/brandingConfig";
import type { CustomerPortalWindowConfig } from "@config/portalConfig";

const originalConfig = window.config;

function setBillingEmail(value: string | undefined): void {
  window.config = {
    ...originalConfig,
    CUSTOMER_PORTAL_BILLING_EMAIL: value,
  } as CustomerPortalWindowConfig;
}

describe("billingEmail", () => {
  afterEach(() => {
    window.config = originalConfig;
  });

  it("is the WSO2 billing address when none is set", () => {
    setBillingEmail(undefined);
    expect(billingEmail()).toBe(DEFAULT_BILLING_EMAIL);
    expect(DEFAULT_BILLING_EMAIL).toBe("billing@wso2.com");
  });

  it("uses the configured address, trimmed", () => {
    setBillingEmail("  billing@acme.example ");
    expect(billingEmail()).toBe("billing@acme.example");
  });

  it("falls back to the default for a blank or malformed value", () => {
    for (const value of [
      "   ",
      "billing",
      "billing@",
      "two words@acme.example",
      "a@b@c",
      "mailto:billing@acme.example",
      "billing@acme.example?subject=hi",
      ".billing@acme.example",
      "billing.@acme.example",
      "billing..accounts@acme.example",
    ]) {
      setBillingEmail(value);
      expect(billingEmail()).toBe(DEFAULT_BILLING_EMAIL);
    }
  });

  it("accepts dots between non-empty parts of the local part", () => {
    setBillingEmail("billing.accounts@acme.example");
    expect(billingEmail()).toBe("billing.accounts@acme.example");
  });

  it("accepts a # in the local part, which RFC 5322 allows unquoted", () => {
    setBillingEmail("billing#accounts@acme.example");
    expect(billingEmail()).toBe("billing#accounts@acme.example");
  });
});

describe("billingEmailHref", () => {
  afterEach(() => {
    window.config = originalConfig;
  });

  it("links the default address as is", () => {
    setBillingEmail(undefined);
    expect(billingEmailHref()).toBe("mailto:billing@wso2.com");
  });

  it("percent-encodes characters that would end the recipient", () => {
    setBillingEmail("billing#accounts@acme.example");
    expect(billingEmailHref()).toBe("mailto:billing%23accounts@acme.example");
    setBillingEmail("bill?ing%x@acme.example");
    expect(billingEmailHref()).toBe("mailto:bill%3Fing%25x@acme.example");
  });
});
