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
import ProjectMetadataPrimaryRow from "@features/project-details/components/project-overview/project-information/ProjectMetadataPrimaryRow";

describe("ProjectMetadataPrimaryRow", () => {
  it("renders created date and support tier labels", () => {
    render(
      <ProjectMetadataPrimaryRow
        createdDate="Jan 1, 2024"
        type={{ id: "1", label: "Subscription" }}
        supportTier="Premium"
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Created Date")).toBeInTheDocument();
    expect(screen.getByText("Premium")).toBeInTheDocument();
  });
});
