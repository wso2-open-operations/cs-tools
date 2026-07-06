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
import { describe, expect, it } from "vitest";
import {
  ErrorPageProvider,
  useErrorPageContext,
} from "@context/error-page/ErrorPageContext";

function Consumer() {
  const {
    isErrorPageDisplayed,
    setIsErrorPageDisplayed,
    isProjectSuspended,
    setIsProjectSuspended,
  } = useErrorPageContext();

  return (
    <div>
      <span data-testid="error">{String(isErrorPageDisplayed)}</span>
      <span data-testid="suspended">{String(isProjectSuspended)}</span>
      <button type="button" onClick={() => setIsErrorPageDisplayed(true)}>
        Show error
      </button>
      <button type="button" onClick={() => setIsProjectSuspended(true)}>
        Suspend
      </button>
    </div>
  );
}

describe("ErrorPageContext", () => {
  it("provides error and suspension flags", () => {
    render(
      <ErrorPageProvider>
        <Consumer />
      </ErrorPageProvider>,
    );

    expect(screen.getByTestId("error")).toHaveTextContent("false");
    fireEvent.click(screen.getByText("Show error"));
    expect(screen.getByTestId("error")).toHaveTextContent("true");

    fireEvent.click(screen.getByText("Suspend"));
    expect(screen.getByTestId("suspended")).toHaveTextContent("true");
  });
});
