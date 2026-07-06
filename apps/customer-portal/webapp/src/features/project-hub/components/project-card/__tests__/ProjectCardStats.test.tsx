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
import ProjectCardStats from "@features/project-hub/components/project-card/ProjectCardStats";

describe("ProjectCardStats", () => {
  it("renders outstanding and active chat counts", () => {
    render(
      <ProjectCardStats
        date="2026-01-01"
        outstandingCount={10}
        activeChatsCount={5}
        actionRequiredCount={2}
      />,
    );

    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("Action Required")).toBeInTheDocument();
    expect(screen.getByText("Active Chats")).toBeInTheDocument();
  });

  it("renders error indicators when isError is true", () => {
    render(
      <ProjectCardStats
        date="2026-01-01"
        outstandingCount={0}
        activeChatsCount={0}
        actionRequiredCount={0}
        isError
      />,
    );

    expect(screen.getAllByTestId("error-indicator").length).toBeGreaterThan(0);
  });

  it("renders skeletons when isLoading is true", () => {
    const { container } = render(
      <ProjectCardStats
        date="2026-01-01"
        outstandingCount={0}
        activeChatsCount={0}
        actionRequiredCount={0}
        isLoading
      />,
    );

    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });
});
