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
import { fetchAllCaseTimeCards } from "@features/usage-metrics/api/fetchAllCaseTimeCards";
import { downloadTimeCardsCsv, downloadTimeCardsPdf } from "@features/project-details/utils/timeCardsCsvExport";
import type { CaseTimeCard } from "@features/usage-metrics/types/timeTracking";
import type { TimeCardSearchRequest } from "@features/usage-metrics/types/timeTracking";

type ExportFormat = "csv" | "pdf";

export type TimeCardsCsvExportButtonProps = {
  projectId: string;
  projectName?: string;
  filters: TimeCardSearchRequest["filters"];
  prefetchedCards?: CaseTimeCard[];
  totalRecords?: number;
  disabled?: boolean;
};

/**
 * Button that exports the current time card list as CSV or PDF.
 *
 * @param {TimeCardsCsvExportButtonProps} props - Export configuration.
 * @returns {JSX.Element} Export button with CSV / PDF options.
 */
export default function TimeCardsCsvExportButton({
  projectId,
  projectName,
  filters,
  prefetchedCards = [],
  totalRecords = 0,
  disabled = false,
}: TimeCardsCsvExportButtonProps): JSX.Element {
  const authFetch = useAuthApiClient();
  const { showError } = useErrorBanner();
  const [exportingFormat, setExportingFormat] = useState<ExportFormat | null>(null);
  const isExportingRef = useRef(false);
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

  const isExporting = exportingFormat !== null;
  const filenamePrefix = projectName?.trim()
    ? projectName.trim().replace(/\s+/g, "-").toLowerCase()
    : projectId;

  const handleOpen = (event: MouseEvent<HTMLElement>) => {
    if (!isExporting) setAnchorEl(event.currentTarget);
  };

  const handleClose = () => setAnchorEl(null);

  const handleExport = useCallback(
    async (format: ExportFormat) => {
      if (!projectId || isExportingRef.current) return;
      handleClose();
      isExportingRef.current = true;
      setExportingFormat(format);
      try {
        const needsFullFetch = totalRecords > 0 && prefetchedCards.length < totalRecords;
        const cards =
          needsFullFetch || prefetchedCards.length === 0
            ? await fetchAllCaseTimeCards(authFetch, projectId, filters)
            : prefetchedCards;

        if (cards.length === 0) {
          showError("No time cards to export for the current filters.");
          return;
        }

        if (format === "csv") {
          downloadTimeCardsCsv(cards, filenamePrefix);
        } else {
          downloadTimeCardsPdf(cards, filenamePrefix, projectName);
        }
      } catch (error) {
        showError(error instanceof Error ? error.message : "Failed to export time cards.");
      } finally {
        isExportingRef.current = false;
        setExportingFormat(null);
      }
    },
    [authFetch, filters, filenamePrefix, prefetchedCards, projectId, projectName, showError, totalRecords],
  );

  return (
    <>
      <Button
        type="button"
        variant="outlined"
        size="small"
        onClick={handleOpen}
        disabled={disabled || isExporting || !projectId}
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
        <MenuItem onClick={() => void handleExport("csv")}>Export to CSV</MenuItem>
        <MenuItem onClick={() => void handleExport("pdf")}>Export to PDF</MenuItem>
      </Menu>
    </>
  );
}
