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

import { Button, CircularProgress, Menu, MenuItem } from "@wso2/oxygen-ui";
import { ChevronDown, Download } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useRef, useState, type JSX, type MouseEvent } from "react";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { fetchProjectCaseSearchResults } from "@features/support/api/fetchProjectCaseSearchResults";
import type { CaseListItem, CaseSearchRequest } from "@features/support/types/cases";
import {
  downloadCaseListCsv,
  downloadCaseListPdf,
  type CaseListCsvExportVariant,
} from "@features/support/utils/casesCsvExport";

export type CaseListCsvExportButtonProps = {
  projectId: string;
  projectName?: string;
  caseSearchRequest: Omit<CaseSearchRequest, "pagination">;
  filenamePrefix: string;
  exportVariant?: CaseListCsvExportVariant;
  prefetchedCases?: CaseListItem[];
  totalRecords?: number;
  disabled?: boolean;
  emptyMessage?: string;
};

type ExportFormat = "csv" | "pdf";

/**
 * Button that exports the current case list search/filter result set as CSV.
 *
 * @param {CaseListCsvExportButtonProps} props - Export configuration.
 * @returns {JSX.Element} Download Results button.
 */
export default function CaseListCsvExportButton({
  projectId,
  projectName,
  caseSearchRequest,
  filenamePrefix,
  exportVariant = "withType",
  prefetchedCases = [],
  totalRecords = 0,
  disabled = false,
  emptyMessage = "No results to export for the current search or filters.",
}: CaseListCsvExportButtonProps): JSX.Element {
  const authFetch = useAuthApiClient();
  const { showError } = useErrorBanner();
  const [exportingFormat, setExportingFormat] = useState<ExportFormat | null>(null);
  const isExportingRef = useRef(false);
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

  const isExporting = exportingFormat !== null;

  const handleOpen = (event: MouseEvent<HTMLElement>) => {
    if (!isExporting) {
      setAnchorEl(event.currentTarget);
    }
  };

  const handleClose = () => setAnchorEl(null);

  const handleExport = useCallback(
    async (format: ExportFormat) => {
      if (!projectId || isExportingRef.current) return;
      handleClose();
      isExportingRef.current = true;
      setExportingFormat(format);
      try {
        const needsFullFetch =
          totalRecords > 0 && prefetchedCases.length < totalRecords;
        const cases = needsFullFetch
          ? await fetchProjectCaseSearchResults(authFetch, projectId, caseSearchRequest)
          : prefetchedCases.length > 0
            ? prefetchedCases
            : await fetchProjectCaseSearchResults(authFetch, projectId, caseSearchRequest);

        if (cases.length === 0) {
          showError(emptyMessage);
          return;
        }

        if (format === "csv") {
          downloadCaseListCsv(cases, exportVariant, filenamePrefix, projectId, projectName);
        } else {
          downloadCaseListPdf(cases, exportVariant, filenamePrefix, projectId, projectName);
        }
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "Failed to export results.";
        showError(message);
      } finally {
        isExportingRef.current = false;
        setExportingFormat(null);
      }
    },
    [
      authFetch,
      caseSearchRequest,
      emptyMessage,
      exportVariant,
      filenamePrefix,
      prefetchedCases,
      projectId,
      projectName,
      showError,
      totalRecords,
    ],
  );

  const isDisabled = disabled || isExporting || !projectId;

  return (
    <>
      <Button
        type="button"
        variant="outlined"
        size="small"
        onClick={handleOpen}
        disabled={isDisabled}
        startIcon={
          isExporting ? (
            <CircularProgress size={16} color="inherit" />
          ) : (
            <Download size={16} />
          )
        }
        endIcon={<ChevronDown size={16} />}
      >
        {isExporting ? "Exporting..." : "Export"}
      </Button>
      <Menu anchorEl={anchorEl} open={Boolean(anchorEl)} onClose={handleClose}>
        <MenuItem onClick={() => void handleExport("csv")}>
          Export to CSV
        </MenuItem>
        <MenuItem onClick={() => void handleExport("pdf")}>
          Export to PDF
        </MenuItem>
      </Menu>
    </>
  );
}
