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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { ConversationSummary } from "@components/support/case-creation-layout/form-sections/conversation-summary-section/ConversationSummary";

vi.mock("@components/common/error-indicator/ErrorIndicator", () => ({
  __esModule: true,
  default: ({ entityName }: { entityName: string }) => (
    <span data-testid="error-indicator">{entityName}</span>
  ),
}));

function renderSummary(
  props: Partial<Parameters<typeof ConversationSummary>[0]> = {},
) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <ConversationSummary {...props} />
    </ThemeProvider>,
  );
}

describe("ConversationSummary", () => {
  it("should render Conversation Summary header", () => {
    renderSummary();
    expect(screen.getByText("Conversation Summary")).toBeInTheDocument();
  });

  it("should render Messages exchanged, Troubleshooting attempts, KB articles reviewed labels", () => {
    renderSummary();
    expect(screen.getAllByText("Messages exchanged").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Troubleshooting attempts").length).toBeGreaterThan(0);
    expect(screen.getAllByText("KB articles reviewed").length).toBeGreaterThan(0);
  });

  it("should render View full conversation link", () => {
    renderSummary();
    expect(screen.getByText("View full conversation")).toBeInTheDocument();
  });

  it("should render attachment tip text", () => {
    renderSummary();
    expect(
      screen.getByText(/All conversation details will be attached to your case/i),
    ).toBeInTheDocument();
  });

  it("should show error indicators when metadata is undefined", () => {
    renderSummary();
    const indicators = screen.getAllByTestId("error-indicator");
    expect(indicators.length).toBe(3);
    expect(indicators.map((el) => el.textContent)).toContain(
      "Messages exchanged",
    );
    expect(indicators.map((el) => el.textContent)).toContain(
      "Troubleshooting attempts",
    );
    expect(indicators.map((el) => el.textContent)).toContain(
      "KB articles reviewed",
    );
  });

  it("should display metadata values when provided", () => {
    renderSummary({
      metadata: {
        conversationSummary: {
          messagesExchanged: 5,
          troubleshootingAttempts: "3 steps",
          kbArticlesReviewed: "2 articles",
        },
      } as any,
    });
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("3 steps")).toBeInTheDocument();
    expect(screen.getByText("2 articles")).toBeInTheDocument();
  });

  it("should show skeletons when isLoading is true", () => {
    renderSummary({ isLoading: true });
    const skeletons = document.querySelectorAll('[class*="MuiSkeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
