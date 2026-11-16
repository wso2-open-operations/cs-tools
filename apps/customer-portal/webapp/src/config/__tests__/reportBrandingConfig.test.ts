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

import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  DEFAULT_REPORT_COMPANY_NAME,
  hasConfiguredReportCompanyName,
  reportCompanyName,
  reportLogoUrl,
  reportSupportLine,
} from "@config/reportBrandingConfig";
import type { CustomerPortalWindowConfig } from "@config/portalConfig";

const originalConfig = window.config;

function setConfig(values: Partial<CustomerPortalWindowConfig>): void {
  window.config = { ...originalConfig, ...values } as CustomerPortalWindowConfig;
}

describe("reportBrandingConfig", () => {
  beforeEach(() => {
    setConfig({
      CUSTOMER_PORTAL_COMPANY_NAME: undefined,
      CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL: undefined,
      CUSTOMER_PORTAL_LOGO_DARK_MODE_URL: undefined,
    });
  });

  afterEach(() => {
    window.config = originalConfig;
  });

  it("prints the WSO2 support line when no company name is set", () => {
    expect(reportCompanyName()).toBe(DEFAULT_REPORT_COMPANY_NAME);
    expect(reportSupportLine()).toBe("WSO2 Support 24x7");
    expect(hasConfiguredReportCompanyName()).toBe(false);
  });

  it("treats a blank company name as unset", () => {
    setConfig({ CUSTOMER_PORTAL_COMPANY_NAME: "   " });
    expect(reportSupportLine()).toBe("WSO2 Support 24x7");
    expect(hasConfiguredReportCompanyName()).toBe(false);
  });

  it("puts the configured company name in the support line", () => {
    setConfig({ CUSTOMER_PORTAL_COMPANY_NAME: "  Acme Corp " });
    expect(reportCompanyName()).toBe("Acme Corp");
    expect(reportSupportLine()).toBe("Acme Corp Support 24x7");
    expect(hasConfiguredReportCompanyName()).toBe(true);
  });

  it("has no logo when neither logo is set", () => {
    expect(reportLogoUrl()).toBeUndefined();
  });

  it("prefers the light-mode logo, the reports being printed on white", () => {
    setConfig({
      CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL: "https://cdn.test/light.svg",
      CUSTOMER_PORTAL_LOGO_DARK_MODE_URL: "https://cdn.test/dark.svg",
    });
    expect(reportLogoUrl()).toBe("https://cdn.test/light.svg");
  });

  it("falls back to the dark-mode logo when only that is set", () => {
    setConfig({ CUSTOMER_PORTAL_LOGO_DARK_MODE_URL: " /logo-white.svg " });
    expect(reportLogoUrl()).toBe("/logo-white.svg");
  });
});
