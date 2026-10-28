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
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Link,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, FilePlus, Pause, Play, Archive, PencilLine } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { useLocation } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import {
  useClearStandardChangeTemplateReview,
  useCreateStandardChangeProposal,
  useGetStandardChangeTemplate,
  useGetStandardChangeTemplateVersion,
  useResumeStandardChangeTemplate,
  useSuspendStandardChangeTemplate,
} from "@features/csm-operations/api/useStandardChanges";
import StandardChangeContentView, {
  StandardChangeParametersTable,
} from "@features/csm-operations/components/StandardChangeContentView";
import {
  raiseFromTemplatePath,
  STANDARD_CHANGES_TAB_PATH,
  standardChangeProposalPath,
  standardChangeTemplateStateColor,
  standardChangeTemplateStateLabel,
} from "@features/csm-operations/utils/standardChanges";
import type { BeStandardChangeProposalKind } from "@api/backend/types";

function formatDateTime(value?: string | null): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—";
}

function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message.trim() ? err.message : fallback;
}

function Meta({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        {label}
      </Typography>
      <Typography variant="body2" component="div">
        {children}
      </Typography>
    </Box>
  );
}

/** A confirmation before a template is suspended or resumed. */
type PendingConfirm = "suspend" | "resume" | null;

/**
 * One standard change template (`/operations/standard-changes/:id`): its current
 * version (the text a change request raised from it takes, placeholders
 * highlighted, and its parameters), every earlier version, its usage and the
 * review flag. From here a change request is raised from it, a revision or the
 * retirement is proposed to the CAB, the review flag is cleared and the template
 * suspended or resumed. Who may do the last three (the owner or a CAB member) is
 * the server's decision: the buttons are offered and a refusal's reason shown.
 */
export default function StandardChangeTemplateDetailPage(): JSX.Element {
  const id = useNormalizedIdParam("id");
  const navigate = useNavTransition();
  const location = useLocation();
  const { canWrite } = usePortalAccess();
  const backTarget = (location.state as { from?: string } | undefined)?.from ?? STANDARD_CHANGES_TAB_PATH;
  const here = `${location.pathname}${location.search}`;

  const { data: template, isLoading, isError, error, refetch } = useGetStandardChangeTemplate(id);
  // An older version picked from the history; null = the current one.
  const [versionId, setVersionId] = useState<string | null>(null);
  const viewingOlder = !!versionId && versionId !== template?.currentVersion?.id;
  const olderVersion = useGetStandardChangeTemplateVersion(id, viewingOlder ? versionId! : undefined);

  const suspend = useSuspendStandardChangeTemplate();
  const resume = useResumeStandardChangeTemplate();
  const clearReview = useClearStandardChangeTemplateReview();
  const createProposal = useCreateStandardChangeProposal();
  const [confirm, setConfirm] = useState<PendingConfirm>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={() => navigate(backTarget)}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Skeleton variant="rounded" height={120} />
        <Skeleton variant="rounded" height={320} />
      </Box>
    );
  }
  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <QueryErrorState
          message={errorText(error, "Failed to load the standard change template.")}
          error={error}
          onRetry={() => void refetch()}
        />
      </Box>
    );
  }
  if (!template) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Standard change template not found</Typography>
      </Box>
    );
  }

  const current = template.currentVersion;
  const shown = viewingOlder ? olderVersion.data : current;
  const busy = suspend.isPending || resume.isPending || clearReview.isPending || createProposal.isPending;
  const onError = (fallback: string) => (err: unknown) => setActionError(errorText(err, fallback));

  const propose = (kind: BeStandardChangeProposalKind): void => {
    setActionError(null);
    createProposal.mutate(
      { kind, templateId: template.id },
      {
        onSuccess: (p) => navigate(standardChangeProposalPath(p.id), { state: { from: here } }),
        onError: onError("Could not start the proposal."),
      },
    );
  };

  const runConfirm = (): void => {
    const which = confirm;
    setConfirm(null);
    setActionError(null);
    if (which === "suspend") suspend.mutate(template.id, { onError: onError("Could not suspend the template.") });
    if (which === "resume") resume.mutate(template.id, { onError: onError("Could not resume the template.") });
  };

  const openProposalNote = template.openProposal && (
    <Alert severity="info">
      A revision or retirement of this template is open:{" "}
      <Link
        component="button"
        onClick={() => navigate(standardChangeProposalPath(template.openProposal!.id), { state: { from: here } })}
      >
        {template.openProposal.name || "open proposal"}
      </Link>
      . Only one can be open at a time.
    </Alert>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box sx={{ display: "flex", gap: 2, alignItems: "flex-start", justifyContent: "space-between", flexWrap: "wrap" }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0, flex: 1 }}>
          <Typography variant="h6" sx={{ fontFamily: "monospace", fontWeight: 700 }}>
            {template.number}
          </Typography>
          <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
            <Chip
              size="small"
              color={standardChangeTemplateStateColor(template.state)}
              label={standardChangeTemplateStateLabel(template.state)}
            />
            {current && <Chip size="small" variant="outlined" label={`Version ${current.version}`} />}
            {template.reviewRequired && <Chip size="small" variant="outlined" color="warning" label="Review needed" />}
          </Box>
          <Typography variant="h5">{template.name}</Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
          {/* The create form is behind write access (RequireWriteAccess), like every "Create" of Operations. */}
          {canWrite && (
            <Button
              variant="contained"
              size="small"
              startIcon={<FilePlus size={16} />}
              disabled={template.state !== "active"}
              onClick={() => navigate(raiseFromTemplatePath(template.id), { state: { from: here } })}
            >
              Raise a change request
            </Button>
          )}
          {canWrite && template.state !== "retired" && (
            <>
              <Button
                variant="outlined"
                size="small"
                startIcon={<PencilLine size={16} />}
                disabled={busy || !!template.openProposal}
                onClick={() => propose("modify")}
              >
                Propose a revision
              </Button>
              <Button
                variant="outlined"
                size="small"
                color="error"
                startIcon={<Archive size={16} />}
                disabled={busy || !!template.openProposal}
                onClick={() => propose("retire")}
              >
                Propose retirement
              </Button>
              {template.state === "active" ? (
                <Button
                  variant="outlined"
                  size="small"
                  color="warning"
                  startIcon={<Pause size={16} />}
                  disabled={busy}
                  onClick={() => setConfirm("suspend")}
                >
                  Suspend
                </Button>
              ) : (
                <Button
                  variant="outlined"
                  size="small"
                  startIcon={<Play size={16} />}
                  disabled={busy}
                  onClick={() => setConfirm("resume")}
                >
                  Resume
                </Button>
              )}
            </>
          )}
        </Box>
      </Box>

      {actionError && (
        <Alert severity="error" onClose={() => setActionError(null)}>
          {actionError}
        </Alert>
      )}
      {template.state === "suspended" && (
        <Alert severity="warning">
          Suspended: no change request can be raised from this template, and one already raised from it cannot be
          sent for approval, until it is resumed.
        </Alert>
      )}
      {template.state === "retired" && (
        <Alert severity="info">Retired: this template is kept as a record and can no longer be used.</Alert>
      )}
      {openProposalNote}

      {template.reviewRequired && (
        <Alert
          severity="warning"
          role="region"
          aria-label="Review needed"
          action={
            canWrite ? (
              <Button
                color="inherit"
                size="small"
                disabled={busy}
                onClick={() => {
                  setActionError(null);
                  clearReview.mutate(template.id, { onError: onError("Could not clear the review flag.") });
                }}
              >
                Clear review
              </Button>
            ) : undefined
          }
        >
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            Review needed{template.reviewRequiredOn ? ` since ${formatDateTime(template.reviewRequiredOn)}` : ""}
          </Typography>
          <Typography variant="body2">
            {template.reviewRequiredReason ?? "A change request raised from this template was rolled back or deviated from its plan."}
          </Typography>
          {template.reviewRequiredChangeRequest && (
            <Link
              component="button"
              variant="body2"
              onClick={() =>
                navigate(`/operations/change-requests/${encodeURIComponent(template.reviewRequiredChangeRequest!.id)}`, {
                  state: { from: here },
                })
              }
            >
              {template.reviewRequiredChangeRequest.name || "Open the change request"}
            </Link>
          )}
          <Typography variant="caption" sx={{ display: "block", mt: 0.5 }}>
            The template can still be used. Propose a revision (approving one clears the flag), or clear it once
            reviewed.
          </Typography>
        </Alert>
      )}

      <Card sx={{ p: 2.5 }}>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))", md: "repeat(4, minmax(0, 1fr))" },
          }}
        >
          <Meta label="Owner">{template.owner?.name || template.owner?.email || "—"}</Meta>
          <Meta label="Current version">
            {current ? `${current.version}, approved ${formatDateTime(current.approvedOn)}` : "—"}
          </Meta>
          <Meta label="Change requests raised">
            {`${template.usage.total} (${template.usage.open} open, ${template.usage.closed} closed, ${template.usage.rolledBack} rolled back, ${template.usage.canceled} canceled)`}
          </Meta>
          <Meta label="Last used">{formatDateTime(template.usage.lastUsedOn)}</Meta>
        </Box>
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
            {viewingOlder
              ? `Version ${olderVersion.data?.version ?? ""} (not the current version)`
              : `What the template specifies${current ? ` (version ${current.version})` : ""}`}
          </Typography>
          {viewingOlder && (
            <Button size="small" variant="text" onClick={() => setVersionId(null)}>
              Back to the current version
            </Button>
          )}
        </Box>
        {viewingOlder && olderVersion.isLoading ? (
          <Skeleton variant="rounded" height={240} />
        ) : viewingOlder && olderVersion.isError ? (
          <QueryErrorState
            message={errorText(olderVersion.error, "Failed to load the version.")}
            error={olderVersion.error}
            onRetry={() => void olderVersion.refetch()}
          />
        ) : shown ? (
          <>
            <Typography variant="body2" color="text.secondary">
              Highlighted {"{{placeholders}}"} are filled in from the change specifics when a change request is raised;
              everything else is fixed.
            </Typography>
            <StandardChangeContentView content={shown} />
            <Typography variant="subtitle2">Parameters</Typography>
            <StandardChangeParametersTable parameters={shown.parameters ?? []} />
          </>
        ) : (
          <Typography variant="body2" color="text.secondary">
            This template has no approved version.
          </Typography>
        )}
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
          Version history
        </Typography>
        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
          <TableContainer>
            <Table size="small" aria-label="Version history" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
              <TableHead>
                <TableRow sx={{ bgcolor: "action.hover" }}>
                  <TableCell sx={{ width: 90 }}>Version</TableCell>
                  <TableCell>Approved</TableCell>
                  <TableCell>Approved by</TableCell>
                  <TableCell>Proposal</TableCell>
                  <TableCell align="right" sx={{ width: 140 }} />
                </TableRow>
              </TableHead>
              <TableBody>
                {(template.versions ?? []).map((v) => {
                  const isCurrent = v.id === current?.id;
                  const isShown = isCurrent ? !viewingOlder : versionId === v.id;
                  return (
                    <TableRow key={v.id} selected={isShown}>
                      <TableCell>
                        {v.version}
                        {isCurrent && " (current)"}
                      </TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDateTime(v.approvedOn)}</TableCell>
                      <TableCell sx={{ maxWidth: 240, wordBreak: "break-word" }}>{v.approvedBy || "—"}</TableCell>
                      <TableCell>
                        {v.proposal ? (
                          <Link
                            component="button"
                            variant="body2"
                            onClick={() => navigate(standardChangeProposalPath(v.proposal!.id), { state: { from: here } })}
                          >
                            {v.proposal.name || "Proposal"}
                          </Link>
                        ) : (
                          "—"
                        )}
                      </TableCell>
                      <TableCell align="right">
                        {!isShown && (
                          <Button
                            size="small"
                            variant="text"
                            aria-label={`View version ${v.version}`}
                            onClick={() => setVersionId(isCurrent ? null : v.id)}
                          >
                            View
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      </Card>

      <Dialog open={confirm !== null} onClose={() => setConfirm(null)} aria-labelledby="std-confirm-title">
        <DialogTitle id="std-confirm-title">
          {confirm === "suspend" ? `Suspend ${template.number}?` : `Resume ${template.number}?`}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {confirm === "suspend"
              ? "No change request can be raised from a suspended template, and one already raised from it cannot be sent for approval. It can be resumed later."
              : "Change requests can be raised from it again, following its current version."}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setConfirm(null)}>
            Go back
          </Button>
          <Button variant="contained" color={confirm === "suspend" ? "warning" : "primary"} onClick={runConfirm}>
            {confirm === "suspend" ? "Suspend" : "Resume"}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
