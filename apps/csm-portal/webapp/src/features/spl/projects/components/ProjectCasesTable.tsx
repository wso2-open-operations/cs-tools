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

// Case links point at /spl/cases/:caseId, not this app's own /cases route
// (that's CSM Portal's own, unrelated case-management feature).
import { useEffect, useState } from "react";
import { Box, type SelectChangeEvent } from "@wso2/oxygen-ui";
import { useGetProjectCases } from "../api/useProjectsApi";
import DefaultTable from "./DefaultTable";
import { SelectDropdown } from "./SelectDropdown";
import type { CaseDetails } from "../projectTypes";

const STATES = [
  "Open", "Work In Progress", "Awaiting Info", "Solution Proposed", "In Progress",
  "Waiting on Client", "Waiting on WSO2", "Closed", "Cancelled", "Reopened", "Differed",
];
const DEFAULT_STATES = [
  "Open", "Work In Progress", "Awaiting Info", "Solution Proposed", "In Progress",
  "Waiting on Client", "Waiting on WSO2", "Closed",
];
const CASE_TYPES = [
  "Query", "Admin Task", "Incident", "Hosting Query", "Cloud Incident", "Cloud Query", "Task",
  "Hosting", "Announcement", "Engagement", "NFR", "Story", "New Feature", "Change Requests",
  "Bug", "Sub-Task", "Improvement", "Hosting Task",
];
const DEFAULT_CASE_TYPES = [
  "Query", "Admin Task", "Incident", "Hosting Query", "Cloud Incident", "Cloud Query", "Task", "Hosting",
];

function readStored(key: string, fallback: string[]): string[] {
  const raw = localStorage.getItem(key);
  if (!raw) return fallback;
  try {
    return JSON.parse(raw) as string[];
  } catch {
    return fallback;
  }
}

export default function ProjectCasesTable({ id, isTypeCloud }: { id: string; isTypeCloud: boolean }) {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const [stateValues, setStateValues] = useState<string[]>(() => readStored("spl.stateValues", DEFAULT_STATES));
  const [caseTypeValues, setCaseTypeValues] = useState<string[]>(() =>
    isTypeCloud ? [] : readStored("spl.caseTypeValues", DEFAULT_CASE_TYPES),
  );

  const { data, isLoading, error } = useGetProjectCases({
    projectId: id,
    offset: page * rowsPerPage,
    limit: rowsPerPage,
    stateFilters: stateValues,
    caseTypeFilters: caseTypeValues,
  });

  useEffect(() => {
    if (!isTypeCloud) localStorage.setItem("spl.caseTypeValues", JSON.stringify(caseTypeValues));
    localStorage.setItem("spl.stateValues", JSON.stringify(stateValues));
  }, [isTypeCloud, caseTypeValues, stateValues]);

  const handleCaseTypeChange = (event: SelectChangeEvent<string[]>) => {
    const { value } = event.target;
    setCaseTypeValues(typeof value === "string" ? value.split(",") : value);
  };
  const handleStateChange = (event: SelectChangeEvent<string[]>) => {
    const { value } = event.target;
    setStateValues(typeof value === "string" ? value.split(",") : value);
  };
  const handleCaseTableRowClick = (rowData: CaseDetails) => {
    window.open(`/spl/cases/${rowData.id}`, "_blank", "noopener,noreferrer");
  };

  const colNameArray = ["Number", "Case ID", "Short Description", "Case Type", "Priority", "State"];
  const colAttributeArray = ["number", "caseId", "shortDescription", "caseType", "priority", "state"];

  return (
    <Box>
      <Box paddingBottom={2}>
        {!isTypeCloud && (
          <SelectDropdown
            label="Case Type"
            allValues={CASE_TYPES}
            values={caseTypeValues}
            handleSelectChange={handleCaseTypeChange}
          />
        )}
        <SelectDropdown label="State" allValues={STATES} values={stateValues} handleSelectChange={handleStateChange} />
      </Box>
      <Box>
        <DefaultTable
          data={data}
          loading={isLoading}
          error={error}
          page={page}
          setPage={setPage}
          rowsPerPage={rowsPerPage}
          setRowsPerPage={setRowsPerPage}
          colNameArray={isTypeCloud ? colNameArray.filter((c) => c !== "Case Type") : colNameArray}
          colAttributeArray={isTypeCloud ? colAttributeArray.filter((c) => c !== "caseType") : colAttributeArray}
          handleRowClick={handleCaseTableRowClick}
        />
      </Box>
    </Box>
  );
}
