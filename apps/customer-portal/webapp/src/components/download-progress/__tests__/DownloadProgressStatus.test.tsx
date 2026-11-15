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
import DownloadProgressStatus from "@components/download-progress/DownloadProgressStatus";

describe("DownloadProgressStatus", () => {
  it("is a 1px region, so it never widens the page around the download button", () => {
    render(<DownloadProgressStatus name="log.txt" downloading progress={null} />);

    expect(screen.getByRole("status")).toHaveStyle({
      position: "absolute",
      width: "1px",
      height: "1px",
      overflow: "hidden",
    });
  });

  it("announces the download in steps of 10%", () => {
    const { rerender } = render(
      <DownloadProgressStatus name="log.txt" downloading progress={null} />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Downloading log.txt");

    rerender(<DownloadProgressStatus name="log.txt" downloading progress={47} />);
    expect(screen.getByRole("status")).toHaveTextContent("Downloading log.txt: 40%");
  });

  it("stays mounted and empty while nothing downloads", () => {
    render(<DownloadProgressStatus name="log.txt" downloading={false} progress={null} />);

    expect(screen.getByRole("status")).toBeEmptyDOMElement();
  });
});
