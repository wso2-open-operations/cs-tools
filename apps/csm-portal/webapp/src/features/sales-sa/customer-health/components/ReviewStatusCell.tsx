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

// Ported from apps/support-portal-lite/webapp's ReviewStatusCell.tsx.
// Its own useHealthToast (a domain-local success/error toast, invented
// because one-wso2 had no global equivalent) is dropped here in favor of
// this app's own already-mounted SuccessBannerProvider/ErrorBannerProvider
// (see App.tsx) — no need for a second, parallel notification system.
import { useState } from "react";
import {
  Box, Typography, Button, Dialog, DialogTitle, DialogContent, DialogActions,
  TextField, Stack, Divider, Chip, Collapse, Tooltip,
} from "@wso2/oxygen-ui";
import { alpha, useTheme } from "@mui/material/styles";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useOpenProjectRisk, useMarkProjectHealthy, useCloseProjectRisk, useProjectRiskHistory, closeProjectRiskErrorMessage } from "../api/useProjectRisk";
import CircularLoading from "./CircularLoading";
import type { ProjectDetail, ProjectHealthStatus } from "../api/customerHealthTypes";

interface ReviewStatusCellProps {
  project: ProjectDetail;
  accountSysId: string;
  healthStatus: ProjectHealthStatus | null;
  onStatusChanged: () => void;
  onAddActionItem?: () => void;
}

function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return "";
  return new Date(dateStr).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

// The backend has no dedicated "healthy review" record type — a healthy
// review is just a risk-history row the backend inserts with this exact
// literal as its openedComment (csm-portal-backend's
// internal/risk/health.go, MarkProjectHealthy). Named here so the six
// comparisons below can't drift out of sync with each other on a wording
// change.
const HEALTHY_REVIEW_COMMENT = "Marked as healthy";

export default function ReviewStatusCell({
  project, accountSysId, healthStatus, onStatusChanged, onAddActionItem,
}: ReviewStatusCellProps) {
  const theme = useTheme();
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const openRiskMutation = useOpenProjectRisk();
  const markHealthyMutation = useMarkProjectHealthy();
  const closeRiskMutation = useCloseProjectRisk();
  const loading = openRiskMutation.isPending || markHealthyMutation.isPending || closeRiskMutation.isPending;

  const [markAtRiskOpen, setMarkAtRiskOpen] = useState(false);
  const [closeRiskOpen, setCloseRiskOpen] = useState(false);
  const [markHealthyOpen, setMarkHealthyOpen] = useState(false);
  const [postCloseOpen, setPostCloseOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);

  const [atRiskComment, setAtRiskComment] = useState("");
  const [markHealthyComment, setMarkHealthyComment] = useState("");
  const [closeRiskComment, setCloseRiskComment] = useState("");
  const [expandedActionItems, setExpandedActionItems] = useState<Set<number>>(new Set());

  const { data: historyData, isLoading: historyLoading } = useProjectRiskHistory(project.sysId, historyOpen);

  const status = healthStatus?.healthStatus?.status ?? "to_be_reviewed";
  const openRisk = healthStatus?.openRisk ?? null;
  const hasOpenItems = (openRisk?.actionItems ?? []).some((item) => item.status === "open" || item.status === "in_progress");

  // Blocks the one input value that would make this risk indistinguishable
  // from a healthy review in the history view below (see
  // HEALTHY_REVIEW_COMMENT) -- an exact, case-sensitive match, since that's
  // the same comparison the history view itself uses.
  const atRiskCommentReserved = atRiskComment.trim() === HEALTHY_REVIEW_COMMENT;

  const handleMarkAtRisk = async () => {
    if (!atRiskComment.trim() || atRiskCommentReserved) return;
    try {
      await openRiskMutation.mutateAsync({ projectSysId: project.sysId, accountSysId, comment: atRiskComment });
      showSuccess(`${project.name} marked as at risk.`);
      setMarkAtRiskOpen(false);
      setAtRiskComment("");
      onStatusChanged();
    } catch {
      showError("Failed to mark project as at risk.");
    }
  };

  const handleMarkHealthy = async () => {
    if (!markHealthyComment.trim()) return;
    try {
      await markHealthyMutation.mutateAsync({ projectSysId: project.sysId, accountSysId, comment: markHealthyComment.trim() });
      showSuccess(`${project.name} marked as healthy.`);
      setMarkHealthyOpen(false);
      setMarkHealthyComment("");
      onStatusChanged();
    } catch {
      showError("Failed to mark project as healthy.");
    }
  };

  const handleCloseRisk = async () => {
    if (!closeRiskComment.trim()) return;
    try {
      await closeRiskMutation.mutateAsync({ riskId: openRisk!.id, comment: closeRiskComment });
      showSuccess(`Risk closed for ${project.name}.`);
      setCloseRiskOpen(false);
      setCloseRiskComment("");
      setPostCloseOpen(true);
      onStatusChanged();
    } catch (err) {
      showError(closeProjectRiskErrorMessage(err, "Failed to close risk."));
    }
  };

  const handlePostCloseLeave = () => {
    setPostCloseOpen(false);
    onStatusChanged();
  };

  const handleOpenHistory = () => {
    setHistoryOpen(true);
  };

  const riskCycleCount = historyData ? historyData.filter((r) => r.openedComment !== HEALTHY_REVIEW_COMMENT).length : 0;
  const healthyReviewCount = historyData ? historyData.filter((r) => r.openedComment === HEALTHY_REVIEW_COMMENT).length : 0;

  const riskNumberMap = new Map<number, number>();
  if (historyData) {
    const sortedAsc = [...historyData]
      .filter((r) => r.openedComment !== HEALTHY_REVIEW_COMMENT)
      .sort((a, b) => new Date(a.openedOn).getTime() - new Date(b.openedOn).getTime());
    sortedAsc.forEach((r, idx) => riskNumberMap.set(r.id, idx + 1));
  }

  const displayHistory = historyData
    ? [...historyData].sort((a, b) => {
        const aIsOpen = a.status === "open" && a.openedComment !== HEALTHY_REVIEW_COMMENT;
        const bIsOpen = b.status === "open" && b.openedComment !== HEALTHY_REVIEW_COMMENT;
        if (aIsOpen && !bIsOpen) return -1;
        if (!aIsOpen && bIsOpen) return 1;
        return new Date(b.openedOn).getTime() - new Date(a.openedOn).getTime();
      })
    : [];

  return (
    <Box>
      <CircularLoading openLoading={loading} />
      <Box>
        {status === "to_be_reviewed" && (
          <>
            <Typography variant="caption" color="text.secondary">To Be Reviewed</Typography>
            <Box sx={{ mt: 2, maxWidth: 200, mx: "auto" }}>
              <Stack spacing={1}>
                <Button size="small" fullWidth variant="contained" sx={{ backgroundColor: "#EF5350", "&:hover": { backgroundColor: "#D32F2F" }, color: "#fff", fontWeight: 600, textTransform: "none", py: 0.75 }} onClick={() => setMarkAtRiskOpen(true)}>
                  Mark At Risk
                </Button>
                <Button size="small" fullWidth variant="contained" sx={{ backgroundColor: "#66BB6A", "&:hover": { backgroundColor: "#43A047" }, color: "#fff", fontWeight: 600, textTransform: "none", py: 0.75 }} onClick={() => setMarkHealthyOpen(true)}>
                  Mark Healthy
                </Button>
                <Button size="small" fullWidth variant="outlined" onClick={handleOpenHistory} sx={{ textTransform: "none", borderColor: "#e96900", color: "#e96900", "&:hover": { backgroundColor: alpha("#e96900", 0.12), borderColor: "#e96900" } }}>
                  View Health History
                </Button>
              </Stack>
            </Box>
          </>
        )}
        {status === "at_risk" && (
          <>
            <Chip label="At Risk" size="small" sx={{ fontWeight: 600, borderRadius: "16px", backgroundColor: "transparent", border: `2px solid ${theme.palette.error.main}`, color: theme.palette.error.main, boxShadow: `0 0 8px ${alpha(theme.palette.error.main, 0.3)}`, "&:hover": { backgroundColor: "transparent" } }} />
            {openRisk && (
              <>
                <Box sx={{ mt: 2, p: 1.5, borderLeft: `3px solid ${theme.palette.error.main}`, backgroundColor: alpha(theme.palette.error.main, 0.08), borderRadius: "0 4px 4px 0", overflow: "hidden", minWidth: 0 }}>
                  <Typography variant="body2" sx={{ color: theme.palette.error.main, display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden", wordBreak: "break-word", overflowWrap: "break-word", whiteSpace: "normal", maxWidth: "100%", lineHeight: 1.5 }}>
                    {openRisk.openedComment}
                  </Typography>
                  <Button size="small" variant="text" sx={{ p: 0, minWidth: 0, fontSize: "0.7rem", color: theme.palette.error.main, "&:hover": { backgroundColor: "transparent", textDecoration: "underline" } }} onClick={handleOpenHistory}>
                    Show more
                  </Button>
                  <Typography variant="caption" display="block" mt={0.75} color="text.secondary">
                    {openRisk.openedByEmail} · {formatDate(openRisk.openedOn)}
                  </Typography>
                </Box>

                <Divider sx={{ mt: 2, mb: 1 }} />

                <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1.5 }}>
                  {(openRisk.actionItems ?? []).filter((i) => i.status === "open").length} open ·{" "}
                  {(openRisk.actionItems ?? []).filter((i) => i.status === "in_progress").length} in progress
                </Typography>

                <Box sx={{ maxWidth: 200, mx: "auto" }}>
                  <Stack spacing={1}>
                    {onAddActionItem && (
                      <Button size="small" fullWidth variant="outlined" sx={{ textTransform: "none", color: "#e96900", borderColor: "#e96900", "&:hover": { backgroundColor: alpha("#e96900", 0.12), borderColor: "#e96900" } }} onClick={onAddActionItem}>
                        + Add Action Item
                      </Button>
                    )}
                    <Button size="small" fullWidth variant="outlined" sx={{ textTransform: "none", color: "text.secondary", borderColor: "divider", "&:hover": { backgroundColor: "action.hover", borderColor: "text.secondary" } }} onClick={() => document.getElementById("action-items-section")?.scrollIntoView({ behavior: "smooth" })}>
                      View All Action Items
                    </Button>
                    <Tooltip title={hasOpenItems ? "Complete all open and in-progress action items for this project before closing the risk." : ""} arrow>
                      <span style={{ display: "block" }}>
                        <Button size="small" fullWidth variant="outlined" color="error" disabled={hasOpenItems} onClick={() => setCloseRiskOpen(true)} sx={{ textTransform: "none" }}>
                          Close Risk
                        </Button>
                      </span>
                    </Tooltip>
                    <Button size="small" fullWidth variant="outlined" onClick={handleOpenHistory} sx={{ textTransform: "none", borderColor: "#e96900", color: "#e96900", "&:hover": { backgroundColor: alpha("#e96900", 0.12), borderColor: "#e96900" } }}>
                      View Health History
                    </Button>
                  </Stack>
                </Box>
              </>
            )}
          </>
        )}

        {status === "healthy" && (
          <>
            <Chip label="Healthy" size="small" sx={{ fontWeight: 600, borderRadius: "16px", backgroundColor: "transparent", border: `2px solid ${theme.palette.success.main}`, color: theme.palette.success.main, boxShadow: `0 0 8px ${alpha(theme.palette.success.main, 0.3)}`, "&:hover": { backgroundColor: "transparent" } }} />
            {healthStatus?.healthStatus && (
              <Typography variant="caption" display="block" mt={2} color="text.secondary">
                {healthStatus.healthStatus.reviewedByEmail} · {formatDate(healthStatus.healthStatus.reviewedOn)}
              </Typography>
            )}
            <Box sx={{ mt: 2, maxWidth: 200, mx: "auto" }}>
              <Stack spacing={1}>
                <Button size="small" fullWidth variant="contained" sx={{ backgroundColor: "#EF5350", "&:hover": { backgroundColor: "#D32F2F" }, color: "#fff", fontWeight: 600, textTransform: "none", py: 0.75 }} onClick={() => setMarkAtRiskOpen(true)}>
                  Mark At Risk
                </Button>
                <Button size="small" fullWidth variant="outlined" onClick={handleOpenHistory} sx={{ textTransform: "none", borderColor: "#e96900", color: "#e96900", "&:hover": { backgroundColor: alpha("#e96900", 0.12), borderColor: "#e96900" } }}>
                  View Health History
                </Button>
              </Stack>
            </Box>
          </>
        )}
      </Box>

      <Dialog open={markAtRiskOpen} onClose={() => setMarkAtRiskOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Mark {project.name} as At Risk</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            required
            multiline
            rows={4}
            placeholder="Add reason for marking at risk..."
            value={atRiskComment}
            onChange={(e) => setAtRiskComment(e.target.value)}
            error={atRiskCommentReserved}
            helperText={atRiskCommentReserved ? "This exact reason is reserved for healthy reviews -- please rephrase." : " "}
            sx={{ mt: 1 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setMarkAtRiskOpen(false); setAtRiskComment(""); }}>Cancel</Button>
          <Button onClick={handleMarkAtRisk} disabled={!atRiskComment.trim() || atRiskCommentReserved} sx={{ color: "warning.main" }}>Confirm</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={markHealthyOpen} onClose={() => { setMarkHealthyOpen(false); setMarkHealthyComment(""); }} maxWidth="sm" fullWidth>
        <DialogTitle>Mark {project.name} as Healthy</DialogTitle>
        <DialogContent>
          <TextField fullWidth required multiline rows={4} placeholder="Add reason for marking healthy..." value={markHealthyComment} onChange={(e) => setMarkHealthyComment(e.target.value)} sx={{ mt: 1 }} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setMarkHealthyOpen(false); setMarkHealthyComment(""); }}>Cancel</Button>
          <Button onClick={handleMarkHealthy} disabled={!markHealthyComment.trim()} sx={{ color: "success.main" }}>Confirm</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={closeRiskOpen} onClose={() => setCloseRiskOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Close Risk for {project.name}</DialogTitle>
        <DialogContent>
          <TextField fullWidth required multiline rows={4} placeholder="Add reason for closing risk..." value={closeRiskComment} onChange={(e) => setCloseRiskComment(e.target.value)} sx={{ mt: 1 }} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setCloseRiskOpen(false); setCloseRiskComment(""); }}>Cancel</Button>
          <Button onClick={handleCloseRisk} disabled={!closeRiskComment.trim()}>Confirm</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={postCloseOpen} onClose={handlePostCloseLeave} maxWidth="sm" fullWidth>
        <DialogTitle>Mark Project as Healthy?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            This project's risk has been closed. Would you like to mark it as Healthy, or leave it as To Be Reviewed?
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={handlePostCloseLeave}>Leave as To Be Reviewed</Button>
          <Button onClick={() => { setPostCloseOpen(false); setMarkHealthyOpen(true); }} sx={{ color: "success.main" }}>Mark Healthy</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={historyOpen} onClose={() => { setHistoryOpen(false); setExpandedActionItems(new Set()); }} maxWidth="md" fullWidth>
        <DialogTitle>
          Health Status History — {project.name}
          <Typography variant="body2" color="text.secondary" mt={0.5}>
            {riskCycleCount} risk cycle(s) · {healthyReviewCount} healthy review(s)
          </Typography>
        </DialogTitle>
        <DialogContent>
          {historyLoading && <Typography variant="body2">Loading...</Typography>}
          {!historyLoading && (!historyData || historyData.length === 0) && (
            <Typography variant="body2" color="text.secondary">No history for this project.</Typography>
          )}
          {!historyLoading && historyData && historyData.length > 0 && (
            <Box sx={{ pt: 1 }}>
              {displayHistory.map((risk, idx, arr) => {
                const isHealthyRecord = risk.openedComment === HEALTHY_REVIEW_COMMENT;
                const isLast = idx === arr.length - 1;
                const dotColor = isHealthyRecord ? theme.palette.success.main : theme.palette.error.main;
                return (
                  <Box key={risk.id} sx={{ display: "flex", mb: 2.5 }}>
                    <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", mr: 2, flexShrink: 0, width: 20 }}>
                      <Box sx={{ width: 14, height: 14, borderRadius: "50%", bgcolor: dotColor, mt: "20px", flexShrink: 0 }} />
                      {!isLast && <Box sx={{ width: 2, flex: 1, minHeight: 16, backgroundColor: "divider", mt: 0.5 }} />}
                    </Box>
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      {isHealthyRecord ? (
                        <Box sx={{ border: `1px solid ${alpha(theme.palette.success.main, 0.3)}`, borderLeft: `3px solid ${theme.palette.success.main}`, borderRadius: 1, backgroundColor: alpha(theme.palette.success.main, 0.08), p: 2 }}>
                          <Stack direction="row" spacing={1} alignItems="center" mb={0.5}>
                            <Chip label="Healthy" size="small" sx={{ bgcolor: theme.palette.success.main, color: "#fff", fontWeight: 700 }} />
                            <Typography variant="body2" color="text.secondary">{formatDate(risk.openedOn)}</Typography>
                          </Stack>
                          <Typography variant="body2">
                            Reviewed by: <strong style={{ color: theme.palette.info.main }}>{risk.openedByEmail}</strong>
                          </Typography>
                          {risk.closedComment && risk.closedComment !== "Reviewed and confirmed healthy" && (
                            <Box sx={{ borderLeft: `3px solid ${theme.palette.success.main}`, backgroundColor: alpha(theme.palette.success.main, 0.08), p: 1, borderRadius: "0 4px 4px 0", mt: 1 }}>
                              <Typography variant="body2" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word", lineHeight: 1.6 }}>
                                {risk.closedComment}
                              </Typography>
                            </Box>
                          )}
                        </Box>
                      ) : (
                        <Box sx={{ border: `1px solid ${alpha(theme.palette.error.main, 0.3)}`, borderLeft: `3px solid ${theme.palette.error.main}`, borderRadius: 1, overflow: "hidden" }}>
                          <Box sx={{ p: 2 }}>
                            <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: "0.05em", display: "block", mb: 0.5 }}>
                              Risk #{riskNumberMap.get(risk.id)}
                            </Typography>
                            {risk.status === "open" && (
                              <Box sx={{ mb: 0.75 }}>
                                <Chip label="Currently Open" size="small" sx={{
                                  bgcolor: theme.palette.error.main, color: "#fff", fontWeight: 700,
                                  animation: "riskPulse 1.5s ease-in-out infinite",
                                  "@keyframes riskPulse": {
                                    "0%": { boxShadow: "0 0 0 0 rgba(211,47,47,0.5)" },
                                    "70%": { boxShadow: "0 0 0 7px rgba(211,47,47,0)" },
                                    "100%": { boxShadow: "0 0 0 0 rgba(211,47,47,0)" },
                                  },
                                }} />
                              </Box>
                            )}
                            <Typography variant="body2" color="text.secondary" sx={{ mb: 0.5 }}>
                              Opened on {formatDate(risk.openedOn)}
                            </Typography>
                            <Typography variant="body2">
                              By: <strong style={{ color: theme.palette.info.main }}>{risk.openedByEmail}</strong>
                            </Typography>
                            <Box sx={{ borderLeft: `3px solid ${theme.palette.error.main}`, backgroundColor: alpha(theme.palette.error.main, 0.08), p: 1, borderRadius: "0 4px 4px 0", mt: 1 }}>
                              <Typography variant="body2" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word", lineHeight: 1.6 }}>
                                <strong>Reason:</strong> {risk.openedComment}
                              </Typography>
                            </Box>
                          </Box>
                          {risk.closedOn && (
                            <>
                              <Divider />
                              <Box sx={{ p: 2, backgroundColor: "action.hover" }}>
                                <Typography variant="body2" sx={{ mb: 0.5 }}>
                                  Risk #{riskNumberMap.get(risk.id)} was closed on <strong>{formatDate(risk.closedOn)}</strong>
                                </Typography>
                                <Typography variant="body2">
                                  By: <strong style={{ color: theme.palette.info.main }}>{risk.closedByEmail}</strong>
                                </Typography>
                                {risk.closedComment && (
                                  <Box sx={{ borderLeft: `3px solid ${theme.palette.text.disabled}`, backgroundColor: "action.hover", p: 1, borderRadius: "0 4px 4px 0", mt: 1 }}>
                                    <Typography variant="body2" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word", lineHeight: 1.6 }}>
                                      <strong>Resolution:</strong> {risk.closedComment}
                                    </Typography>
                                  </Box>
                                )}
                              </Box>
                            </>
                          )}
                          {risk.actionItems.length > 0 && (
                            <>
                              <Divider />
                              <Box sx={{ p: 2, backgroundColor: "action.hover" }}>
                                <Button
                                  size="small"
                                  variant="text"
                                  sx={{ p: 0, textTransform: "none", fontWeight: 600, color: "text.primary" }}
                                  onClick={() => {
                                    setExpandedActionItems((prev) => {
                                      const next = new Set(prev);
                                      if (next.has(risk.id)) next.delete(risk.id);
                                      else next.add(risk.id);
                                      return next;
                                    });
                                  }}
                                >
                                  Action Items ({risk.actionItems.length}) {expandedActionItems.has(risk.id) ? "▴" : "▾"}
                                </Button>
                                <Collapse in={expandedActionItems.has(risk.id)}>
                                  <Box sx={{ mt: 1 }}>
                                    {risk.actionItems.map((item) => (
                                      <Box key={item.id} sx={{ mb: 1 }}>
                                        <Stack direction="row" spacing={1} alignItems="center">
                                          <Typography variant="body2" fontWeight={600}>{item.title}</Typography>
                                          <Chip label={item.status} size="small" color={item.status === "resolved" ? "success" : "default"} />
                                        </Stack>
                                        {item.resolutionComment && (
                                          <Typography variant="caption" display="block" color="text.secondary" mt={0.25}>
                                            {item.resolutionComment}
                                          </Typography>
                                        )}
                                      </Box>
                                    ))}
                                  </Box>
                                </Collapse>
                              </Box>
                            </>
                          )}
                        </Box>
                      )}
                    </Box>
                  </Box>
                );
              })}
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setHistoryOpen(false); setExpandedActionItems(new Set()); }}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
