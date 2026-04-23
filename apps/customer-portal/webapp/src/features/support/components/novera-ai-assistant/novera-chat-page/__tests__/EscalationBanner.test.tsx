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
import EscalationBanner from "@features/support/components/novera-ai-assistant/novera-chat-page/EscalationBanner";
// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Button: ({ children, onClick }: any) => (
    <button data-testid="button" onClick={onClick}>
      {children}
    </button>
  ),
  CircularProgress: () => <span data-testid="circular-progress" />,
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Stack: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
  colors: {
    orange: {
      700: "#C2410C",
    },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Sparkles: () => <svg data-testid="icon-sparkles" />,
  FileText: () => <svg data-testid="icon-file-text" />,
}));

describe("EscalationBanner", () => {
  it("should render correctly when visible", () => {
    render(<EscalationBanner visible={true} onCreateCase={vi.fn()} />);

    expect(screen.getByText(/Thank you for describing the issue/i)).toBeInTheDocument();
    expect(screen.getByText("Create Case")).toBeInTheDocument();
    expect(screen.getByTestId("icon-sparkles")).toBeInTheDocument();
  });

  it("should not render when not visible", () => {
    const { container } = render(
      <EscalationBanner visible={false} onCreateCase={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("should call onCreateCase when button is clicked", () => {
    const onCreateCase = vi.fn();
    render(<EscalationBanner visible={true} onCreateCase={onCreateCase} />);

    screen.getByText("Create Case").click();
    expect(onCreateCase).toHaveBeenCalledTimes(1);
  });
});
