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
import CaseDetailsTabPanels from "@case-details/CaseDetailsTabPanels";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

function renderTabPanels(activeTab: number) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsTabPanels activeTab={activeTab} />
    </ThemeProvider>,
  );
}

describe("CaseDetailsTabPanels", () => {
  it("should show Activity placeholder when activeTab is 0", () => {
    renderTabPanels(0);
    expect(screen.getByText("Activity timeline will appear here.")).toBeInTheDocument();
  });

  it("should show Details placeholder when activeTab is 1", () => {
    renderTabPanels(1);
    expect(screen.getByText("Details appear here.")).toBeInTheDocument();
  });

  it("should show Attachments placeholder when activeTab is 2", () => {
    renderTabPanels(2);
    expect(screen.getByText("Attachments will appear here.")).toBeInTheDocument();
  });

  it("should show Calls placeholder when activeTab is 3", () => {
    renderTabPanels(3);
    expect(screen.getByText("Calls will appear here.")).toBeInTheDocument();
  });

  it("should show Knowledge Base placeholder when activeTab is 4", () => {
    renderTabPanels(4);
    expect(screen.getByText("Knowledge Base articles will appear here.")).toBeInTheDocument();
  });
});
