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

import { useMemo, type JSX } from "react";
import type { CaseListItem } from "@features/support/types/cases";
import type { ChangeRequestItem } from "@features/operations/types/changeRequests";
import ListCard from "@components/list-view/ListCard";

export interface SearchChangeRequestCardProps {
  changeRequest: ChangeRequestItem;
  onClick: (changeRequest: ChangeRequestItem) => void;
}

/**
 * Compact change-request card used in the header search dropdown.
 * Adapts a ChangeRequestItem to the shared ListCard layout so search
 * results look uniform across cases and change requests.
 *
 * @param {SearchChangeRequestCardProps} props - Change request data and click handler.
 * @returns {JSX.Element} The rendered card.
 */
export default function SearchChangeRequestCard({
  changeRequest,
  onClick,
}: SearchChangeRequestCardProps): JSX.Element {
  const caseShaped = useMemo<CaseListItem>(
    () => ({
      id: changeRequest.id,
      internalId: changeRequest.internalId ?? undefined,
      number: changeRequest.number,
      title: changeRequest.title,
      description: changeRequest.description ?? "",
      assignedEngineer: changeRequest.assignedEngineer,
      project: changeRequest.project ?? { id: "", label: "" },
      issueType: null,
      deployedProduct: changeRequest.deployedProduct,
      deployment: changeRequest.deployment,
      severity: null,
      status: changeRequest.state,
      type: changeRequest.type,
      createdOn: changeRequest.createdOn,
      createdBy: changeRequest.createdBy,
      updatedOn: changeRequest.updatedOn,
      updatedBy: changeRequest.updatedBy,
    }),
    [changeRequest],
  );

  return (
    <ListCard
      caseItem={caseShaped}
      onClick={() => onClick(changeRequest)}
      hideSeverity
    />
  );
}
