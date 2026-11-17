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
import { describe, expect, it } from "vitest";
import Footer from "@components/footer/Footer";
import {
  COMPANY_NAME,
  PRIVACY_POLICY_URL,
  TERMS_OF_SERVICE_URL,
} from "@features/project-hub/constants/appLayoutConstants";

// Footer is a compound component (Footer.Copyright / Footer.Link), so render the
// real Oxygen UI implementation instead of a flat mock.

describe("Footer", () => {
  it("should render the company name", () => {
    render(<Footer />);
    expect(screen.getByText(new RegExp(COMPANY_NAME, "i"))).toBeDefined();
    expect(
      screen.getByText(new RegExp(`${new Date().getFullYear()}`)),
    ).toBeDefined();
  });

  it("should render the terms of service link with correct URL", () => {
    render(<Footer />);
    const termsLink = screen.getByRole("link", { name: "Terms & Conditions" });
    expect(termsLink).toHaveAttribute("href", TERMS_OF_SERVICE_URL);
  });

  it("should render the privacy policy link with correct URL", () => {
    render(<Footer />);
    const privacyLink = screen.getByRole("link", { name: "Privacy Policy" });
    expect(privacyLink).toHaveAttribute("href", PRIVACY_POLICY_URL);
  });
});
