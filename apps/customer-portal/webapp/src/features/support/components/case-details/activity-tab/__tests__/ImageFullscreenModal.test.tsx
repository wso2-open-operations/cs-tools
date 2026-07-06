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
import ImageFullscreenModal from "../ImageFullscreenModal";

describe("ImageFullscreenModal", () => {
  it("renders image and closes on close button click", () => {
    const onClose = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <ImageFullscreenModal open imageSrc="https://example.com/a.png" onClose={onClose} />
      </ThemeProvider>,
    );
    expect(screen.getByRole("img", { name: /full size/i })).toHaveAttribute(
      "src",
      "https://example.com/a.png",
    );
    fireEvent.click(screen.getByRole("button", { name: /close/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
