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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  MenuItem,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useGetCsmProjects } from "@features/csm-projects/api/useGetCsmProjects";
import { usePostCsmEngagement } from "@features/csm-engagements/api/usePostCsmEngagement";
import {
  ENGAGEMENT_DELIVERY_LABEL,
  ENGAGEMENT_PAYMENT_TYPE_LABEL,
  ENGAGEMENT_TYPES_ALL,
  ENGAGEMENT_TYPE_LABEL,
} from "@features/csm-engagements/utils/engagements";
import type {
  CreateCsmEngagementInput,
  CsmEngagementDeliveryMode,
  CsmEngagementDetail,
  CsmEngagementPaymentType,
  CsmEngagementType,
} from "@features/csm-engagements/types/csmEngagements";

interface CreateEngagementDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated: (engagement: CsmEngagementDetail) => void;
  /** Optional preselected project for in-context create. */
  presetProjectId?: string;
}

const DELIVERY_MODES: CsmEngagementDeliveryMode[] = ["remote", "onsite", "hybrid"];
const PAYMENT_TYPES: CsmEngagementPaymentType[] = ["paid", "foc"];

interface FormState {
  projectId: string;
  name: string;
  type: CsmEngagementType;
  deliveryMode: CsmEngagementDeliveryMode;
  plannedStartDate: string;
  plannedEndDate: string;
  paymentType: CsmEngagementPaymentType;
  opportunityRef: string;
  scope: string;
  description: string;
}

const today = (): string => new Date().toISOString().slice(0, 10);
const dateOffset = (days: number): string =>
  new Date(Date.now() + days * 86_400_000).toISOString().slice(0, 10);

function emptyForm(presetProjectId?: string): FormState {
  return {
    projectId: presetProjectId ?? "",
    name: "",
    type: "customer_onboarding",
    deliveryMode: "remote",
    plannedStartDate: today(),
    plannedEndDate: dateOffset(30),
    paymentType: "paid",
    opportunityRef: "",
    scope: "",
    description: "",
  };
}

export default function CreateEngagementDialog({
  open,
  onClose,
  onCreated,
  presetProjectId,
}: CreateEngagementDialogProps): JSX.Element {
  const create = usePostCsmEngagement();
  const { showError } = useErrorBanner();
  const { data: projectsData, isLoading: isProjectsLoading } =
    useGetCsmProjects("all_customers");
  const [form, setForm] = useState<FormState>(() => emptyForm(presetProjectId));
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (!open) {
      setForm(emptyForm(presetProjectId));
      setSubmitted(false);
    }
  }, [open, presetProjectId]);

  const set = <K extends keyof FormState>(k: K, v: FormState[K]): void =>
    setForm((s) => ({ ...s, [k]: v }));

  const projects = projectsData?.projects ?? [];
  const selectedProject = useMemo(
    () => projects.find((p) => p.id === form.projectId),
    [projects, form.projectId],
  );

  const missingRequired =
    !form.projectId ||
    !form.name.trim() ||
    !form.scope.trim() ||
    !form.plannedStartDate ||
    !form.plannedEndDate;

  const handleSubmit = async (): Promise<void> => {
    setSubmitted(true);
    if (missingRequired || !selectedProject) return;
    const input: CreateCsmEngagementInput = {
      name: form.name.trim(),
      type: form.type,
      projectId: selectedProject.id,
      projectName: selectedProject.name,
      accountId: selectedProject.accountId,
      customer: selectedProject.customer,
      deliveryMode: form.deliveryMode,
      plannedStartDate: form.plannedStartDate,
      plannedEndDate: form.plannedEndDate,
      scope: form.scope.trim(),
      description: form.description.trim(),
      billing: {
        paymentType: form.paymentType,
        opportunityRef: form.opportunityRef.trim() || undefined,
      },
    };
    try {
      const result = await create.mutateAsync(input);
      onCreated(result);
    } catch (e) {
      showError(e instanceof Error ? e.message : "Could not create engagement.");
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle
        sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}
      >
        <span>Create engagement</span>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={18} />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {/* Project comes first — engagements are scoped to a project the same
              way cases are. The selected project determines the customer and
              account, so those aren't asked for separately. */}
          <TextField
            size="small"
            select
            label="Project"
            required
            value={form.projectId}
            onChange={(e) => set("projectId", e.target.value)}
            error={submitted && !form.projectId}
            helperText={
              submitted && !form.projectId
                ? "Required"
                : selectedProject
                ? `${selectedProject.customer} · ${selectedProject.name}`
                : isProjectsLoading
                ? "Loading projects…"
                : "Pick the project this engagement is scoped to"
            }
            disabled={!!presetProjectId || isProjectsLoading}
          >
            {projects.map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {p.customer} — {p.name}
              </MenuItem>
            ))}
          </TextField>

          <Typography variant="caption" color="text.secondary">
            Basics
          </Typography>
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "2fr 1fr" }, gap: 2 }}>
            <TextField
              size="small"
              label="Engagement name"
              required
              value={form.name}
              onChange={(e) => set("name", e.target.value)}
              error={submitted && !form.name.trim()}
              helperText={submitted && !form.name.trim() ? "Required" : ""}
            />
            <TextField
              size="small"
              select
              label="Type"
              value={form.type}
              onChange={(e) => set("type", e.target.value as CsmEngagementType)}
            >
              {ENGAGEMENT_TYPES_ALL.map((t) => (
                <MenuItem key={t} value={t}>
                  {ENGAGEMENT_TYPE_LABEL[t]}
                </MenuItem>
              ))}
            </TextField>
          </Box>

          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "1fr 1fr 1fr" }, gap: 2 }}>
            <TextField
              size="small"
              select
              label="Delivery mode"
              value={form.deliveryMode}
              onChange={(e) => set("deliveryMode", e.target.value as CsmEngagementDeliveryMode)}
            >
              {DELIVERY_MODES.map((m) => (
                <MenuItem key={m} value={m}>
                  {ENGAGEMENT_DELIVERY_LABEL[m]}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              size="small"
              label="Planned start"
              type="date"
              required
              value={form.plannedStartDate}
              onChange={(e) => set("plannedStartDate", e.target.value)}
              InputLabelProps={{ shrink: true }}
            />
            <TextField
              size="small"
              label="Planned end"
              type="date"
              required
              value={form.plannedEndDate}
              onChange={(e) => set("plannedEndDate", e.target.value)}
              InputLabelProps={{ shrink: true }}
            />
          </Box>

          <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
            Commercial
          </Typography>
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "1fr 2fr" }, gap: 2 }}>
            <TextField
              size="small"
              select
              label="Payment type"
              value={form.paymentType}
              onChange={(e) => set("paymentType", e.target.value as CsmEngagementPaymentType)}
            >
              {PAYMENT_TYPES.map((p) => (
                <MenuItem key={p} value={p}>
                  {ENGAGEMENT_PAYMENT_TYPE_LABEL[p]}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              size="small"
              label="Salesforce opportunity ref (optional)"
              placeholder="e.g. 00k4400000eKZRFAA4"
              value={form.opportunityRef}
              onChange={(e) => set("opportunityRef", e.target.value)}
              helperText="Only set if this engagement originates from a Salesforce opportunity."
            />
          </Box>

          <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
            Scope &amp; description
          </Typography>
          <TextField
            size="small"
            label="Scope statement"
            required
            multiline
            minRows={3}
            value={form.scope}
            onChange={(e) => set("scope", e.target.value)}
            placeholder="What this engagement does and doesn't include."
            error={submitted && !form.scope.trim()}
            helperText={submitted && !form.scope.trim() ? "Required" : ""}
          />
          <TextField
            size="small"
            label="Description / background"
            multiline
            minRows={3}
            value={form.description}
            onChange={(e) => set("description", e.target.value)}
          />
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, py: 2 }}>
        <Button variant="text" onClick={onClose} disabled={create.isPending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => void handleSubmit()}
          disabled={create.isPending}
        >
          {create.isPending ? "Creating…" : "Create engagement"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
