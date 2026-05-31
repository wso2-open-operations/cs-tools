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

import { Box, Button, Card, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, ArrowRight } from "@wso2/oxygen-ui-icons-react";
import { useEffect, type JSX, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router";
import {
  useGetCsmAccountDetail,
  useGetProjectsForAccount,
} from "@features/csm-accounts/api/useGetCsmAccounts";
import ProjectsList from "@features/csm-projects/components/ProjectsList";
import { casesHref } from "@features/csm-cases/utils/casesFiltersUrl";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";

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

  const recordView = useRecordRecentView();
  useEffect(() => {
    if (!account) return;
    recordView({
      kind: "account",
      id: account.id,
      title: account.name,
      subtitle: `${account.tier} · ${account.projectCount} project${account.projectCount === 1 ? "" : "s"}`,
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

  if (isAccountError) {
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
        <Typography variant="body1" color="error">
          Could not load account {accountId}.
        </Typography>
      </Box>
    );
  }

  if (!account) {
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
        <Typography variant="h5">Account not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No account with id <code>{accountId}</code> in the current mock dataset.
        </Typography>
      </Box>
    );
  }

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

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Summary</Typography>
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
              {formatRelativeTime(account.lastActivityAt)}
            </Typography>
          </MetaCell>
        </Box>
      </Card>

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
          <Box>
            <Typography variant="subtitle1">Projects</Typography>
            <Typography variant="caption" color="text.secondary">
              An account has many projects; each project belongs to exactly
              one account. {projects?.length ?? 0} project
              {(projects?.length ?? 0) === 1 ? "" : "s"} under {account.name}.
            </Typography>
          </Box>
          <Button
            size="small"
            variant="outlined"
            endIcon={<ArrowRight size={16} />}
            onClick={() => navigate(casesHref({ search: account.name }))}
          >
            View all cases
          </Button>
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
    </Box>
  );
}
