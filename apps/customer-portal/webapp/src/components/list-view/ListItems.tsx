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

import { Box, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { CaseListItem } from "@features/support/types/cases";
import ListCard from "@components/list-view/ListCard";
import ListSkeleton from "@components/list-view/ListSkeleton";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";

export interface ListItemsProps {
  cases: CaseListItem[];
  isLoading: boolean;
  isError?: boolean;
  hasListRefinement?: boolean;
  onCaseClick?: (caseItem: CaseListItem) => void;
  entityName?: string;
  hideSeverity?: boolean;
  showInternalId?: boolean;
}

/**
 * Component to display cases as cards.
 *
 * @param {ListItemsProps} props - Cases array and loading state.
 * @returns {JSX.Element} The rendered case cards list.
 */
export default function ListItems({
  cases,
  isLoading,
  isError = false,
  hasListRefinement = false,
  onCaseClick,
  entityName = "items",
  hideSeverity = false,
  showInternalId = false,
}: ListItemsProps): JSX.Element {
  if (isLoading) {
    return <ListSkeleton />;
  }

  if (isError) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <ErrorIndicator entityName={entityName} size="medium" />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          Failed to load {entityName}. Please try again.
        </Typography>
      </Box>
    );
  }

  if (cases.length === 0) {
    if (hasListRefinement) {
      return (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: 6,
          }}
        >
          <SearchNoResultsIcon
            style={{
              width: 200,
              maxWidth: "100%",
              height: "auto",
              marginBottom: 16,
            }}
          />
          <Typography variant="body1" color="text.secondary">
            No {entityName} found. Try adjusting your filters or search query.
          </Typography>
        </Box>
      );
    }
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          py: 6,
        }}
      >
        <EmptyIcon
          style={{
            width: 200,
            maxWidth: "100%",
            height: "auto",
            marginBottom: 16,
          }}
        />
        <Typography variant="body1" color="text.secondary">
          No {entityName} yet.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {cases.map((caseItem) => (
        <ListCard
          key={caseItem.id}
          caseItem={caseItem}
          onClick={onCaseClick}
          hideSeverity={hideSeverity}
          showInternalId={showInternalId}
        />
      ))}
    </Box>
  );
}
