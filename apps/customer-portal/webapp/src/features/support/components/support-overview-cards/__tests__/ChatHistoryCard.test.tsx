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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it, vi } from "vitest";
import ChatHistoryCard from "../ChatHistoryCard";

describe("ChatHistoryCard", () => {
  it("calls action callback when card action button clicked", () => {
    const onItemAction = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <ChatHistoryCard
          item={
            {
              chatId: "chat-1",
              title: "Chat title",
              status: "Open",
              startedTime: "2026-01-01T00:00:00Z",
              messages: 3,
              kbArticles: 1,
            } as never
          }
          onItemAction={onItemAction}
        />
      </ThemeProvider>,
    );
    const actionButton = screen
      .getAllByRole("button", { name: /view|resume/i })
      .find((element) => element.tagName.toLowerCase() === "span");
    expect(actionButton).toBeDefined();
    fireEvent.click(actionButton as Element);
    expect(onItemAction).toHaveBeenCalled();
  });
});
