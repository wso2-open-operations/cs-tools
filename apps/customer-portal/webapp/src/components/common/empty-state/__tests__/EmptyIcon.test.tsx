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
import EmptyIcon from "@components/common/empty-state/EmptyIcon";

describe("EmptyIcon", () => {
  it("should render the empty state SVG", () => {
    const { container } = render(<EmptyIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute("viewBox", "0 0 268 229");
    expect(svg).toHaveAttribute("aria-hidden", "true");
  });

  it("should accept and pass through SVG props", () => {
    render(<EmptyIcon data-testid="custom-empty-icon" />);

    expect(screen.getByTestId("custom-empty-icon")).toBeInTheDocument();
  });
});
