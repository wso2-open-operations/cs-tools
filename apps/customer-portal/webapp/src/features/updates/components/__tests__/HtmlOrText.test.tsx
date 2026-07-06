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
import { HtmlOrText } from "@features/updates/components/HtmlOrText";

describe("HtmlOrText", () => {
  it("renders formatted html content", () => {
    const { container } = render(<HtmlOrText content="<p><strong>Hello</strong></p>" isDark={false} />);
    expect(container.querySelector("strong")?.textContent).toBe("Hello");
  });

  it("renders plain escaped text safely", () => {
    render(<HtmlOrText content={"<Product_Home>\nline2"} isDark={false} />);
    expect(screen.getByText(/<Product_Home>/)).toBeInTheDocument();
    expect(screen.getByText(/line2/)).toBeInTheDocument();
  });
});

