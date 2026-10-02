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
// features/spl/cases/components/Search.tsx — rewritten against
// useBackendApi() (this app's existing authenticated fetch, no mock
// fallback needed since CSM_PORTAL_BACKEND_BASE_URL is always configured
// here, unlike the source app's optional SPL_APP_BACKEND_BASE_URL). Kept
// local to features/spl/cases/ (not shared across domains), same
// file-collision-avoidance precedent DefaultTable.tsx documents — the
// Accounts/Projects port needs its own copy of this, not a shared import.
import { useEffect, useState, type ChangeEvent } from "react";
import { InputAdornment, TextField } from "@wso2/oxygen-ui";
import { SearchIcon } from "@wso2/oxygen-ui-icons-react";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useBackendApi } from "@api/backend/client";
import { SearchResultBox } from "./SearchResultBox";
import type { CaseDetailsWithCount, AccountSummary, ProjectSummary } from "../api/caseTypes";

type SearchOptions = "account" | "myAccount" | "case" | "project";
type SearchResult = CaseDetailsWithCount | AccountSummary[] | ProjectSummary[];

// Account/project/case search used to call this backend's own /spl/accounts,
// /spl/projects, /spl/cases GET routes directly; all three now call CS
// Portal's own POST /accounts/search, /projects/search, /cases/search --
// see useSplAccountsApi.ts/useSplProjectsApi.ts/useCases.ts for the same
// merge, and cs-tools' csm-portal-backend main.go SPL route registration
// comment for why. Kept as a raw fetch here (not those hooks' React Query
// versions) since this is a debounced free-text search, not a cacheable
// query keyed on stable params.
interface EntityRef {
  id: string;
  name: string;
}
interface EntityAccountSearchItem {
  id: string;
  number: string;
  name: string;
}
interface EntitySearchAccountsResponse {
  accounts: EntityAccountSearchItem[];
}
interface EntityProjectSearchItem {
  id: string;
  key: string;
  name: string;
}
interface EntitySearchProjectsResponse {
  projects: EntityProjectSearchItem[];
}
interface EntityCaseSearchItem {
  id: string;
  internalId: string;
  number: string;
  subject: string | null;
  state: string | null;
  caseType?: string;
  priority?: string;
  product: EntityRef | null;
}
interface EntitySearchCasesResponse {
  cases: EntityCaseSearchItem[];
  total: number;
}
const caseStateToDisplay: Record<string, string> = {
  open: "Open",
  work_in_progress: "Work In Progress",
  awaiting_info: "Awaiting Info",
  solution_proposed: "Solution Proposed",
  waiting_on_wso2: "Waiting on WSO2",
  reopened: "Reopened",
  closed: "Closed",
};

export default function Search({
  searchOption,
  setShowTable,
}: {
  searchOption: SearchOptions;
  setShowTable?: (value: boolean) => void;
}) {
  const email = useIdTokenClaims()?.email;
  const api = useBackendApi();
  const [inputValue, setInputValue] = useState("");
  const [data, setData] = useState<SearchResult>();

  const onInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value;
    setInputValue(value);
    if (setShowTable) setShowTable(value.length < 4);
  };

  useEffect(() => {
    if (inputValue.length < 4) return;
    let cancelled = false;
    const pagination = { offset: 0, limit: 10 };

    let request: Promise<SearchResult>;
    if (searchOption === "account" || searchOption === "myAccount") {
      const body = {
        filters: {
          searchQuery: inputValue,
          ownerEmail: searchOption === "myAccount" ? email ?? "" : "",
        },
        pagination,
      };
      request = api
        .post<typeof body, EntitySearchAccountsResponse>("/accounts/search", body)
        .then((r): AccountSummary[] => (r.accounts ?? []).map((a) => ({ id: a.id, number: a.number, name: a.name })));
    } else if (searchOption === "project") {
      const body = { searchQuery: inputValue, pagination };
      request = api
        .post<typeof body, EntitySearchProjectsResponse>("/projects/search", body)
        .then((r): ProjectSummary[] => (r.projects ?? []).map((p) => ({ id: p.id, number: p.key, name: p.name })));
    } else {
      const body = {
        filters: { searchQuery: inputValue, filters: [] },
        sortBy: { field: "createdOn", order: "desc" },
        pagination,
      };
      request = api
        .post<typeof body, EntitySearchCasesResponse>("/cases/search", body)
        .then(
          (r): CaseDetailsWithCount => ({
            count: r.total,
            cases: (r.cases ?? []).map((c) => ({
              id: c.id,
              caseId: c.internalId,
              number: c.number,
              caseType: "case",
              priority: "",
              shortDescription: c.subject ?? "",
              state: caseStateToDisplay[c.state ?? ""] ?? c.state ?? "",
              openedAt: "",
              openedBy: "",
              description: "",
              assignedTo: "",
              accountNumber: "",
              accountName: "",
              projectNumber: "",
              projectKey: "",
              productName: c.product?.name ?? "",
              lastWSO2CommentTime: "",
              lastCustomerCommentTime: "",
              projectDeploymentName: "",
              projectDeploymentType: "",
            })),
          }),
        );
    }

    request
      .then((result) => {
        if (!cancelled) setData(result);
      })
      .catch(() => {
        // Search is best-effort — a failed lookup just leaves the result list empty.
      });

    return () => {
      cancelled = true;
    };
  }, [inputValue, searchOption, email, api]);

  return (
    <div>
      <TextField
        fullWidth
        variant="outlined"
        placeholder={
          searchOption === "case"
            ? "Search by Case Number"
            : searchOption === "account" || searchOption === "myAccount"
              ? "Search by Account Name"
              : "Search by Project Name"
        }
        value={inputValue}
        onChange={onInputChange}
        sx={{ maxWidth: 640, mx: "auto", display: "block", "& .MuiOutlinedInput-root": { borderRadius: 8 } }}
        slotProps={{ input: { endAdornment: <InputAdornment position="end"><SearchIcon size={18} /></InputAdornment> } }}
      />
      {inputValue.length >= 4 && <SearchResultBox searchDataResponse={data} type={searchOption} />}
    </div>
  );
}
