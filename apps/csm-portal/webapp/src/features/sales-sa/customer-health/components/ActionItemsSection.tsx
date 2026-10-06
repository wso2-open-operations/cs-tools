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

// Ported from apps/support-portal-lite/webapp's ActionItemsSection.tsx.
// Its own useHealthToast is dropped in favor of this app's already-mounted
// SuccessBannerProvider/ErrorBannerProvider — see ReviewStatusCell.tsx's
// same note.
import { useState, useEffect, useMemo, useRef } from "react";
import {
  Box, Typography, Paper, Stack, Button, Chip, Tabs, Tab, Select, MenuItem, FormControl, InputLabel,
  Dialog, DialogTitle, DialogContent, DialogActions, TextField, Tooltip,
} from "@wso2/oxygen-ui";
import { ClipboardListIcon, PlusIcon, MessageSquareIcon, ChevronDownIcon, ChevronUpIcon } from "@wso2/oxygen-ui-icons-react";
import { alpha, useTheme } from "@mui/material/styles";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  useAccountActionItems,
  useCreateActionItem,
  useUpdateActionItemStatus,
  useActionItemComments,
  useCreateActionItemComment,
} from "../api/useActionItems";
import CircularLoading from "./CircularLoading";
import type { ProjectDetail, ProjectHealthStatus, RiskActionItem } from "../api/customerHealthTypes";

const PRIORITY_BORDER: Record<string, string> = { high: "#d32f2f", medium: "#ed6c02", low: "#1976d2" };
const STATUS_LABELS: Record<string, string> = { open: "Open", in_progress: "In Progress", resolved: "Resolved", cancelled: "Cancelled" };

function getPriorityLabel(priority: string): string {
  const p = priority.toUpperCase();
  return p === "MEDIUM" ? "MED" : p;
}

function getPriorityChipColor(priority: string): "error" | "warning" | "info" | "default" {
  switch (priority.toLowerCase()) {
    case "high": return "error";
    case "medium": return "warning";
    case "low": return "info";
    default: return "default";
  }
}

function getStatusChipColor(status: string): "warning" | "info" | "success" | "default" {
  switch (status) {
    case "open": return "warning";
    case "in_progress": return "info";
    case "resolved": return "success";
    case "cancelled": return "default";
    default: return "default";
  }
}

function DueDateDisplay({ dueDate, status }: { dueDate: string; status: string }) {
  const [y, m, d] = dueDate.split("-").map(Number);
  const date = new Date(y, m - 1, d);
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const isOverdue = date < today && (status === "open" || status === "in_progress");
  return isOverdue ? (
    <Typography variant="caption" sx={{ color: "error.main" }}>{date.toLocaleDateString()} (Overdue)</Typography>
  ) : (
    <Typography variant="caption" color="text.secondary">{date.toLocaleDateString()}</Typography>
  );
}

interface ProjectWithRisk {
  sysId: string;
  name: string;
  riskId: number;
}

interface ActionItemsSectionProps {
  accountId: string;
  projects: ProjectDetail[];
  healthStatusData: ProjectHealthStatus[];
  onDataChanged?: () => void;
  openAddForProject?: string | null;
  onAddForProjectConsumed?: () => void;
}

interface AddActionItemDialogProps {
  open: boolean;
  onClose: () => void;
  accountId: string;
  projectsWithOpenRisks: ProjectWithRisk[];
  onCreated: () => void;
  defaultProjectSysId?: string;
}

function AddActionItemDialog({ open, onClose, accountId, projectsWithOpenRisks, onCreated, defaultProjectSysId }: AddActionItemDialogProps) {
  const createActionItem = useCreateActionItem();
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const [selectedProjectSysId, setSelectedProjectSysId] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("medium");
  const [assignedTo, setAssignedTo] = useState("");
  const [dueDate, setDueDate] = useState("");

  useEffect(() => {
    if (open) {
      setSelectedProjectSysId(defaultProjectSysId ?? projectsWithOpenRisks[0]?.sysId ?? "");
      setTitle("");
      setDescription("");
      setPriority("medium");
      setAssignedTo("");
      setDueDate("");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const selectedProject = projectsWithOpenRisks.find((p) => p.sysId === selectedProjectSysId);
  const canCreate = !!title.trim() && !!selectedProject && !!priority && !!dueDate;

  const handleCreate = async () => {
    if (!canCreate) return;
    try {
      await createActionItem.mutateAsync({
        riskId: selectedProject!.riskId,
        payload: {
          title: title.trim(),
          description: description.trim() || null,
          priority,
          assignedToEmail: assignedTo.trim() || null,
          dueDate,
          projectSysId: selectedProject!.sysId,
          accountSysId: accountId,
        },
      });
      showSuccess("Action item created successfully.");
      onCreated();
    } catch {
      showError("Failed to create action item.");
    }
  };

  return (
    <>
      <CircularLoading openLoading={createActionItem.isPending} />
      <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
        <DialogTitle>Add Action Item</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl fullWidth required>
              <InputLabel>Project</InputLabel>
              <Select value={selectedProjectSysId} label="Project" onChange={(e) => setSelectedProjectSysId(e.target.value)}>
                {projectsWithOpenRisks.map((p) => (
                  <MenuItem key={p.sysId} value={p.sysId}>{p.name}</MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField fullWidth required label="Title" value={title} onChange={(e) => setTitle(e.target.value)} />
            <TextField fullWidth label="Description" multiline rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
            <FormControl fullWidth required>
              <InputLabel>Priority</InputLabel>
              <Select value={priority} label="Priority" onChange={(e) => setPriority(e.target.value)}>
                <MenuItem value="high">High</MenuItem>
                <MenuItem value="medium">Medium</MenuItem>
                <MenuItem value="low">Low</MenuItem>
              </Select>
            </FormControl>
            <TextField fullWidth label="Assigned To (Email)" value={assignedTo} onChange={(e) => setAssignedTo(e.target.value)} />
            <TextField
              fullWidth required label="Due Date" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)}
              slotProps={{ inputLabel: { shrink: true } }}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>Cancel</Button>
          <Button onClick={handleCreate} disabled={!canCreate} sx={{ color: "#e96900" }}>Create</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

interface InlineCommentThreadProps {
  itemId: number;
  onCommentPosted: (itemId: number) => void;
}

function InlineCommentThread({ itemId, onCommentPosted }: InlineCommentThreadProps) {
  const theme = useTheme();
  const { showError } = useErrorBanner();
  const { data: comments, isLoading: loading } = useActionItemComments(itemId, true);
  const createComment = useCreateActionItemComment();
  const [newComment, setNewComment] = useState("");
  const commentsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    commentsEndRef.current?.scrollIntoView({ block: "end" });
  }, [comments]);

  const handlePost = async () => {
    if (!newComment.trim() || createComment.isPending) return;
    try {
      await createComment.mutateAsync({ actionItemId: itemId, comment: newComment.trim() });
      setNewComment("");
      onCommentPosted(itemId);
    } catch {
      showError("Failed to post comment.");
    }
  };

  const formatCommentDate = (dateStr: string) => {
    const d = new Date(dateStr);
    return (
      d.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" }) +
      " · " +
      d.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit", hour12: true })
    );
  };

  return (
    <Box sx={{ borderTop: `1px solid ${theme.palette.divider}` }}>
      <Box sx={{ maxHeight: 300, overflowY: "auto", px: 2, py: 1 }}>
        {loading ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>Loading comments...</Typography>
        ) : !comments || comments.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>No comments yet. Be the first to comment.</Typography>
        ) : (
          <>
            {comments.map((c, idx) => (
              <Box key={c.id} sx={{ py: 1.25, borderTop: idx > 0 ? `1px solid ${theme.palette.divider}` : "none" }}>
                <Stack direction="row" alignItems="baseline" spacing={1}>
                  <Typography variant="body2" fontWeight={700}>{c.createdByEmail}</Typography>
                  <Typography variant="caption" color="text.secondary">{formatCommentDate(c.createdOn)}</Typography>
                </Stack>
                <Typography variant="body2" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{c.comment}</Typography>
              </Box>
            ))}
            <div ref={commentsEndRef} />
          </>
        )}
      </Box>
      <Box sx={{ px: 2, pb: 1.5, display: "flex", gap: 1, alignItems: "flex-end" }}>
        <TextField
          fullWidth multiline maxRows={3} size="small" placeholder="Add a comment..."
          value={newComment} onChange={(e) => setNewComment(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void handlePost(); } }}
        />
        <Button variant="contained" disabled={!newComment.trim() || createComment.isPending} onClick={handlePost} sx={{ minWidth: 64, flexShrink: 0, bgcolor: "#e96900", "&:hover": { bgcolor: "#c85a00" } }}>
          {createComment.isPending ? "..." : "Post"}
        </Button>
      </Box>
    </Box>
  );
}

export default function ActionItemsSection({
  accountId, projects, healthStatusData, onDataChanged, openAddForProject, onAddForProjectConsumed,
}: ActionItemsSectionProps) {
  const theme = useTheme();
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const updateStatus = useUpdateActionItemStatus();
  const [activeTab, setActiveTab] = useState("all");
  const [selectedProjectSysId, setSelectedProjectSysId] = useState("");

  const [addOpen, setAddOpen] = useState(false);
  const [addDefaultProject, setAddDefaultProject] = useState<string | undefined>(undefined);

  const [statusDialogOpen, setStatusDialogOpen] = useState(false);
  const [pendingChange, setPendingChange] = useState<{ itemId: number; newStatus: string } | null>(null);
  const [statusComment, setStatusComment] = useState("");

  const [commentCounts, setCommentCounts] = useState<Record<string, number>>({});
  const [expandedItems, setExpandedItems] = useState<Set<number>>(new Set());

  const { data: actionItems, isLoading: itemsLoading, refetch: refetchActionItems } = useAccountActionItems(accountId);

  useEffect(() => {
    if (openAddForProject) {
      setAddDefaultProject(openAddForProject);
      setAddOpen(true);
      onAddForProjectConsumed?.();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openAddForProject]);

  useEffect(() => {
    if (actionItems) {
      const counts: Record<string, number> = {};
      actionItems.forEach((item: RiskActionItem) => { counts[item.id] = item.commentCount ?? 0; });
      setCommentCounts(counts);
    }
  }, [actionItems]);

  const projectsWithOpenRisks = useMemo<ProjectWithRisk[]>(() => {
    return healthStatusData
      .filter((s) => s.healthStatus.status === "at_risk" && s.openRisk !== null)
      .map((s) => ({
        sysId: s.projectSysId,
        name: projects.find((p) => p.sysId === s.projectSysId)?.name ?? s.projectSysId,
        riskId: s.openRisk!.id,
      }));
  }, [healthStatusData, projects]);

  const projectFilteredItems = useMemo(() => {
    if (!actionItems) return [];
    if (!selectedProjectSysId) return actionItems;
    return actionItems.filter((i) => i.projectSysId === selectedProjectSysId);
  }, [actionItems, selectedProjectSysId]);

  const counts = useMemo(() => ({
    all: projectFilteredItems.length,
    open: projectFilteredItems.filter((i) => i.status === "open").length,
    in_progress: projectFilteredItems.filter((i) => i.status === "in_progress").length,
    resolved: projectFilteredItems.filter((i) => i.status === "resolved").length,
    cancelled: projectFilteredItems.filter((i) => i.status === "cancelled").length,
  }), [projectFilteredItems]);

  const displayedItems = useMemo(() => {
    if (activeTab === "all") return projectFilteredItems;
    return projectFilteredItems.filter((i) => i.status === activeTab);
  }, [projectFilteredItems, activeTab]);

  const refresh = () => {
    void refetchActionItems();
    onDataChanged?.();
  };

  const getProjectName = (projectSysId: string) => projects.find((p) => p.sysId === projectSysId)?.name ?? projectSysId;

  const handleCommentPosted = (itemId: number) => {
    setCommentCounts((prev) => ({ ...prev, [itemId]: (prev[itemId] ?? 0) + 1 }));
  };

  const toggleExpanded = (itemId: number) => {
    setExpandedItems((prev) => {
      const next = new Set(prev);
      if (next.has(itemId)) next.delete(itemId);
      else next.add(itemId);
      return next;
    });
  };

  const handleStatusChange = async (itemId: number, newStatus: string) => {
    if (newStatus === "resolved" || newStatus === "cancelled") {
      setPendingChange({ itemId, newStatus });
      setStatusComment("");
      setStatusDialogOpen(true);
      return;
    }
    try {
      await updateStatus.mutateAsync({ actionItemId: itemId, status: newStatus, resolutionComment: null });
      showSuccess("Status updated.");
      refresh();
    } catch {
      showError("Failed to update status.");
    }
  };

  const handleStatusChangeConfirm = async () => {
    if (!pendingChange || !statusComment.trim()) return;
    try {
      await updateStatus.mutateAsync({ actionItemId: pendingChange.itemId, status: pendingChange.newStatus, resolutionComment: statusComment });
      showSuccess("Status updated.");
      setStatusDialogOpen(false);
      setPendingChange(null);
      setStatusComment("");
      refresh();
    } catch {
      showError("Failed to update status.");
    }
  };

  return (
    <Box id="action-items-section" sx={{ mt: 3 }}>
      <CircularLoading openLoading={updateStatus.isPending} />

      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }} flexWrap="wrap">
        <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
          <Box sx={{ color: "#e96900", display: "inline-flex" }}>
            <ClipboardListIcon size={20} />
          </Box>
          <Typography variant="h6" fontWeight="bold">Risk Mitigation Action Items</Typography>
          <Typography variant="body2" color="text.secondary">
            {counts.open} open · {counts.in_progress} in progress · {counts.resolved} resolved · {counts.cancelled} cancelled
          </Typography>
        </Stack>
        <Tooltip title={projectsWithOpenRisks.length === 0 ? "Action items can only be created when there's an active risk" : ""}>
          <span>
            <Button
              variant="outlined" startIcon={<PlusIcon size={16} />} onClick={() => setAddOpen(true)}
              disabled={projectsWithOpenRisks.length === 0}
              sx={{ color: "#e96900", borderColor: "#e96900", ":hover": { bgcolor: alpha("#e96900", 0.12), borderColor: "#e96900" } }}
            >
              Add Action Item
            </Button>
          </span>
        </Tooltip>
      </Stack>

      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }} flexWrap="wrap" gap={1}>
        <Tabs
          value={activeTab}
          onChange={(_, v: string) => setActiveTab(v)}
          textColor="inherit"
          sx={{ minHeight: 36, "& .MuiTab-root": { minHeight: 36, py: 0.5, fontWeight: 600, fontSize: "0.75rem" } }}
        >
          <Tab label={`ALL (${counts.all})`} value="all" />
          <Tab label={`OPEN (${counts.open})`} value="open" />
          <Tab label={`IN PROGRESS (${counts.in_progress})`} value="in_progress" />
          <Tab label={`RESOLVED (${counts.resolved})`} value="resolved" />
          <Tab label={`CANCELLED (${counts.cancelled})`} value="cancelled" />
        </Tabs>
        <FormControl size="small" sx={{ minWidth: 180 }}>
          <InputLabel>All Projects</InputLabel>
          <Select value={selectedProjectSysId} label="All Projects" onChange={(e) => setSelectedProjectSysId(e.target.value)}>
            <MenuItem value="">All Projects</MenuItem>
            {projectsWithOpenRisks.map((p) => (
              <MenuItem key={p.sysId} value={p.sysId}>{p.name}</MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>

      {itemsLoading && <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>Loading...</Typography>}
      {!itemsLoading && displayedItems.length === 0 && (
        <Typography variant="body2" color="text.secondary" sx={{ py: 2, textAlign: "center" }}>No action items found.</Typography>
      )}

      <Stack spacing={1.5}>
        {displayedItems.map((item) => {
          const isMuted = item.status === "resolved" || item.status === "cancelled";
          const borderColor = PRIORITY_BORDER[item.priority.toLowerCase()] ?? "#9e9e9e";

          return (
            <Paper key={item.id} elevation={1} sx={{ borderLeft: `4px solid ${borderColor}`, overflow: "hidden", opacity: isMuted ? 0.65 : 1 }}>
              <Box sx={{ p: 2 }}>
                <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
                  <Chip label={getPriorityLabel(item.priority)} color={getPriorityChipColor(item.priority)} size="small" sx={{ fontWeight: 700, fontSize: "0.7rem" }} />
                  <Typography variant="body1" fontWeight="bold">{item.title}</Typography>
                  <Chip label={getProjectName(item.projectSysId)} size="small" variant="outlined" sx={{ fontSize: "0.7rem" }} />
                </Stack>

                {item.description && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>{item.description}</Typography>
                )}

                {item.resolutionComment && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5, fontStyle: "italic" }}>
                    {item.status === "cancelled" ? "Cancelled: " : "Resolution: "}
                    {item.resolutionComment}
                  </Typography>
                )}

                <Stack direction="row" alignItems="center" spacing={2} flexWrap="wrap" sx={{ mt: 1 }}>
                  {item.assignedToEmail && (
                    <Typography variant="caption" color="text.secondary">Assigned: {item.assignedToEmail}</Typography>
                  )}
                  {item.dueDate && (
                    <Box display="flex" alignItems="center" gap={0.5}>
                      <Typography variant="caption" color="text.secondary">Due:</Typography>
                      <DueDateDisplay dueDate={item.dueDate} status={item.status} />
                    </Box>
                  )}
                  <Box sx={{ ml: "auto" }}>
                    {isMuted ? (
                      <Chip label={STATUS_LABELS[item.status]} color={getStatusChipColor(item.status)} size="small" />
                    ) : (
                      <Select
                        size="small" value={item.status}
                        onChange={(e) => void handleStatusChange(item.id, e.target.value)}
                        sx={{ fontSize: "0.8rem", minWidth: 130 }}
                      >
                        <MenuItem value="open">Open</MenuItem>
                        <MenuItem value="in_progress">In Progress</MenuItem>
                        <MenuItem value="resolved">Resolved</MenuItem>
                        <MenuItem value="cancelled">Cancelled</MenuItem>
                      </Select>
                    )}
                  </Box>
                </Stack>
              </Box>

              <Box
                onClick={() => toggleExpanded(item.id)}
                sx={{ display: "flex", alignItems: "center", gap: 0.5, px: 2, py: 0.75, backgroundColor: "action.hover", borderTop: `1px solid ${theme.palette.divider}`, cursor: "pointer", userSelect: "none", "&:hover": { backgroundColor: "action.selected" } }}
              >
                <Box sx={{ color: "text.secondary", display: "inline-flex" }}>
                  <MessageSquareIcon size={14} />
                </Box>
                <Typography variant="caption" color="text.secondary">
                  Comments ({commentCounts[item.id] ?? item.commentCount ?? 0})
                </Typography>
                <Box sx={{ ml: "auto", display: "flex", alignItems: "center", color: "text.secondary" }}>
                  {expandedItems.has(item.id) ? <ChevronUpIcon size={16} /> : <ChevronDownIcon size={16} />}
                </Box>
              </Box>

              {expandedItems.has(item.id) && <InlineCommentThread itemId={item.id} onCommentPosted={handleCommentPosted} />}
            </Paper>
          );
        })}
      </Stack>

      <Dialog open={statusDialogOpen} onClose={() => setStatusDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{pendingChange?.newStatus === "resolved" ? "Resolve Action Item" : "Cancel Action Item"}</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth required multiline rows={3}
            placeholder={pendingChange?.newStatus === "resolved" ? "Add resolution comment..." : "Add cancellation reason..."}
            value={statusComment} onChange={(e) => setStatusComment(e.target.value)} sx={{ mt: 1 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setStatusDialogOpen(false); setStatusComment(""); }}>Cancel</Button>
          <Button onClick={handleStatusChangeConfirm} disabled={!statusComment.trim()}>Confirm</Button>
        </DialogActions>
      </Dialog>

      <AddActionItemDialog
        open={addOpen}
        onClose={() => { setAddOpen(false); setAddDefaultProject(undefined); }}
        accountId={accountId}
        projectsWithOpenRisks={projectsWithOpenRisks}
        onCreated={() => { setAddOpen(false); setAddDefaultProject(undefined); refresh(); }}
        defaultProjectSysId={addDefaultProject}
      />
    </Box>
  );
}
