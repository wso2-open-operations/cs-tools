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
  Box,
  Button,
  Card,
  Chip,
  Skeleton,
  Tab,
  Tabs,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ArrowRight } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router";
import {
  useGetCsmAccountDetail,
  useGetProjectsForAccount,
} from "@features/csm-accounts/api/useGetCsmAccounts";
import ProjectsList from "@features/csm-projects/components/ProjectsList";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import CasesList from "@features/csm-cases/components/CasesList";
import { casesHref } from "@features/csm-cases/utils/casesFiltersUrl";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { isMockMode } from "@api/backend/client";
import { getMockAccountContacts } from "@features/csm-projects/api/mocks/contactsMocks";
import ContactsCard from "@features/csm-projects/components/ContactsCard";
import RelativeTime from "@components/RelativeTime";

type AccountTab = "summary" | "projects" | "cases" | "contacts";

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

export default function CsmAccountDetailPage(): JSX.Element {
  const { accountId } = useParams<{ accountId: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<AccountTab>("summary");

  const {
    data: account,
    isLoading: isAccountLoading,
    isError: isAccountError,
  } = useGetCsmAccountDetail(accountId);
  const {
    data: projects,
    isLoading: isProjectsLoading,
    isError: isProjectsError,
  } = useGetProjectsForAccount(accountId);
  const { data: casesData, isLoading: isCasesLoading } =
    useGetCsmCases("all_customers");

  // The full account roster includes both WSO2-side assignments
  // (Account Manager, Technical Owner) and customer-side contacts. We surface
  // AM/TO names in the Summary header and keep the Contacts tab customer-only.
  // Contacts are mock-only for now — no BE endpoint yet, so render nothing
  // when the mock toggle is off rather than leaking seeded data.
  const allAccountContacts = useMemo(
    () =>
      accountId && isMockMode() ? getMockAccountContacts(accountId) : [],
    [accountId],
  );
  const contacts = useMemo(
    () =>
      allAccountContacts.filter(
        (c) =>
          !c.roles.includes("Account Manager") &&
          !c.roles.includes("Technical Owner"),
      ),
    [allAccountContacts],
  );

  const accountCases = useMemo(() => {
    const rows = casesData?.cases ?? [];
    return rows
      .filter((c) => c.accountId === accountId)
      .slice()
      .sort((a, b) => {
        const aClosed = a.state === "closed" ? 1 : 0;
        const bClosed = b.state === "closed" ? 1 : 0;
        if (aClosed !== bClosed) return aClosed - bClosed;
        return a.minutesToBreach - b.minutesToBreach;
      });
  }, [casesData?.cases, accountId]);

  // Pull derived account-manager / technical-owner from the FULL contact
  // list (not the filtered customer-only list passed to the Contacts tab).
  const accountManager = useMemo(
    () =>
      allAccountContacts.find((c) => c.roles.includes("Account Manager"))
        ?.name,
    [allAccountContacts],
  );
  const technicalOwner = useMemo(
    () =>
      allAccountContacts.find((c) => c.roles.includes("Technical Owner"))
        ?.name,
    [allAccountContacts],
  );

  const recordView = useRecordRecentView();
  useEffect(() => {
    if (!account) return;
    recordView({
      kind: "account",
      id: account.id,
      title: account.name,
      subtitle: `${account.tier} · ${account.projectCount} project${
        account.projectCount === 1 ? "" : "s"
      }`,
      href: `/accounts/${account.id}`,
    });
  }, [account, recordView]);

  if (isAccountLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" width={240} height={32} />
        <Skeleton variant="rectangular" height={140} />
        <Skeleton variant="rectangular" height={200} />
      </Box>
    );
  }

  if (isAccountError || !account) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/accounts")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to accounts
        </Button>
        <Typography variant="h5">
          {isAccountError ? "Could not load account" : "Account not found"}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          No account with id <code>{accountId}</code> in the current dataset.
        </Typography>
      </Box>
    );
  }

  const projectCount = projects?.length ?? 0;
  const caseCount = accountCases.length;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/accounts")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to accounts
      </Button>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            flexWrap: "wrap",
          }}
        >
          <Typography variant="h5">{account.name}</Typography>
          <Chip
            size="small"
            variant="outlined"
            label={account.tier}
            color={
              account.tier === "Platinum"
                ? "primary"
                : account.tier === "Gold"
                  ? "warning"
                  : "default"
            }
          />
          <Chip
            size="small"
            variant="outlined"
            label={account.status}
            color={
              account.status === "Active"
                ? "success"
                : account.status === "Onboarding"
                  ? "warning"
                  : "default"
            }
          />
          {account.breachedCount > 0 && (
            <Chip
              size="small"
              color="error"
              label={`${account.breachedCount} breached`}
            />
          )}
        </Box>
        <Typography variant="body2" color="text.secondary">
          Account id <code>{account.id}</code>
        </Typography>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, value) => setActiveTab(value as AccountTab)}
        >
          <Tab value="summary" label="Summary" />
          <Tab
            value="projects"
            label={`Projects${projectCount > 0 ? ` (${projectCount})` : ""}`}
          />
          <Tab
            value="cases"
            label={`Cases${caseCount > 0 ? ` (${caseCount})` : ""}`}
          />
          <Tab
            value="contacts"
            label={`Contacts${contacts.length > 0 ? ` (${contacts.length})` : ""}`}
          />
        </Tabs>
      </Box>

      {activeTab === "summary" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
          <Card
            sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}
          >
            <Typography variant="subtitle2">Account</Typography>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: {
                  xs: "1fr 1fr",
                  md: "repeat(5, minmax(0, 1fr))",
                },
                gap: 2,
              }}
            >
              <MetaCell label="Projects">
                <Typography variant="h6">{account.projectCount}</Typography>
              </MetaCell>
              <MetaCell label="Open cases">
                <Typography variant="h6">{account.openCaseCount}</Typography>
              </MetaCell>
              <MetaCell label="S0 / S1">
                <Typography
                  variant="h6"
                  color={account.s0s1Count > 0 ? "error" : "text.primary"}
                >
                  {account.s0s1Count}
                </Typography>
              </MetaCell>
              <MetaCell label="Breached">
                <Typography
                  variant="h6"
                  color={account.breachedCount > 0 ? "error" : "text.primary"}
                >
                  {account.breachedCount}
                </Typography>
              </MetaCell>
              <MetaCell label="Last activity">
                <Typography variant="body2">
                  <RelativeTime iso={account.lastActivityAt} />
                </Typography>
              </MetaCell>
              <MetaCell label="Account Manager">
                <Typography variant="body2">{accountManager ?? "—"}</Typography>
              </MetaCell>
              <MetaCell label="Technical Owner">
                <Typography variant="body2">{technicalOwner ?? "—"}</Typography>
              </MetaCell>
            </Box>
          </Card>
        </Box>
      )}

      {activeTab === "contacts" && (
        <ContactsCard
          contacts={contacts}
          subtitle="Account-level contacts. Project-specific contacts are listed on each project's detail page."
        />
      )}

      {activeTab === "projects" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 2,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="caption" color="text.secondary">
              {projectCount} project{projectCount === 1 ? "" : "s"} under{" "}
              {account.name}.
            </Typography>
          </Box>
          {isProjectsError && !isProjectsLoading ? (
            <Typography variant="body2" color="error">
              Could not load projects.
            </Typography>
          ) : (
            <ProjectsList
              projects={projects ?? []}
              isLoading={isProjectsLoading}
            />
          )}
        </Box>
      )}

      {activeTab === "cases" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 2,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="caption" color="text.secondary">
              {caseCount} case{caseCount === 1 ? "" : "s"} across all projects
              under {account.name}.
            </Typography>
            <Button
              size="small"
              variant="outlined"
              endIcon={<ArrowRight size={16} />}
              onClick={() => navigate(casesHref({ search: account.name }))}
            >
              View in cases list
            </Button>
          </Box>
          <CasesList cases={accountCases} isLoading={isCasesLoading} />
        </Box>
      )}
    </Box>
  );
}
