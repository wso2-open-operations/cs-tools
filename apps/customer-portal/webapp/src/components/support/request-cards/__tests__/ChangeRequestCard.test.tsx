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
import ChangeRequestCard from "@components/support/request-cards/ChangeRequestCard";
import { CHANGE_REQUEST_BULLET_ITEMS } from "@constants/supportConstants";

// Mock the child component to isolate the test
vi.mock("@components/support/request-cards/RequestCard", () => ({
  default: ({
    title,
    subtitle,
    bulletItems,
  }: {
    title: string;
    subtitle: string;
    bulletItems: string[];
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
    </div>
  ),
}));

describe("ChangeRequestCard", () => {
  it("should render with correct props", () => {
    render(
      <MemoryRouter>
        <ChangeRequestCard />
      </MemoryRouter>,
    );

    expect(screen.getByTestId("mock-request-card")).toBeInTheDocument();
    expect(screen.getByTestId("card-title")).toHaveTextContent(
      "Change Requests",
    );
    expect(screen.getByTestId("card-subtitle")).toHaveTextContent(
      "Track infrastructure changes",
    );

    const items = screen.getAllByTestId("card-item");
    expect(items).toHaveLength(CHANGE_REQUEST_BULLET_ITEMS.length);
    expect(items[0]).toHaveTextContent(CHANGE_REQUEST_BULLET_ITEMS[0]);
  });
});
