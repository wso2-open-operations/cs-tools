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

import { Button } from "@wso2/oxygen-ui";
import { Download } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";

interface ExportPdfButtonProps {
  /** Builds and downloads the report. Async since callers dynamically
   * `import()` their report generator (keeps `jspdf` out of the main bundle —
   * same convention as `features/updates/utils/updateReportPdf.ts`). Any
   * rejection (e.g. the chunk fetch failing offline) should be caught and
   * surfaced by the caller (e.g. via `useErrorBanner`'s `showError`) — this
   * button only tracks the pending state, it doesn't swallow the error. */
  onExport: () => Promise<void>;
  disabled?: boolean;
}

/**
 * "Export as PDF" — builds an actual formatted report document (case/incident/
 * change-request details + full comment trail) via `jsPDF`, downloaded
 * client-side. Distinct from the browser's native Ctrl+P/Cmd+P print (which
 * still works as its own, separately-fixed mechanism — see
 * `src/styles/print.css`): that renders the on-screen UI; this produces a
 * standalone document meant to be shared with a customer or kept as
 * compliance evidence.
 *
 * `csm-print-hide` on the button itself, same as every other piece of
 * screen-only chrome, so it never appears in a native print/save-as-PDF.
 */
export default function ExportPdfButton({
  onExport,
  disabled = false,
}: ExportPdfButtonProps): JSX.Element {
  const [isExporting, setIsExporting] = useState(false);

  const handleClick = async (): Promise<void> => {
    setIsExporting(true);
    try {
      await onExport();
    } finally {
      setIsExporting(false);
    }
  };

  return (
    <Button
      className="csm-print-hide"
      variant="text"
      size="small"
      startIcon={<Download size={16} />}
      onClick={() => void handleClick()}
      disabled={disabled || isExporting}
    >
      {isExporting ? "Exporting…" : "Export as PDF"}
    </Button>
  );
}
