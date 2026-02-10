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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import RequestCard from "@components/support/request-cards/RequestCard";

const MockIcon = () => <span data-testid="mock-icon">Icon</span>;

// Mock oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, component, ...props }: any) => (
    <div data-testid="box" data-component={component} {...props}>
      {children}
    </div>
  ),
  Button: ({ children, onClick, endIcon, startIcon }: any) => (
    <button onClick={onClick}>
      {startIcon} {children} {endIcon}
    </button>
  ),
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  alpha: (color: string) => color,
  useTheme: () => ({
    palette: {
      info: { light: "#03A9F4", main: "#0288D1" },
      grey: { 300: "#E0E0E0" },
    },
  }),
}));

describe("RequestCard", () => {
  it("should render title, subtitle, info box and buttons", () => {
    render(
      <RequestCard
        title="Test Card"
        subtitle="Test subtitle"
        icon={MockIcon}
        paletteKey="info"
        accentColor="#1976D2"
        infoBoxTitle="What is this?"
        infoBoxDescription="A test description."
        bulletItems={["Item one", "Item two"]}
        secondaryButtonLabel="View All"
        onSecondaryClick={vi.fn()}
      />,
    );

    expect(screen.getByText("Test Card")).toBeInTheDocument();
    expect(screen.getByText("Test subtitle")).toBeInTheDocument();
    expect(screen.getByText("What is this?")).toBeInTheDocument();
    expect(screen.getByText("A test description.")).toBeInTheDocument();
    expect(screen.getByText("Item one")).toBeInTheDocument();
    expect(screen.getByText("Item two")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /view all/i }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("mock-icon")).toBeInTheDocument();
  });

  it("should render primary button when provided", () => {
    render(
      <RequestCard
        title="Test"
        subtitle="Sub"
        icon={MockIcon}
        paletteKey="info"
        accentColor="#1976D2"
        infoBoxTitle="Title"
        infoBoxDescription="Desc"
        bulletItems={[]}
        secondaryButtonLabel="Secondary"
        onSecondaryClick={vi.fn()}
        primaryButton={{
          label: "Create New",
          onClick: vi.fn(),
          icon: MockIcon,
        }}
      />,
    );

    expect(
      screen.getByRole("button", { name: /create new/i }),
    ).toBeInTheDocument();
  });

  it("should call onSecondaryClick when secondary button is clicked", () => {
    const onSecondaryClick = vi.fn();
    render(
      <RequestCard
        title="Test"
        subtitle="Sub"
        icon={MockIcon}
        paletteKey="info"
        accentColor="#1976D2"
        infoBoxTitle="Title"
        infoBoxDescription="Desc"
        bulletItems={[]}
        secondaryButtonLabel="View All"
        onSecondaryClick={onSecondaryClick}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /view all/i }));
    expect(onSecondaryClick).toHaveBeenCalledTimes(1);
  });

  it("should call primaryButton.onClick when primary button is clicked", () => {
    const onPrimaryClick = vi.fn();
    render(
      <RequestCard
        title="Test"
        subtitle="Sub"
        icon={MockIcon}
        paletteKey="info"
        accentColor="#1976D2"
        infoBoxTitle="Title"
        infoBoxDescription="Desc"
        bulletItems={[]}
        secondaryButtonLabel="Secondary"
        onSecondaryClick={vi.fn()}
        primaryButton={{
          label: "Create",
          onClick: onPrimaryClick,
          icon: MockIcon,
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    expect(onPrimaryClick).toHaveBeenCalledTimes(1);
  });
});
