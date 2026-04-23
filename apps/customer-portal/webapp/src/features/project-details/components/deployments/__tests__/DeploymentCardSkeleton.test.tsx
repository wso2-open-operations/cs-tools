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

import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import DeploymentCardSkeleton from "@features/project-details/components/deployments/DeploymentCardSkeleton";

describe("DeploymentCardSkeleton", () => {
  it("should render skeleton placeholders for deployment card structure", () => {
    const { container } = render(<DeploymentCardSkeleton />);

    // Should render skeleton elements within the card
    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render multiple skeleton text variants", () => {
    const { container } = render(<DeploymentCardSkeleton />);

    // Check for text skeleton variants
    const textSkeletons = container.querySelectorAll(".MuiSkeleton-text");
    expect(textSkeletons.length).toBeGreaterThan(0);
  });

  it("should render skeleton rounded variants for chips and buttons", () => {
    const { container } = render(<DeploymentCardSkeleton />);

    // Check for rounded skeleton variants (for chips, buttons, etc.)
    const roundedSkeletons = container.querySelectorAll(".MuiSkeleton-rounded");
    expect(roundedSkeletons.length).toBeGreaterThan(0);
  });

  it("should render skeleton circular variant for icons", () => {
    const { container } = render(<DeploymentCardSkeleton />);

    // Check for circular skeleton variants (for icons)
    const circularSkeletons = container.querySelectorAll(
      ".MuiSkeleton-circular",
    );
    expect(circularSkeletons.length).toBeGreaterThan(0);
  });

  it("should render within a Card component", () => {
    const { container } = render(<DeploymentCardSkeleton />);

    // Should be wrapped in a Card
    const card = container.querySelector(".MuiCard-root");
    expect(card).toBeInTheDocument();
  });

  it("should render within CardContent", () => {
    const { container } = render(<DeploymentCardSkeleton />);

    const cardContent = container.querySelector(".MuiCardContent-root");
    expect(cardContent).toBeInTheDocument();
  });

  it("should match the layout structure of the actual DeploymentCard", () => {
    const { container } = render(<DeploymentCardSkeleton />);

    // Should have dividers to match the actual card layout
    const dividers = container.querySelectorAll(".MuiDivider-root");
    expect(dividers.length).toBeGreaterThan(0);

    // Should have multiple product skeleton placeholders (2 as per the component)
    const productSkeletons = container.querySelectorAll(
      ".MuiSkeleton-rounded[style*='height']",
    );
    expect(productSkeletons.length).toBeGreaterThan(0);
  });
});
