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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it } from "vitest";
import CallRequestList from "../CallRequestList";

describe("CallRequestList", () => {
  it("renders cards for request list", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestList
          requests={[
            { id: "1", state: { label: "Scheduled" }, reason: "Discuss issue" },
            { id: "2", state: { label: "Completed" }, reason: "Follow up" },
          ] as never}
        />
      </ThemeProvider>,
    );
    expect(screen.getAllByText(/reason \/ notes/i).length).toBe(2);
    expect(screen.getByText("Discuss issue")).toBeInTheDocument();
    expect(screen.getByText("Follow up")).toBeInTheDocument();
  });
});
