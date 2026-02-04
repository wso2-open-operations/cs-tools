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
import ProjectCardSkeleton from "@/components/projectHub/projectCard/ProjectCardSkeleton";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Card: ({ children }: any) => <div data-testid="card">{children}</div>,
  Divider: () => <hr data-testid="divider" />,
  Form: {
    CardContent: ({ children }: any) => (
      <div data-testid="card-content">{children}</div>
    ),
    CardHeader: ({ title, subheader }: any) => (
      <div data-testid="card-header">
        {title}
        {subheader}
      </div>
    ),
    CardActions: ({ children }: any) => (
      <div data-testid="card-actions">{children}</div>
    ),
  },
  Skeleton: ({ variant, width, height }: any) => (
    <div data-testid={`skeleton-${variant}`} style={{ width, height }} />
  ),
  Typography: ({ children, variant, sx }: any) => (
    <div data-testid={`typography-${variant}`} style={sx}>
      {children}
    </div>
  ),
  Stack: ({ children }: any) => <div data-testid="stack">{children}</div>,
}));

describe("ProjectCardSkeleton", () => {
  it("should render all sections of the skeleton", () => {
    render(<ProjectCardSkeleton />);

    expect(screen.getByTestId("card")).toBeInTheDocument();
    expect(screen.getByTestId("card-header")).toBeInTheDocument();
    expect(screen.getAllByTestId("card-content")).toHaveLength(2);
    expect(screen.getByTestId("card-actions")).toBeInTheDocument();
  });

  it("should render skeleton placeholders", () => {
    render(<ProjectCardSkeleton />);

    expect(screen.getAllByTestId("skeleton-rounded")).toHaveLength(3);
    expect(screen.getAllByTestId("skeleton-text")).toHaveLength(9);
    expect(screen.getAllByTestId("skeleton-circular")).toHaveLength(3);
  });
});
