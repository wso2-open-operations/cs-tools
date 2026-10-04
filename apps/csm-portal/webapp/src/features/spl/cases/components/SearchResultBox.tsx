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

// Ported from apps/support-portal-lite/webapp's own
// features/spl/cases/components/SearchResultBox.tsx. Routes updated to this
// app's "/spl/*" prefix (see csmNavItems.ts).
import { Card, Stack, Typography } from "@wso2/oxygen-ui";
import { alpha, useTheme } from "@mui/material/styles";
import { useNavigate } from "react-router";
import { LinearLoadingPanel, NoResultsPanel } from "./StatePanels";
import type { CaseDetails, CaseDetailsWithCount, AccountSummary, ProjectSummary } from "../api/caseTypes";

type SearchOptions = "account" | "myAccount" | "case" | "project";

export function SearchResultBox({
  searchDataResponse,
  type,
}: {
  searchDataResponse: CaseDetailsWithCount | AccountSummary[] | ProjectSummary[] | undefined;
  type: SearchOptions;
}) {
  const navigate = useNavigate();
  const theme = useTheme();
  // Same warm-orange hover identity in both schemes — see CaseStateCard.
  // Scoped with applyStyles, never a mode check.
  const hoverSx = {
    backgroundColor: alpha("#ff7300", 0.35),
    ...theme.applyStyles("dark", { backgroundColor: alpha("#ff7300", 0.24) }),
  };

  const navigateTo = (id: string) => {
    if (type === "case") navigate(`/spl/cases/${id}`);
    else if (type === "account" || type === "myAccount") navigate(`/spl/accounts/${id}`);
    else if (type === "project") navigate(`/spl/projects/${id}`);
  };

  if (typeof searchDataResponse === "undefined") return <LinearLoadingPanel />;

  const items =
    type === "case" ? (searchDataResponse as CaseDetailsWithCount).cases : (searchDataResponse as AccountSummary[] | ProjectSummary[]);

  if (!items || items.length === 0) return <NoResultsPanel />;

  return (
    <Stack spacing={1.5} sx={{ mt: 2, maxWidth: 640, mx: "auto" }}>
      {type === "case"
        ? (items as CaseDetails[]).map((item, index) => (
            <Card
              key={index}
              variant="outlined"
              sx={{ p: 2, cursor: "pointer", "&:hover": hoverSx }}
              onClick={() => navigateTo(item.id)}
            >
              <Typography variant="subtitle1" fontWeight={700}>
                {item.caseId}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {item.number}
              </Typography>
            </Card>
          ))
        : (items as (AccountSummary | ProjectSummary)[]).map((item, index) => (
            <Card
              key={index}
              variant="outlined"
              sx={{ p: 2, cursor: "pointer", "&:hover": hoverSx }}
              onClick={() => navigateTo(item.id)}
            >
              <Typography variant="subtitle1" fontWeight={700}>
                {item.name}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {item.number}
              </Typography>
            </Card>
          ))}
    </Stack>
  );
}
