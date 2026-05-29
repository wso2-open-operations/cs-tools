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
import { describe, expect, it, vi } from "vitest";
import EditCaseAttachmentModal from "../EditCaseAttachmentModal";

const mutateAsync = vi.fn().mockResolvedValue(undefined);
vi.mock("@features/support/api/usePatchCaseAttachment", () => ({
  usePatchCaseAttachment: () => ({ isPending: false, mutateAsync }),
}));

describe("EditCaseAttachmentModal", () => {
  it("submits attachment rename", async () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <EditCaseAttachmentModal
          open
          attachment={{ id: "a1", name: "old.txt" } as never}
          caseId="case-1"
          onClose={vi.fn()}
        />
      </ThemeProvider>,
    );
    fireEvent.change(screen.getByLabelText(/attachment name/i), {
      target: { value: "new.txt" },
    });
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    expect(mutateAsync).toHaveBeenCalledWith({
      caseId: "case-1",
      attachmentId: "a1",
      body: { name: "new.txt" },
    });
  });
});
