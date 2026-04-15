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
import ChatHistoryList from "@features/support/components/support-overview-cards/ChatHistoryList";
import type { ChatHistoryItem } from "@features/support/types/conversations";

const mockItems: ChatHistoryItem[] = [
  {
    chatId: "1628192673",
    title: "How do I configure custom claims in JWT tokens?",
    startedTime: "2 hours ago",
    messages: 8,
    kbArticles: 3,
    status: "Resolved",
  },
  {
    chatId: "1628192674",
    title: "Getting error 401 when calling the API endpoint...",
    startedTime: "1 day ago",
    messages: 5,
    kbArticles: 2,
    status: "Still Open",
  },
];

describe("ChatHistoryList", () => {
  it("should render empty state when no items", () => {
    render(<ChatHistoryList items={[]} />);
    expect(screen.getByText("No chat history.")).toBeInTheDocument();
  });

  it("should render chat title, meta and status", () => {
    render(<ChatHistoryList items={mockItems} />);
    expect(
      screen.getByText("How do I configure custom claims in JWT tokens?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Getting error 401 when calling the API endpoint..."),
    ).toBeInTheDocument();
    expect(screen.getByText("Resolved")).toBeInTheDocument();
    expect(screen.getByText("Still Open")).toBeInTheDocument();
    expect(screen.getByText("2 hours ago")).toBeInTheDocument();
    expect(screen.getByText(/8 messages/)).toBeInTheDocument();
    expect(screen.getByText(/3 KB articles/)).toBeInTheDocument();
  });

  it("should show View for Resolved and Resume for Still Open", () => {
    render(<ChatHistoryList items={mockItems} />);
    const viewButtons = screen.getAllByRole("button", { name: /view/i });
    const resumeButtons = screen.getAllByRole("button", { name: /resume/i });
    expect(viewButtons.length).toBeGreaterThanOrEqual(1);
    expect(resumeButtons.length).toBeGreaterThanOrEqual(1);
  });

  it("should call onItemAction with chatId and action when View/Resume clicked", () => {
    const onItemAction = vi.fn();
    render(<ChatHistoryList items={mockItems} onItemAction={onItemAction} />);

    fireEvent.click(screen.getByRole("button", { name: "View" }));
    expect(onItemAction).toHaveBeenCalledWith("1628192673", "view");

    fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    expect(onItemAction).toHaveBeenCalledWith("1628192674", "resume");
  });
});
