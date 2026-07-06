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
import AllConversationsList from "../AllConversationsList";

describe("AllConversationsList", () => {
  it("renders conversation and triggers click callback", () => {
    const onConversationClick = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <AllConversationsList
          conversations={[
            {
              id: "c-1",
              number: "CH-1",
              state: { label: "Open" },
              initialMessage: "Need help",
              createdOn: "2026-01-01T10:00:00Z",
              messageCount: 2,
              createdBy: "alex",
            } as never,
          ]}
          isLoading={false}
          onConversationClick={onConversationClick}
        />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText("CH-1"));
    expect(onConversationClick).toHaveBeenCalledTimes(1);
  });
});
