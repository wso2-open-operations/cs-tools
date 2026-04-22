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
import Error500Page from "@components/error/Error500Page";

describe("Error500Page", () => {
  it("should render the default message and illustration", () => {
    render(<Error500Page />);

    expect(
      screen.getByAltText("500 server error illustration"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Something went wrong on our side\. Please try again in a few moments\./,
      ),
    ).toBeInTheDocument();
  });

  it("should render a custom message when provided", () => {
    render(<Error500Page message="Upstream timeout." />);

    expect(screen.getByText("Upstream timeout.")).toBeInTheDocument();
  });
});
