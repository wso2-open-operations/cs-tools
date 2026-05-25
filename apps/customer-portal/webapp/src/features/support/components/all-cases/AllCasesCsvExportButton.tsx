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

import type { JSX } from "react";
import CaseListCsvExportButton, {
  type CaseListCsvExportButtonProps,
} from "@features/support/components/list-export/CaseListCsvExportButton";

export type AllCasesCsvExportButtonProps = Omit<
  CaseListCsvExportButtonProps,
  "filenamePrefix" | "exportVariant" | "emptyMessage"
>;

/**
 * Download Results button for the All Cases list (includes Severity column).
 *
 * @param {AllCasesCsvExportButtonProps} props - Project id, search request, and disabled state.
 * @returns {JSX.Element} Download Results button.
 */
export default function AllCasesCsvExportButton(
  props: AllCasesCsvExportButtonProps,
): JSX.Element {
  return (
    <CaseListCsvExportButton
      {...props}
      filenamePrefix="cases"
      exportVariant="allCases"
      emptyMessage="No cases to export for the current search or filters."
    />
  );
}
