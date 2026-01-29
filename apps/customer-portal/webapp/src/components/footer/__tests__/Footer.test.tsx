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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import Footer from "@/components/footer/Footer";
import {
  COMPANY_NAME,
  PRIVACY_POLICY_URL,
  TERMS_OF_SERVICE_URL,
} from "@/constants/appLayoutConstants";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Footer: ({
    companyName,
    termsUrl,
    privacyUrl,
  }: {
    companyName: string;
    termsUrl: string;
    privacyUrl: string;
  }) => (
    <div data-testid="footer-ui">
      <span>{companyName}</span>
      <a href={termsUrl}>Terms of Service</a>
      <a href={privacyUrl}>Privacy Policy</a>
    </div>
  ),
}));

describe("Footer", () => {
  it("should render the company name", () => {
    render(<Footer />);
    expect(screen.getByText(new RegExp(COMPANY_NAME, "i"))).toBeDefined();
  });

  it("should render the terms of service link with correct URL", () => {
    render(<Footer />);
    const termsLink = screen.getByRole("link", { name: /terms/i });
    expect(termsLink).toHaveAttribute("href", TERMS_OF_SERVICE_URL);
  });

  it("should render the privacy policy link with correct URL", () => {
    render(<Footer />);
    const privacyLink = screen.getByRole("link", { name: /privacy/i });
    expect(privacyLink).toHaveAttribute("href", PRIVACY_POLICY_URL);
  });
});
