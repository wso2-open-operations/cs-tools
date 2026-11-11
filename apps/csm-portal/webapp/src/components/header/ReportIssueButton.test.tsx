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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import ReportIssueButton from "@components/header/ReportIssueButton";

afterEach(() => {
  window.config = {} as typeof window.config;
});

describe("ReportIssueButton", () => {
  it("renders nothing when CSM_PORTAL_REPORT_ISSUE_URL isn't configured", () => {
    window.config = {} as typeof window.config;
    const { container } = render(<ReportIssueButton />);
    expect(container).toBeEmptyDOMElement();
  });

  it("opens the configured URL in a new tab when clicked", () => {
    window.config = {
      CSM_PORTAL_REPORT_ISSUE_URL: "https://github.com/example/example-repo/issues/new",
    } as typeof window.config;
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    render(<ReportIssueButton />);
    fireEvent.click(screen.getByRole("button", { name: /report an issue/i }));

    expect(openSpy).toHaveBeenCalledWith(
      "https://github.com/example/example-repo/issues/new",
      "_blank",
      "noopener,noreferrer",
    );
    openSpy.mockRestore();
  });
});
