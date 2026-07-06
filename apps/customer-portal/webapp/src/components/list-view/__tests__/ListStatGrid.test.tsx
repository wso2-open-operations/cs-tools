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
import ListStatGrid from "@components/list-view/ListStatGrid";
import { ALL_CASES_STAT_CONFIGS } from "@features/support/constants/supportConstants";

describe("ListStatGrid", () => {
  it("renders stat label when loaded", () => {
    const config = ALL_CASES_STAT_CONFIGS[0];
    render(
      <ListStatGrid
        isLoading={false}
        configs={[config]}
        stats={{ [config.key]: 3 }}
      />,
    );
    expect(screen.getByText(config.label)).toBeInTheDocument();
  });
});
