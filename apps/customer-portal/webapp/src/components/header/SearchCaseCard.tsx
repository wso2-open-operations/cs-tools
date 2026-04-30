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
import type { CaseListItem } from "@features/support/types/cases";
import ListCard from "@components/list-view/ListCard";

export interface SearchCaseCardProps {
  caseItem: CaseListItem;
  onClick: (caseItem: CaseListItem) => void;
}

/**
 * Compact case card used in the header search dropdown.
 * Always shows the WSO2 internal ID alongside the public number.
 *
 * @param {SearchCaseCardProps} props - Case data and click handler.
 * @returns {JSX.Element} The rendered card.
 */
export default function SearchCaseCard({
  caseItem,
  onClick,
}: SearchCaseCardProps): JSX.Element {
  return <ListCard caseItem={caseItem} onClick={onClick} />;
}
