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
import RecommendationsCard from "../RecommendationsCard";

describe("RecommendationsCard", () => {
  it("opens article when recommendation card is clicked", () => {
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);
    render(
      <ThemeProvider theme={createTheme()}>
        <RecommendationsCard
          recommendations={[{ articleId: "a1", title: "How to fix", score: 0.9 } as never]}
        />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByText("How to fix"));
    expect(openSpy).toHaveBeenCalled();
  });
});
