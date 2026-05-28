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
import Error404Page from "@components/error/Error404Page";

vi.mock("@api/useAttachmentPreview", () => ({
  useAttachmentPreview: () => ({ data: null, isLoading: false }),
}));

describe("Error404Page", () => {
  it("renders", () => {
    render(<Error404Page />);
    expect(screen.getByAltText("404 not found illustration")).toBeInTheDocument();
  });
});
