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
import ContactRow from "../ContactRow";
import { User } from "@wso2/oxygen-ui-icons-react";
import type { Contact } from "@/constants/projectDetailsConstants";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, sx }: any) => {
    // Pass relevant styles to DOM for testing
    return (
      <div data-testid="box" data-sx={JSON.stringify(sx)}>
        {children}
      </div>
    );
  },
  Typography: ({ children }: any) => <span>{children}</span>,
  Divider: () => <hr />,
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Mail: () => <svg data-testid="icon-mail" />,
  User: () => <svg data-testid="icon-user" />,
}));

describe("ContactRow", () => {
  const mockContact: Contact = {
    role: "Test Role",
    email: "test@example.com",
    icon: <User />,
    bgColor: "#123456",
  };

  it("should render contact role and email correctly", () => {
    render(<ContactRow contact={mockContact} />);

    expect(screen.getByText("Test Role")).toBeInTheDocument();
    expect(screen.getByText("test@example.com")).toBeInTheDocument();
  });

  it("should render the mailto link correctly", () => {
    render(<ContactRow contact={mockContact} />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "mailto:test@example.com");
  });

  it("should render the contact icon", () => {
    render(<ContactRow contact={mockContact} />);
    expect(screen.getByTestId("icon-user")).toBeInTheDocument();
  });
});
