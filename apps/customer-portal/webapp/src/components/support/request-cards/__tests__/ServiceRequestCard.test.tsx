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
import { MemoryRouter } from "react-router";
import ServiceRequestCard from "@components/support/request-cards/ServiceRequestCard";
import { SERVICE_REQUEST_BULLET_ITEMS } from "@constants/supportConstants";

// Mock the child component to isolate the test
vi.mock("@components/support/request-cards/RequestCard", () => ({
  default: ({
    title,
    subtitle,
    bulletItems,
    primaryButton,
  }: {
    title: string;
    subtitle: string;
    bulletItems: string[];
    primaryButton?: { label: string };
  }) => (
    <div data-testid="mock-request-card">
      <span data-testid="card-title">{title}</span>
      <span data-testid="card-subtitle">{subtitle}</span>
      <ul>
        {bulletItems.map((item) => (
          <li key={item} data-testid="card-item">
            {item}
          </li>
        ))}
      </ul>
      {primaryButton && <button>{primaryButton.label}</button>}
    </div>
  ),
}));

describe("ServiceRequestCard", () => {
  it("should render with correct props", () => {
    render(
      <MemoryRouter>
        <ServiceRequestCard />
      </MemoryRouter>,
    );

    expect(screen.getByTestId("mock-request-card")).toBeInTheDocument();
    expect(screen.getByTestId("card-title")).toHaveTextContent(
      "Service Requests",
    );
    expect(screen.getByTestId("card-subtitle")).toHaveTextContent(
      "Manage deployment operations",
    );
    expect(screen.getByText("New Service Request")).toBeInTheDocument();

    const items = screen.getAllByTestId("card-item");
    expect(items).toHaveLength(SERVICE_REQUEST_BULLET_ITEMS.length);
    expect(items[0]).toHaveTextContent(SERVICE_REQUEST_BULLET_ITEMS[0]);
  });
});
