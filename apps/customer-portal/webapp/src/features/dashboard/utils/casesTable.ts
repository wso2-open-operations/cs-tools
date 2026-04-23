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

import {
  CASE_TABLE_CLOSED_STATUS_NORMALIZED,
  CASE_TABLE_OUTSTANDING_STATUS_IDS,
  CASE_TABLE_RESOLVED_LABEL_SUBSTRING,
} from "@/features/dashboard/constants/casesTable";
import {
  CaseTableMetadataKey,
  CaseTableSeverityColorPath,
  CaseTableStatusColorPath,
} from "@features/dashboard/types/casesTable";
import { isS0SeverityLabel } from "@features/dashboard/utils/dashboard";
import { mapSeverityToDisplay } from "@features/support/utils/support";
import {
  CaseSeverityLevel,
  CaseStatus,
} from "@features/support/constants/supportConstants";
import type { NavigateFunction } from "react-router";

/**
 * Returns the Oxygen ui color path for a given severity label.
 * @param label - The severity label (e.g., S0, S1, S2, S3, S4).
 * @returns The Oxygen ui color path (e.g., "error.main").
 */
export const getSeverityColor = (label?: string): string => {
  const normalized = label?.toUpperCase() || "";
  switch (normalized) {
    case CaseSeverityLevel.S0:
      return CaseTableSeverityColorPath.S0;
    case CaseSeverityLevel.S1:
      return CaseTableSeverityColorPath.S1;
    case CaseSeverityLevel.S2:
      return CaseTableSeverityColorPath.S2;
    case CaseSeverityLevel.S3:
      return CaseTableSeverityColorPath.S3;
    case CaseSeverityLevel.S4:
      return CaseTableSeverityColorPath.S4;
    default:
      return CaseTableSeverityColorPath.Default;
  }
};

/**
 * Get status color based on label.
 * @param label - Status label
 * @returns Color string
 */
export const getStatusColor = (label?: string): string => {
  const normalized = label?.toLowerCase() || "";
  switch (true) {
    case normalized.includes(CaseStatus.OPEN.toLowerCase()):
    case normalized.includes(CaseStatus.REOPENED.toLowerCase()):
      return CaseTableStatusColorPath.OpenOrReopened;
    case normalized.includes(CaseStatus.AWAITING_INFO.toLowerCase()):
      return CaseTableStatusColorPath.AwaitingInfo;
    case normalized.includes(CaseStatus.WORK_IN_PROGRESS.toLowerCase()):
      return CaseTableStatusColorPath.WorkInProgress;
    case normalized.includes(CaseStatus.CLOSED.toLowerCase()):
    case normalized.includes(CaseStatus.SOLUTION_PROPOSED.toLowerCase()):
    case normalized.includes(CASE_TABLE_RESOLVED_LABEL_SUBSTRING):
      return CaseTableStatusColorPath.SuccessTerminal;
    default:
      return CaseTableStatusColorPath.Default;
  }
};

function isClosedCaseStatusLabel(label?: string): boolean {
  const normalized = label?.trim().toLowerCase() ?? "";
  switch (normalized) {
    case CASE_TABLE_CLOSED_STATUS_NORMALIZED:
      return true;
    default:
      return false;
  }
}

export function countCasesTableActiveFilters(
  filters: Record<string, string | number | undefined>,
): number {
  return Object.values(filters).filter((v) => v !== "" && v != null).length;
}

export function resolveCasesTableDefaultStatusIds(
  caseStates: Array<{ label: string; id: string }> | undefined,
): number[] {
  return (
    caseStates
      ?.filter((status) => !isClosedCaseStatusLabel(status.label))
      .map((status) => Number(status.id)) ?? []
  );
}

export function resolveCasesTableSearchStatusIds(
  selectedStatusId: string | number | undefined,
  defaultStatusIds: number[],
): number[] {
  switch (true) {
    case Boolean(selectedStatusId):
      return [Number(selectedStatusId)];
    case defaultStatusIds.length > 0:
      return defaultStatusIds;
    default:
      return [...CASE_TABLE_OUTSTANDING_STATUS_IDS];
  }
}

export function filterCasesTableMetadataOptions(
  metadataKey: string,
  metadataOptions: unknown,
  excludeS0: boolean,
  restrictSeverityToLow: boolean = false,
): Array<{ label: string; id: string }> {
  if (!Array.isArray(metadataOptions)) {
    return [];
  }
  switch (metadataKey) {
    case CaseTableMetadataKey.Severities:
      return (
        metadataOptions as Array<{ label: string; id: string }>
      ).filter((item) => {
        if (excludeS0 && isS0SeverityLabel(item.label)) {
          return false;
        }
        if (restrictSeverityToLow) {
          return mapSeverityToDisplay(item.label).startsWith(CaseSeverityLevel.S4);
        }
        return true;
      });
    case CaseTableMetadataKey.CaseStates:
      return (metadataOptions as Array<{ label: string; id: string }>).filter(
        (s) => !isClosedCaseStatusLabel(s.label),
      );
    default:
      return metadataOptions as Array<{ label: string; id: string }>;
  }
}

export function mapCasesTableFilterOptionLabel(
  metadataKey: string,
  itemLabel: string,
): string {
  switch (metadataKey) {
    case CaseTableMetadataKey.Severities:
      return mapSeverityToDisplay(itemLabel);
    default:
      return itemLabel;
  }
}

export function navigateToProjectCaseDetail(
  navigate: NavigateFunction,
  projectId: string,
  caseId: string,
): void {
  navigate(`/projects/${projectId}/support/cases/${caseId}`);
}
