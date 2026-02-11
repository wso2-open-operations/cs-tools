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
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import Brand from "@components/common/header/Brand";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Header: {
    Brand: ({ children, onClick }: { children: any; onClick?: () => void }) => (
      <div onClick={onClick}>{children}</div>
    ),
    BrandLogo: ({ children }: { children: any }) => <div>{children}</div>,
    BrandTitle: ({ children }: { children: any }) => <h1>{children}</h1>,
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  WSO2: () => <svg data-testid="wso2-logo" />,
}));

describe("Brand", () => {
  it("should render the wso2 logo and product title", () => {
    render(
      <MemoryRouter>
        <Brand />
      </MemoryRouter>,
    );

    expect(screen.getByTestId("wso2-logo")).toBeInTheDocument();
    expect(screen.getByText("Customer Portal")).toBeInTheDocument();
  });
});
