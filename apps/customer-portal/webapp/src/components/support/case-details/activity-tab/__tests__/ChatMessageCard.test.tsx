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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ChatMessageCard from "@case-details-activity/ChatMessageCard";

function renderCard(props: {
  htmlContent?: string;
  isExpanded?: boolean;
  onToggleExpand?: () => void;
  isCurrentUser?: boolean;
  primaryBg?: string;
} = {}) {
  const defaults = {
    htmlContent: "<p>Short message</p>",
    isExpanded: false,
    onToggleExpand: () => {},
    isCurrentUser: false,
    primaryBg: "rgba(250,123,63,0.1)",
  };
  return render(
    <ThemeProvider theme={createTheme()}>
      <ChatMessageCard {...defaults} {...props} />
    </ThemeProvider>,
  );
}

describe("ChatMessageCard", () => {
  it("should render short content", () => {
    renderCard({ htmlContent: "<p>Hello</p>" });
    expect(screen.getByText("Hello")).toBeInTheDocument();
  });

  it("should not show expand button for short content", () => {
    renderCard({ htmlContent: "<p>Short</p>" });
    expect(screen.queryByRole("button", { name: /show more/i })).not.toBeInTheDocument();
  });

  it("should show expand button for long content", () => {
    const longText = "a".repeat(250);
    renderCard({ htmlContent: `<p>${longText}</p>` });
    expect(screen.getByRole("button", { name: /show more/i })).toBeInTheDocument();
  });

  it("should call onToggleExpand when expand button clicked", () => {
    const onToggle = vi.fn();
    const longText = "a".repeat(250);
    renderCard({
      htmlContent: `<p>${longText}</p>`,
      onToggleExpand: onToggle,
    });
    const btn = screen.getByRole("button", { name: /show more/i });
    fireEvent.click(btn);
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
