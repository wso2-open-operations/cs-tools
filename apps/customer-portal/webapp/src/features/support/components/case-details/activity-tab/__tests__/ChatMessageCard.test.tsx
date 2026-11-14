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
import { describe, expect, it} from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ChatMessageCard from "@case-details-activity/ChatMessageCard";

function renderCard(props: {
  htmlContent?: string;
  isCurrentUser?: boolean;
  primaryBg?: string;
} = {}) {
  const defaults = {
    htmlContent: "<p>Short message</p>",
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

  it("should not show expand button even for long content", () => {
    const longText = "a".repeat(250);
    renderCard({ htmlContent: `<p>${longText}</p>` });
    expect(
      screen.queryByRole("button", { name: /show more/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /show less/i }),
    ).not.toBeInTheDocument();
  });
  describe("nested list wrapper", () => {
    // The CSM portal editor exports a nested list as a wrapper <li> that only
    // holds the sub-list; old comments carry just the class, so the card hides
    // its marker (otherwise an empty bullet shows before the sub-list).
    const NESTED =
      '<ul><li value="1">Parent</li><li value="2" class="editor-list-item-nested"><ul><li value="1">Child</li></ul></li></ul>';

    it("hides the marker of the nested-list wrapper li", () => {
      const { container } = renderCard({ htmlContent: NESTED });
      const wrapper = container.querySelector("li.editor-list-item-nested");
      expect(wrapper).not.toBeNull();
      expect(getComputedStyle(wrapper as Element).listStyleType).toBe("none");
    });

    it("keeps the bullet on a normal li", () => {
      const { container } = renderCard({ htmlContent: NESTED });
      const normal = container.querySelector("li:not(.editor-list-item-nested)");
      expect(normal).not.toBeNull();
      expect(getComputedStyle(normal as Element).listStyleType).not.toBe("none");
    });
  });
});
