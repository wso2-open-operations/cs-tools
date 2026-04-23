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
import Error400Page from "@components/error/Error400Page";

describe("Error400Page", () => {
  it("should render the default message and illustration", () => {
    render(<Error400Page />);

    expect(
      screen.getByAltText("400 bad request illustration"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /This request could not be processed\. Check the link or try again from the portal navigation\./,
      ),
    ).toBeInTheDocument();
  });

  it("should render a custom API message when provided", () => {
    render(<Error400Page message="Invalid case id format." />);

    expect(screen.getByText("Invalid case id format.")).toBeInTheDocument();
  });
});
