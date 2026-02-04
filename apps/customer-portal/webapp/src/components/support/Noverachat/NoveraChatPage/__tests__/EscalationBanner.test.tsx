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
import EscalationBanner from "@/components/support/Noverachat/NoveraChatPage/EscalationBanner";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Button: ({ children, onClick, variant }: any) => (
    <button data-testid="button" onClick={onClick} data-variant={variant}>
      {children}
    </button>
  ),
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  CircleAlert: () => <svg data-testid="icon-alert" />,
}));

describe("EscalationBanner", () => {
  it("should render correctly when visible", () => {
    render(<EscalationBanner visible={true} />);

    expect(screen.getByText(/Need more help?/i)).toBeInTheDocument();
    expect(screen.getByText("Create Case")).toBeInTheDocument();
    expect(screen.getByTestId("icon-alert")).toBeInTheDocument();
  });

  it("should not render when not visible", () => {
    const { container } = render(<EscalationBanner visible={false} />);
    expect(container).toBeEmptyDOMElement();
  });
});
