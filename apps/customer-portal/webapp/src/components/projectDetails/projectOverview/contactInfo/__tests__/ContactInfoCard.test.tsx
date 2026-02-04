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
import ContactInfoCard from "../ContactInfoCard";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Card: ({ children }: any) => <div>{children}</div>,
  CardContent: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Divider: () => <hr data-testid="divider" />,
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Users: () => <svg data-testid="icon-users" />,
  Mail: () => <svg data-testid="icon-mail" />,
}));

// Mock constants
vi.mock("@/constants/projectDetailsConstants", () => ({
  contacts: [
    {
      role: "Role 1",
      email: "email1@example.com",
      icon: "icon-1",
      bgColor: "#000",
    },
    {
      role: "Role 2",
      email: "email2@example.com",
      icon: "icon-2",
      bgColor: "#fff",
    },
  ],
}));

// Mock ContactRow to avoid testing implementation details of child
vi.mock("../ContactRow", () => ({
  default: ({ contact }: any) => (
    <div data-testid="contact-row">
      {contact.role} - {contact.email}
    </div>
  ),
}));

describe("ContactInfoCard", () => {
  it("should render card title 'Contact Information'", () => {
    render(<ContactInfoCard />);
    expect(screen.getByText("Contact Information")).toBeInTheDocument();
  });

  it("should render correct number of contact rows", () => {
    render(<ContactInfoCard />);
    const rows = screen.getAllByTestId("contact-row");
    expect(rows).toHaveLength(2);
  });

  it("should render correct contact details in rows", () => {
    render(<ContactInfoCard />);
    expect(screen.getByText("Role 1 - email1@example.com")).toBeInTheDocument();
    expect(screen.getByText("Role 2 - email2@example.com")).toBeInTheDocument();
  });
});
