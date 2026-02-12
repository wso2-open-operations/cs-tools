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

import { Box, Paper, alpha, useTheme } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type { CaseDetails } from "@models/responses";
import { getStatusColor } from "@utils/casesTable";
import { resolveColorFromTheme, getStatusIconElement } from "@utils/support";
import CaseDetailsSkeleton from "@components/support/case-details/CaseDetailsSkeleton";
import CaseDetailsBackButton from "@case-details/CaseDetailsBackButton";
import CaseDetailsTabs from "@case-details/CaseDetailsTabs";
import CaseDetailsTabPanels from "@case-details/CaseDetailsTabPanels";
import CaseDetailsHeader from "@case-details/CaseDetailsHeader";
import CaseDetailsActionRow from "@case-details/CaseDetailsActionRow";

export interface CaseDetailsContentProps {
  data: CaseDetails | undefined;
  isLoading: boolean;
  isError: boolean;
  caseId: string;
  onBack: () => void;
}

/**
 * CaseDetailsContent displays case header and details in a layout similar to the template.
 *
 * @param {CaseDetailsContentProps} props - Data, loading/error state, and callbacks.
 * @returns {JSX.Element} The rendered case details content.
 */
export default function CaseDetailsContent({
  data,
  isLoading,
  isError,
  caseId,
  onBack,
}: CaseDetailsContentProps): JSX.Element {
  const theme = useTheme();
  const [activeTab, setActiveTab] = useState(0);
  const [focusMode, setFocusMode] = useState(false);

  const statusLabel = data?.state?.label;
  const severityLabel = data?.severity?.label;
  const statusColorPath = getStatusColor(statusLabel ?? undefined);
  const resolvedStatusColor = resolveColorFromTheme(statusColorPath, theme);
  const statusChipIcon = getStatusIconElement(statusLabel, 12);
  const statusChipSx = useMemo(
    () => ({
      bgcolor: alpha(resolvedStatusColor, 0.1),
      color: resolvedStatusColor,
      height: 20,
      fontSize: "0.75rem",
      px: 0,
      "& .MuiChip-icon": {
        color: "inherit",
        ml: "6px",
        mr: "6px",
      },
      "& .MuiChip-label": {
        pl: 0,
        pr: "6px",
      },
    }),
    [resolvedStatusColor],
  );

  const assignedEngineer = data?.assignedEngineer;
  const engineerInitials =
    assignedEngineer && typeof assignedEngineer === "string"
      ? assignedEngineer
          .split(" ")
          .filter(Boolean)
          .map((n) => n[0])
          .join("")
          .toUpperCase()
          .slice(0, 2)
      : "--";

  if (isLoading) {
    return (
      <Box>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <CaseDetailsBackButton
            onClick={onBack}
            sx={{ mb: 2, ml: -0.5, alignSelf: "flex-start" }}
          />
          <CaseDetailsSkeleton />
        </Paper>
      </Box>
    );
  }

  return (
    <Box>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Paper
          variant="outlined"
          sx={{
            p: 2,
            position: "sticky",
            top: 0,
            zIndex: 10,
            borderRadius: 0,
          }}
        >
          <CaseDetailsBackButton onClick={onBack} />

          {!focusMode && (
            <>
              <CaseDetailsHeader
                caseNumber={data?.number}
                title={data?.title}
                severityLabel={severityLabel}
                statusLabel={statusLabel}
                statusChipIcon={statusChipIcon}
                statusChipSx={statusChipSx}
                isError={isError}
              />

              <CaseDetailsActionRow
                assignedEngineer={assignedEngineer}
                engineerInitials={engineerInitials}
                isError={isError}
              />
            </>
          )}

          <CaseDetailsTabs
            focusMode={focusMode}
            value={activeTab}
            onChange={(_e: unknown, newValue: number) => setActiveTab(newValue)}
            onFocusModeToggle={() => setFocusMode((prev) => !prev)}
          />
        </Paper>

        <CaseDetailsTabPanels activeTab={activeTab} caseId={caseId} />
      </Box>
    </Box>
  );
}
