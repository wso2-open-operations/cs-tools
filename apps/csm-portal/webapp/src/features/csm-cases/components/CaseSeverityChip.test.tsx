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
import "@testing-library/jest-dom/vitest";
import CaseSeverityChip from "@features/csm-cases/components/CaseSeverityChip";

describe("CaseSeverityChip", () => {
  it("renders a real severity for a service request", () => {
    render(<CaseSeverityChip caseType="service_request" severity="S3" />);
    expect(screen.getByText("S3")).toBeInTheDocument();
  });

  it("never renders the Unset badge for a service request", () => {
    render(<CaseSeverityChip caseType="service_request" severity="unset" />);
    expect(screen.queryByText("Unset")).not.toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("renders nothing for a service request with no severity when the fallback is null", () => {
    const { container } = render(
      <CaseSeverityChip caseType="service_request" severity="unset" emptyFallback={null} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("keeps the Unset badge for a support case with no severity", () => {
    render(<CaseSeverityChip caseType="case" severity="unset" />);
    expect(screen.getByText("Unset")).toBeInTheDocument();
  });

  it("treats a legacy row with no type as a support case", () => {
    render(<CaseSeverityChip caseType={undefined} severity="S1" />);
    expect(screen.getByText("S1")).toBeInTheDocument();
  });

  it("renders a dash for types with no severity concept", () => {
    render(<CaseSeverityChip caseType="engagement" severity="S1" />);
    expect(screen.queryByText("S1")).not.toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
