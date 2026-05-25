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

import { Button, CircularProgress } from "@wso2/oxygen-ui";
import { Download } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useRef, useState, type JSX } from "react";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { fetchChangeRequestSearchResults } from "@features/operations/api/fetchChangeRequestSearchResults";
import type {
  ChangeRequestItem,
  ChangeRequestSearchRequest,
} from "@features/operations/types/changeRequests";
import { downloadChangeRequestsCsv } from "@features/operations/utils/changeRequestsCsvExport";

export type ChangeRequestsCsvExportButtonProps = {
  projectId: string;
  searchRequest: Omit<ChangeRequestSearchRequest, "pagination">;
  prefetchedItems?: ChangeRequestItem[];
  totalRecords?: number;
  disabled?: boolean;
};

/**
 * Download Results button for the change requests list.
 *
 * @param {ChangeRequestsCsvExportButtonProps} props - Export configuration.
 * @returns {JSX.Element} Download Results button.
 */
export default function ChangeRequestsCsvExportButton({
  projectId,
  searchRequest,
  prefetchedItems = [],
  totalRecords = 0,
  disabled = false,
}: ChangeRequestsCsvExportButtonProps): JSX.Element {
  const authFetch = useAuthApiClient();
  const { showError } = useErrorBanner();
  const [isExporting, setIsExporting] = useState(false);
  const isExportingRef = useRef(false);

  const handleDownload = useCallback(async () => {
    if (!projectId || isExportingRef.current) {
      return;
    }
    isExportingRef.current = true;
    setIsExporting(true);
    try {
      const needsFullFetch =
        totalRecords > 0 && prefetchedItems.length < totalRecords;

      const items = needsFullFetch
        ? await fetchChangeRequestSearchResults(
            authFetch,
            projectId,
            searchRequest,
          )
        : prefetchedItems.length > 0
          ? prefetchedItems
          : await fetchChangeRequestSearchResults(
              authFetch,
              projectId,
              searchRequest,
            );

      if (items.length === 0) {
        showError(
          "No change requests to export for the current search or filters.",
        );
        return;
      }

      downloadChangeRequestsCsv(items, projectId);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to download results.";
      showError(message);
    } finally {
      isExportingRef.current = false;
      setIsExporting(false);
    }
  }, [
    authFetch,
    prefetchedItems,
    projectId,
    searchRequest,
    showError,
    totalRecords,
  ]);

  return (
    <Button
      type="button"
      variant="outlined"
      size="small"
      onClick={() => void handleDownload()}
      disabled={disabled || isExporting || !projectId}
      startIcon={
        isExporting ? (
          <CircularProgress size={16} color="inherit" />
        ) : (
          <Download size={16} />
        )
      }
    >
      {isExporting ? "Downloading results..." : "Download Results"}
    </Button>
  );
}
