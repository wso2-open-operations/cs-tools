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
import "@testing-library/jest-dom/vitest";
import CommentCardHeader from "@features/csm-cases/components/CommentCardHeader";

describe("CommentCardHeader", () => {
  it("renders the header content and the actions", () => {
    render(
      <CommentCardHeader actions={<button type="button">Act</button>}>
        <span>Jane Doe</span>
        <span>5h ago</span>
      </CommentCardHeader>,
    );
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("5h ago")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Act" })).toBeInTheDocument();
  });

  it("keeps the actions in their own column, apart from the wrapping content, and after it", () => {
    render(
      <CommentCardHeader
        actions={
          <>
            <button type="button">First</button>
            <button type="button">Second</button>
          </>
        }
      >
        <span>Jane Doe</span>
      </CommentCardHeader>,
    );
    const content = screen.getByText("Jane Doe").parentElement as HTMLElement;
    const first = screen.getByRole("button", { name: "First" });
    const second = screen.getByRole("button", { name: "Second" });

    // Content wraps on its own; the actions are never inside it, so a long
    // header cannot push them onto a line of their own.
    expect(content).not.toContainElement(first);
    expect(first.parentElement).toBe(second.parentElement);
    expect(
      content.compareDocumentPosition(first) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // Both columns share one row.
    expect(content.parentElement).toBe(first.parentElement?.parentElement);
  });

  it("lets a long unbroken token wrap, so it cannot run under the actions", () => {
    render(
      <CommentCardHeader actions={<button type="button">Act</button>}>
        <span>Averyveryverylongunbrokenname1234567890</span>
      </CommentCardHeader>,
    );
    const content = screen.getByText("Averyveryverylongunbrokenname1234567890")
      .parentElement as HTMLElement;
    expect(content).toHaveStyle({ overflowWrap: "anywhere" });
  });
});
