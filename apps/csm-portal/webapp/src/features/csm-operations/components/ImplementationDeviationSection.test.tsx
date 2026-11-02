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
import ImplementationDeviationSection from "./ImplementationDeviationSection";

describe("ImplementationDeviationSection", () => {
  it.each([undefined, null, "", "<p><br></p>", "   "])("renders nothing for %j", (html) => {
    const { container } = render(<ImplementationDeviationSection html={html} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders a named section with the sanitized deviation", () => {
    render(
      <ImplementationDeviationSection html={'<p>Restarted the nodes one at a time.</p><img src="x" onerror="alert(1)">'} />,
    );
    const section = screen.getByRole("region", { name: "Deviation from the implementation plan" });
    expect(section).toHaveTextContent("Restarted the nodes one at a time.");
    expect(section.innerHTML).not.toContain("onerror");
  });
});
