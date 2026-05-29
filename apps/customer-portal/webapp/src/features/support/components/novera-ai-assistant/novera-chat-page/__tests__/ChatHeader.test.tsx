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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ChatHeader from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatHeader";

describe("ChatHeader", () => {
  it("should call onBack when back button is clicked", () => {
    const onBackMock = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <ChatHeader onBack={onBackMock} />
      </ThemeProvider>,
    );

    const backButton = screen.getByRole("button", { name: /^Back$/i });
    fireEvent.click(backButton);

    expect(onBackMock).toHaveBeenCalled();
  });
});
