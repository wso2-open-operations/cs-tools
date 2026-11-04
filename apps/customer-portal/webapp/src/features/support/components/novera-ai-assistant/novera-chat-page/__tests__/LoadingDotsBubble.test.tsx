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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import LoadingDotsBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/LoadingDotsBubble";

describe("LoadingDotsBubble", () => {
  it("should render loading dots container with bot icon", () => {
    const { container } = render(
      <ThemeProvider theme={createTheme()}>
        <LoadingDotsBubble />
      </ThemeProvider>,
    );
    // No paper bubble wrapper any more: a polite status region with the Novera
    // header (bot icon + label) above the pulsing dots.
    expect(container.querySelector(".MuiPaper-root")).toBeNull();
    expect(
      screen.getByRole("status", { name: "Novera is preparing a response" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Novera")).toBeInTheDocument();
    expect(container.querySelector("svg")).toBeTruthy();
  });
});
