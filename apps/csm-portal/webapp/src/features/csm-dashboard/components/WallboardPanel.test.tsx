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
import { Server } from "@wso2/oxygen-ui-icons-react";
import WallboardPanel from "@features/csm-dashboard/components/WallboardPanel";

describe("WallboardPanel", () => {
  it("renders its title and children", () => {
    render(
      <WallboardPanel section="sre" title="Site Reliability Engineering (SRE)" icon={Server}>
        <div>panel body</div>
      </WallboardPanel>,
    );

    expect(screen.getByText("Site Reliability Engineering (SRE)")).toBeInTheDocument();
    expect(screen.getByText("panel body")).toBeInTheDocument();
  });

  it("renders a distinct panel per section without crashing (cre/sre/security/fde)", () => {
    for (const section of ["cre", "sre", "security", "fde"] as const) {
      const { unmount } = render(
        <WallboardPanel section={section} title={section} icon={Server}>
          <div>body</div>
        </WallboardPanel>,
      );
      expect(screen.getByText(section)).toBeInTheDocument();
      unmount();
    }
  });
});
