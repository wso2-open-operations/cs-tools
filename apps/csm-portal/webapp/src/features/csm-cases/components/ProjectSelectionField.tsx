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
  Chip,
  Divider,
  Modal,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircle, Lock } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import AsyncProjectSelect from "@features/csm-cases/components/AsyncProjectSelect";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import type { BeProject } from "@api/backend/types";

export interface ProjectSelectionFieldProps {
  /** Selected project id ("" when none picked yet). */
  value: string;
  onChange: (next: string) => void;
  /**
   * Present when the project arrived locked-in (`?projectId=` or nav state
   * from a project/case page) — renders the existing read-only locked field
   * instead of the picker/confirmation below.
   */
  lockedProjectId?: string;
  required?: boolean;
  /**
   * Restricts the searchable picker to projects matching this predicate
   * (see {@link AsyncProjectSelect}'s `filterProject`). Not applied to the
   * locked-project display — a project arriving locked is trusted as-is.
   */
  filterProject?: (project: BeProject) => boolean;
}

/**
 * Project picker for the Case/Security Report/Service Request create forms.
 * Creating from a project's own page arrives locked (handled unchanged, as
 * before); creating from a cross-project list page (Cases/Operations/Security
 * Center) shows the searchable {@link AsyncProjectSelect}. Picking a project
 * there doesn't commit it immediately — a confirmation dialog surfaces the
 * account behind it first, since the account is the detail most likely to
 * reveal a wrong-project pick (two projects can share a similar name across
 * different customers). Only on explicit confirmation does the field settle
 * into a persistent, unmissable summary (project + account) with a "Change"
 * action, replacing what would otherwise be one line of small Autocomplete
 * text easy to misread or overlook for the rest of the form.
 */
export default function ProjectSelectionField({
  value,
  onChange,
  lockedProjectId,
  required,
  filterProject,
}: ProjectSelectionFieldProps): JSX.Element {
  const isLocked = !!lockedProjectId;
  // Picked from the Autocomplete but not yet confirmed in the dialog below —
  // kept separate from `value` so the parent's state (and so the rest of the
  // form it gates) only ever reflects a confirmed project.
  const [pendingProjectId, setPendingProjectId] = useState<string | null>(null);

  const selectedProject = useGetProject(
    isLocked ? lockedProjectId : value || undefined,
  );
  const pendingProject = useGetProject(pendingProjectId ?? undefined);

  if (isLocked) {
    const label = selectedProject.data?.name
      ? selectedProject.data.name
      : selectedProject.isLoading
        ? "Loading project…"
        : lockedProjectId;
    return (
      <TextField
        fullWidth
        size="small"
        label="Project"
        required={required}
        value={label}
        slotProps={{
          input: {
            readOnly: true,
            endAdornment: <Lock size={16} aria-hidden style={{ opacity: 0.6 }} />,
          },
          htmlInput: { "aria-readonly": true },
        }}
        helperText="Locked to the project you opened this from. To file against another project, open that project first."
      />
    );
  }

  const handleCancel = (): void => setPendingProjectId(null);
  const handleConfirm = (): void => {
    if (pendingProjectId) onChange(pendingProjectId);
    setPendingProjectId(null);
  };

  if (!value) {
    return (
      <>
        {/* Never wired to `value` directly — a pick here only stages a
            candidate for the confirmation dialog. The Autocomplete stays
            controlled by the still-empty `value`, so it visibly resets to
            unselected the instant a pick is made, rather than showing a
            selection that might still be cancelled. */}
        <AsyncProjectSelect
          value=""
          onChange={(next) => {
            if (next) setPendingProjectId(next);
          }}
          required={required}
          filterProject={filterProject}
        />
        {/*
          A plain `Modal` + `Paper` rather than `Dialog` — `Dialog`'s paper is
          styled by the theme with a more opaque background + heavier blur for
          modals, but this confirmation reads better with the lighter,
          translucent "acrylic" look (oxygen-ui's `MuiPaper.styleOverrides.root`
          gives that for free), matching the global search dropdown
          (QuickNav.tsx), which uses the same Modal+Paper approach for the
          identical reason.
        */}
        <Modal
          open={!!pendingProjectId}
          onClose={handleCancel}
          slotProps={{ backdrop: { sx: { backgroundColor: "transparent" } } }}
        >
          <Paper
            elevation={3}
            sx={{
              position: "fixed",
              top: "20vh",
              left: "50%",
              transform: "translateX(-50%)",
              width: { xs: "calc(100% - 32px)", sm: 420 },
              maxWidth: 420,
              p: 3,
              outline: "none",
              border: "1px solid",
              borderColor: "divider",
            }}
          >
            <Typography variant="h6" sx={{ mb: 1.5 }}>
              Confirm project
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              You&apos;re about to create this against:
            </Typography>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                {pendingProject.data?.name ??
                  (pendingProject.isLoading ? "Loading…" : pendingProjectId)}
              </Typography>
              {pendingProject.data?.key && (
                <Typography variant="caption" color="text.secondary">
                  Key: {pendingProject.data.key}
                </Typography>
              )}
              {pendingProject.data?.account && (
                <>
                  <Divider sx={{ my: 1 }} />
                  <Typography variant="caption" color="text.secondary">
                    Account
                  </Typography>
                  <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                    {pendingProject.data.account.name}
                  </Typography>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                    {pendingProject.data.account.tier && (
                      <Chip
                        size="small"
                        variant="outlined"
                        label={pendingProject.data.account.tier}
                      />
                    )}
                    {pendingProject.data.account.region && (
                      <Typography variant="caption" color="text.secondary">
                        {pendingProject.data.account.region}
                      </Typography>
                    )}
                  </Box>
                </>
              )}
            </Box>
            <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 3 }}>
              <Button variant="outlined" color="inherit" onClick={handleCancel}>
                No
              </Button>
              <Button
                variant="contained"
                onClick={handleConfirm}
                disabled={pendingProject.isLoading}
              >
                Yes
              </Button>
            </Box>
          </Paper>
        </Modal>
      </>
    );
  }

  const projectLabel = selectedProject.data?.name
    ? selectedProject.data.name
    : selectedProject.isLoading
      ? "Loading project…"
      : value;

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1,
        height: 40,
        px: 1.5,
        borderRadius: 1,
        border: "1px solid",
        borderColor: "success.main",
        bgcolor: "success.50",
      }}
    >
      <Box sx={{ display: "flex", color: "success.main", flexShrink: 0 }}>
        <CheckCircle size={16} aria-hidden />
      </Box>
      <Typography
        variant="body2"
        noWrap
        sx={{ fontWeight: 600, lineHeight: 1.3, minWidth: 0, flex: 1 }}
      >
        {projectLabel}
      </Typography>
      <Button
        size="small"
        variant="text"
        onClick={() => onChange("")}
        sx={{ minWidth: 0, px: 1, flexShrink: 0 }}
      >
        Change
      </Button>
    </Box>
  );
}
