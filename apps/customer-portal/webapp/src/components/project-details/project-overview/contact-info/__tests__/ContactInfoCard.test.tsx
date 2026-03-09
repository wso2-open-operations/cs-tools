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
import ContactInfoCard from "@components/project-details/project-overview/contact-info/ContactInfoCard";
import type { ProjectDetails } from "@models/responses";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Card: ({ children }: any) => <div>{children}</div>,
  CardContent: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Divider: () => <hr data-testid="divider" />,
  Skeleton: ({ variant, width, height }: any) => (
    <div
      data-testid={`skeleton-${variant}`}
      data-width={width}
      data-height={height}
    />
  ),
  colors: {
    blue: { 700: "#1976d2" },
    purple: { 400: "#ab47bc" },
  },
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Users: () => <svg data-testid="icon-users" />,
  User: () => <svg data-testid="icon-user" />,
  Shield: () => <svg data-testid="icon-shield" />,
  Mail: () => <svg data-testid="icon-mail" />,
}));

// Mock ContactRow to avoid testing implementation details of child
vi.mock("../ContactRow", () => ({
  default: ({ contact }: any) => (
    <div data-testid="contact-row">
      {contact.role} - {contact.email}
    </div>
  ),
}));

// Mock ErrorIndicator
vi.mock("@components/common/error-indicator/ErrorIndicator", () => ({
  default: ({ entityName }: any) => (
    <div data-testid="error-indicator">Error loading {entityName}</div>
  ),
}));

const mockProject: ProjectDetails = {
  id: "project-1",
  name: "Test Project",
  key: "TP",
  type: { id: "subscription", label: "Subscription" },
  createdOn: "2024-01-01",
  description: "Test Description",
  hasSr: true,
  account: {
    id: "account-1",
    name: "Test Account",
    activationDate: null,
    deactivationDate: null,
    supportTier: "Enterprise",
    region: null,
    ownerEmail: "owner@example.com",
    technicalOwnerEmail: "tech@example.com",
  },
};

describe("ContactInfoCard", () => {
  it("should render card title 'Contact Information'", () => {
    render(<ContactInfoCard project={mockProject} />);
    expect(screen.getByText("Contact Information")).toBeInTheDocument();
  });

  it("should render both owner and technical owner when provided", () => {
    render(<ContactInfoCard project={mockProject} />);
    const rows = screen.getAllByTestId("contact-row");
    expect(rows).toHaveLength(2);
    expect(
      screen.getByText("Account Manager - owner@example.com"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Technical Owner - tech@example.com"),
    ).toBeInTheDocument();
  });

  it("should render only owner when technical owner is not provided", () => {
    const projectWithoutTech: ProjectDetails = {
      ...mockProject,
      account: {
        ...mockProject.account!,
        technicalOwnerEmail: null,
      },
    };
    render(<ContactInfoCard project={projectWithoutTech} />);
    const rows = screen.getAllByTestId("contact-row");
    expect(rows).toHaveLength(1);
    expect(
      screen.getByText("Account Manager - owner@example.com"),
    ).toBeInTheDocument();
  });

  it("should show 'No contact information available' when no contacts provided", () => {
    const projectWithoutContacts: ProjectDetails = {
      ...mockProject,
      account: {
        ...mockProject.account!,
        ownerEmail: null,
        technicalOwnerEmail: null,
      },
    };
    render(<ContactInfoCard project={projectWithoutContacts} />);
    expect(
      screen.getByText("No contact information available"),
    ).toBeInTheDocument();
  });

  it("should handle undefined project", () => {
    render(<ContactInfoCard />);
    expect(
      screen.getByText("No contact information available"),
    ).toBeInTheDocument();
  });

  it("should show skeleton loaders when isLoading is true", () => {
    render(<ContactInfoCard isLoading={true} />);
    const skeletons = screen.getAllByTestId(/skeleton-/);
    expect(skeletons.length).toBeGreaterThan(0);
    expect(screen.queryByTestId("contact-row")).not.toBeInTheDocument();
  });

  it("should show error indicator when isError is true", () => {
    render(<ContactInfoCard isError={true} />);
    expect(screen.getByTestId("error-indicator")).toBeInTheDocument();
    expect(
      screen.getByText("Error loading contact information"),
    ).toBeInTheDocument();
  });

  it("should render contacts when not loading and no error", () => {
    render(
      <ContactInfoCard
        project={mockProject}
        isLoading={false}
        isError={false}
      />,
    );
    const rows = screen.getAllByTestId("contact-row");
    expect(rows).toHaveLength(2);
  });
});
