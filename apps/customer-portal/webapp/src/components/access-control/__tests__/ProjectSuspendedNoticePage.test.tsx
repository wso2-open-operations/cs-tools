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
import ProjectSuspendedNoticePage from "@components/access-control/ProjectSuspendedNoticePage";

vi.mock("@api/useAttachmentPreview", () => ({
  useAttachmentPreview: () => ({ data: null, isLoading: false }),
}));

const project = { id: "p", name: "Proj", closureState: "Suspended" } as never;

describe("ProjectSuspendedNoticePage", () => {
  it("renders the generic suspension notice and billing contact", () => {
    render(<ProjectSuspendedNoticePage project={project} />);
    expect(screen.getByText("Project Suspension Notice")).toBeInTheDocument();
    expect(screen.getByText(/This project has been suspended/i)).toBeInTheDocument();
    expect(screen.getByText("billing@choreolabs.com")).toHaveAttribute(
      "href",
      "mailto:billing@choreolabs.com",
    );
    // The reasons block only appears when the backend supplies reasons.
    expect(screen.queryByText(/suspended for the following reasons/i)).toBeNull();
  });

  it("lists suspension reasons when provided", () => {
    render(
      <ProjectSuspendedNoticePage
        project={
          {
            id: "p",
            name: "Proj",
            closureState: "Suspended",
            suspensionReasons: ["Payment overdue"],
          } as never
        }
      />,
    );
    expect(
      screen.getByText(/suspended for the following reasons/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/Payment overdue/)).toBeInTheDocument();
  });

  it("names WSO2 and the default billing address when nothing is configured", () => {
    render(<ProjectSuspendedNoticePage project={{ id: "p", name: "Proj", closureState: "Suspended" } as never} />);
    expect(screen.getByText(/contact your WSO2 Account\s+Manager/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "billing@choreolabs.com" })).toHaveAttribute("href", "mailto:billing@choreolabs.com");
  });

  it("names the configured company and billing address", () => {
    const originalConfig = window.config;
    window.config = {
      ...originalConfig,
      CUSTOMER_PORTAL_COMPANY_NAME: "Acme Corp",
      CUSTOMER_PORTAL_BILLING_EMAIL: "billing@acme.example",
    };
    try {
      render(<ProjectSuspendedNoticePage project={{ id: "p", name: "Proj", closureState: "Suspended" } as never} />);
      expect(screen.getByText(/contact your Acme Corp Account\s+Manager/)).toBeInTheDocument();
      expect(screen.queryByText(/WSO2 Account/)).not.toBeInTheDocument();
      expect(screen.getByRole("link", { name: "billing@acme.example" })).toHaveAttribute(
        "href",
        "mailto:billing@acme.example",
      );
    } finally {
      window.config = originalConfig;
    }
  });

  it("encodes a # in the billing link but shows the address unchanged", () => {
    const originalConfig = window.config;
    window.config = { ...originalConfig, CUSTOMER_PORTAL_BILLING_EMAIL: "billing#accounts@acme.example" };
    try {
      render(<ProjectSuspendedNoticePage project={{ id: "p", name: "Proj", closureState: "Suspended" } as never} />);
      expect(screen.getByRole("link", { name: "billing#accounts@acme.example" })).toHaveAttribute(
        "href",
        "mailto:billing%23accounts@acme.example",
      );
    } finally {
      window.config = originalConfig;
    }
  });
});
