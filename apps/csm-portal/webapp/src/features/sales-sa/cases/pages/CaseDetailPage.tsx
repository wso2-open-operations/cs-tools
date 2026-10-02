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
// features/spl/cases/pages/CaseDetailPage.tsx — rewritten against
// useGetCase (React Query) instead of useSplApi's useGetApi. No SplShell
// wrapper (RouteGuard in App.tsx already gates the route tree and mounts
// PermissionProvider). The add-work-note composer the source app had here
// is gone: no SPL-side role ever grants canAddWorkNotes (permanently false,
// see PermissionProvider.tsx) and the backend endpoint it posted to
// requires PermWrite regardless, so the form and its POST were unreachable
// dead code.
import { type ReactNode } from "react";
import { useParams } from "react-router";
import DOMPurify from "dompurify";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Grid,
  Snackbar,
  Stack,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { UserIcon, CalendarDaysIcon, PackageIcon, ListTodoIcon, ServerIcon, ListChecksIcon, FilePlusIcon } from "@wso2/oxygen-ui-icons-react";
import PathView from "../components/PathView";
import { CaseBox } from "../components/CaseBox";
import { AttachmentBox } from "../components/AttachmentBox";
import { useCaseNotice } from "../utils/useCaseNotice";
import { useGetCase } from "../api/useCases";
import { CASE_CLOSED_STATE } from "../api/caseTypes";
import { ErrorPanel, LinearLoadingPanel, NotFoundPanel } from "../components/StatePanels";
import { BackendApiError } from "@api/backend/client";

const PRIORITY_COLOR: Record<string, string> = {
  "Critical (P1)": "#bf2600",
  "High (P2)": "#ff8b00",
  "Medium (P3)": "#ffc400",
};

const STATE_COLOR: Record<string, string> = {
  Open: "#0052cc",
  "Work In Progress": "#008000",
  "Awaiting Info": "#00bfa5",
  "Solution Proposed": "#4caf50",
};

export default function CaseDetailPage() {
  const { caseId: rawCaseId } = useParams<{ caseId: string }>();
  const caseId = rawCaseId ? DOMPurify.sanitize(rawCaseId) : "";

  const { notice, clear } = useCaseNotice();

  const { data, isLoading, error } = useGetCase(caseId);

  const isStateClosed = data?.state === CASE_CLOSED_STATE;

  if (isLoading) return <LinearLoadingPanel />;
  if (error) return error instanceof BackendApiError && error.status === 404 ? <NotFoundPanel /> : <ErrorPanel />;
  if (!data) return null;

  return (
    <Box sx={{ p: 3 }}>
      <PathView
        accountName={data.accountName || data.accountNumber}
        projectKey={data.projectKey || data.projectNumber}
        accountNumber={data.accountId}
        projectNumber={data.projectId}
        caseKey={data.caseId.split("-")[1]}
      />

      <Card variant="outlined" sx={{ mb: 3 }}>
        <CardContent sx={{ textAlign: "center" }}>
          <Typography variant="h5">{data.shortDescription}</Typography>
          <Typography sx={{ color: "#ff7300", fontWeight: 700, mt: 0.5 }}>
            {data.number}&nbsp;&nbsp;{data.caseId}
          </Typography>
          <Stack direction="row" spacing={1} justifyContent="center" sx={{ py: 1.5, my: 1.5, borderTop: 1, borderBottom: 1, borderColor: "divider" }}>
            {data.caseType && (
              <Tooltip title="Case Type">
                <Chip label={data.caseType} color="secondary" />
              </Tooltip>
            )}
            {data.priority && (
              <Tooltip title="Priority">
                <Chip label={data.priority} sx={{ backgroundColor: PRIORITY_COLOR[data.priority] ?? "#ff7300", color: "#fff" }} />
              </Tooltip>
            )}
            <Tooltip title="Status">
              <Chip label={data.state} sx={{ backgroundColor: STATE_COLOR[data.state] ?? "#8993a4", color: "#fff" }} />
            </Tooltip>
          </Stack>
          <Grid container spacing={2} sx={{ textAlign: "left", mt: 1 }}>
            <Grid size={{ xs: 12, md: 6 }}>
              <Stack spacing={1}>
                <FieldRow icon={<UserIcon size={16} />} label="Assigned to" value={data.assignedTo} />
                <FieldRow icon={<CalendarDaysIcon size={16} />} label="Opened on" value={data.openedAt.split(" ")[0]} />
                <FieldRow icon={<UserIcon size={16} />} label="Opened By" value={data.openedBy} />
                <FieldRow icon={<PackageIcon size={16} />} label="Product" value={data.productName || "N/A"} />
              </Stack>
            </Grid>
            <Grid size={{ xs: 12, md: 6 }}>
              <Stack spacing={1}>
                <FieldRow icon={<ListTodoIcon size={16} />} label="Last WSO2 Comment" value={data.lastWSO2CommentTime} />
                <FieldRow icon={<ListTodoIcon size={16} />} label="Last Customer Comment" value={data.lastCustomerCommentTime} />
                <FieldRow icon={<ServerIcon size={16} />} label="Deployment Name" value={data.projectDeploymentName} />
                <FieldRow icon={<ListChecksIcon size={16} />} label="Deployment Type" value={data.projectDeploymentType} />
              </Stack>
            </Grid>
          </Grid>
        </CardContent>
      </Card>

      <Grid container spacing={3}>
        <Grid size={{ xs: 12, md: 9 }}>
          {isStateClosed ? (
            <Tooltip title="Case is closed">
              <span>
                <Button variant="contained" disabled startIcon={<FilePlusIcon size={16} />}>
                  New Work Note
                </Button>
              </span>
            </Tooltip>
          ) : (
            // Backend enforcement (PermWrite, cs_engineer/admin only) has no
            // SPL-side role that grants it -- see PermissionProvider.tsx's
            // canAddWorkNotes, permanently false. This button stays
            // permanently disabled until a role exists that can actually
            // reach POST /cases/{id}/comments.
            <Tooltip title="You don't have the permission">
              <span>
                <Button variant="contained" disabled startIcon={<FilePlusIcon size={16} />}>
                  New Work Note
                </Button>
              </span>
            </Tooltip>
          )}

          <Box sx={{ mt: 2 }}>
            <CaseBox caseId={caseId} />
          </Box>
        </Grid>

        <Grid size={{ xs: 12, md: 3 }}>
          <Card variant="outlined">
            <CardContent>
              <Typography variant="subtitle1" sx={{ mb: 1.5 }}>
                Attachments
              </Typography>
              <AttachmentBox caseId={caseId} />
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      <Snackbar open={!!notice} autoHideDuration={4000} onClose={clear}>
        {notice ? (
          <Alert severity={notice.severity} onClose={clear} sx={{ width: "100%" }}>
            {notice.message}
          </Alert>
        ) : undefined}
      </Snackbar>
    </Box>
  );
}

function FieldRow({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <Stack direction="row" spacing={1} alignItems="center">
      {icon}
      <Typography variant="body2" color="text.secondary">
        {label}:
      </Typography>
      <Typography variant="body2">{value}</Typography>
    </Stack>
  );
}
