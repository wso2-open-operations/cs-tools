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

// Ported from apps/support-portal-lite/webapp's own features/spl/cases/components/PathView.tsx
// (breadcrumb: Home > Account > Project > Case). Routes updated from that
// app's bare "/cases"/"/accounts"/"/projects" to this app's "/spl/*" prefix
// (see csmNavItems.ts's "spl" section).
import type { ReactNode } from "react";
import { Stack, Tooltip, Button } from "@wso2/oxygen-ui";
import { HomeIcon, ChevronRightIcon } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router";

function PathButton({
  text,
  icon,
  onClick,
  tooltip,
  active,
}: {
  text?: string;
  icon?: ReactNode;
  onClick?: () => void;
  tooltip: string;
  active?: boolean;
}) {
  return (
    <Tooltip title={tooltip}>
      <Button
        size="small"
        onClick={onClick}
        startIcon={icon}
        variant={active ? "contained" : "text"}
        sx={{ textTransform: "none" }}
      >
        {text}
      </Button>
    </Tooltip>
  );
}

export default function PathView({
  accountName,
  projectKey,
  accountNumber,
  projectNumber,
  caseKey,
}: {
  accountName?: string;
  projectKey?: string;
  accountNumber?: string;
  projectNumber?: string;
  caseKey?: string;
}) {
  const navigate = useNavigate();
  const activeButton = caseKey ? "Case" : projectKey ? "Project" : "Account";

  const handleAccountClick = () => {
    if (accountNumber) navigate(`/spl/accounts/${accountNumber}`);
  };

  const handleProjectClick = () => {
    if (projectNumber) navigate(`/spl/projects/${projectNumber}`);
  };

  return (
    <Stack direction="row" spacing={0.5} alignItems="center" sx={{ mb: 2 }}>
      <PathButton icon={<HomeIcon size={16} />} onClick={() => navigate("/spl/cases")} tooltip="Home" />
      <ChevronRightIcon size={14} style={{ opacity: 0.5 }} />
      <PathButton text={accountName} onClick={handleAccountClick} tooltip="Account" active={activeButton === "Account"} />
      {projectKey && (
        <>
          <ChevronRightIcon size={14} style={{ opacity: 0.5 }} />
          <PathButton text={projectKey} onClick={handleProjectClick} tooltip="Project" active={activeButton === "Project"} />
        </>
      )}
      {caseKey && (
        <>
          <ChevronRightIcon size={14} style={{ opacity: 0.5 }} />
          <PathButton text={caseKey} tooltip="Case" active={activeButton === "Case"} />
        </>
      )}
    </Stack>
  );
}
