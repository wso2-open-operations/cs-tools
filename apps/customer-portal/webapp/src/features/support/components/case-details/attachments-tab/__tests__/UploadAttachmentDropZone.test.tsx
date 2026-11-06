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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { afterEach, describe, expect, it, vi } from "vitest";
import UploadAttachmentDropZone from "../UploadAttachmentDropZone";

describe("UploadAttachmentDropZone", () => {
  it("calls choose file handler when button is clicked", () => {
    const onChooseFile = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <UploadAttachmentDropZone
          dragOver={false}
          onDrop={vi.fn()}
          onDragOver={vi.fn()}
          onDragLeave={vi.fn()}
          onChooseFile={onChooseFile}
        />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: /choose file/i }));
    expect(onChooseFile).toHaveBeenCalledTimes(1);
  });

  describe("maximum file size", () => {
    afterEach(() => {
      delete window.config?.CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB;
    });

    const renderZone = () =>
      render(
        <ThemeProvider theme={createTheme()}>
          <UploadAttachmentDropZone
            dragOver={false}
            onDrop={vi.fn()}
            onDragOver={vi.fn()}
            onDragLeave={vi.fn()}
            onChooseFile={vi.fn()}
          />
        </ThemeProvider>,
      );

    it("shows the portal default when nothing is configured", () => {
      renderZone();
      expect(screen.getByText("Maximum file size: 100 MB.")).toBeInTheDocument();
    });

    it("shows the configured limit", () => {
      window.config = {
        ...window.config,
        CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 50,
      } as typeof window.config;
      renderZone();
      expect(screen.getByText("Maximum file size: 50 MB.")).toBeInTheDocument();
    });
  });
});
