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
import ProjectInformationCard from "@features/project-details/components/ProjectInformationCard";

describe("ProjectInformationCard", () => {
  it("renders project name", () => {
    render(
      <ProjectInformationCard
        project={{
          key: "DEMO",
          name: "Demo Project",
          createdOn: "2024-01-15",
          type: { id: "1", label: "Enterprise" },
        } as never}
        slaStatus="Good"
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Demo Project")).toBeInTheDocument();
  });
});
