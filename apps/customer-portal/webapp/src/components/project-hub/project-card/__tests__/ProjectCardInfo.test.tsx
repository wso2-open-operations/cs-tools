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
import ProjectCardInfo from "@components/project-hub/project-card/ProjectCardInfo";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Form: {
    CardHeader: ({ title, subheader }: any) => (
      <div data-testid="card-header">
        <div data-testid="title">{title}</div>
        <div data-testid="subheader">{subheader}</div>
      </div>
    ),
  },
  Typography: ({ children, variant, sx }: any) => (
    <span data-testid={`typography-${variant}`} style={sx}>
      {children}
    </span>
  ),
}));

describe("ProjectCardInfo", () => {
  it("should strip HTML tags from subtitle", () => {
    const props = {
      title: "Test Project",
      subtitle: "<p>This is a test subtitle</p>",
    };

    render(<ProjectCardInfo {...props} />);

    expect(screen.getByText("This is a test subtitle")).toBeInTheDocument();
  });

  it("should use correct Typography variants", () => {
    const props = {
      title: "Title",
      subtitle: "Subtitle",
    };

    render(<ProjectCardInfo {...props} />);

    expect(screen.getByTestId("typography-h6")).toBeInTheDocument();
    expect(screen.getByTestId("typography-body2")).toBeInTheDocument();
  });

  it("should display '--' fallback for empty title and subtitle", () => {
    const props = {
      title: "",
      subtitle: "",
    };

    render(<ProjectCardInfo {...props} />);

    expect(screen.getAllByText("--")).toHaveLength(2);
  });
});
