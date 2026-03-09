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
  Chip,
  FormControl,
  Grid,
  IconButton,
  MenuItem,
  Paper,
  Select,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  File,
  PencilLine,
  Sparkles,
  Upload,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { CaseSeverity, CaseSeverityLevel } from "@constants/supportConstants";
import { isS0SeverityLabel } from "@constants/dashboardConstants";
import type { CaseMetadataResponse } from "@models/responses";
import { getSeverityColor } from "@utils/support";
import Editor from "@components/common/rich-text-editor/Editor";

export interface CaseDetailsSectionProps {
  title?: string;
  setTitle?: (value: string) => void;
  description?: string;
  setDescription?: (value: string) => void;
  issueType?: string;
  setIssueType?: (value: string) => void;
  severity?: string;
  setSeverity?: (value: string) => void;
  metadata?: unknown;
  filters?: CaseMetadataResponse;
  isLoading?: boolean;
  storageKey?: string;
  extraIssueTypes?: string[];
  extraSeverityLevels?: { id: string; label: string; description?: string }[];
  attachments?: File[];
  onAttachmentClick?: () => void;
  onAttachmentRemove?: (index: number) => void;
  isRelatedCaseMode?: boolean;
  isTitleDisabled?: boolean;
  relatedCaseNumber?: string;
  isSecurityReport?: boolean;
  excludeS0?: boolean;
}

/**
 * Renders the Case Details section for case creation.
 *
 * @returns {JSX.Element} The Case Details section.
 */
export function CaseDetailsSection({
  title = "",
  setTitle = () => undefined,
  description = "",
  setDescription = () => undefined,
  issueType = "",
  setIssueType = () => undefined,
  severity = "",
  setSeverity = () => undefined,
  metadata,
  filters,
  isLoading = false,
  extraIssueTypes,
  extraSeverityLevels,
  attachments = [],
  onAttachmentClick,
  onAttachmentRemove,
  isRelatedCaseMode = false,
  isTitleDisabled = false,
  relatedCaseNumber,
  isSecurityReport = false,
  excludeS0 = false,
}: CaseDetailsSectionProps): JSX.Element {
  const [isEditing, setIsEditing] = useState(false);
  const effectiveEditing = isRelatedCaseMode || isEditing;
  const titleReadOnly = isTitleDisabled || !effectiveEditing;
  const meta = metadata as
    | {
        issueTypes?: unknown[];
        severityLevels?: { id: string; label: string; description?: string }[];
      }
    | undefined;
  const baseIssueTypes = filters?.issueTypes ?? meta?.issueTypes ?? [];
  const issueTypes = [
    ...baseIssueTypes,
    ...(extraIssueTypes ?? []).filter(
      (extra) =>
        !baseIssueTypes.some((base: unknown) => {
          const baseLabel =
            typeof base === "string"
              ? base
              : (base as { label?: string }).label;
          return baseLabel === extra;
        }),
    ),
  ];
  const baseSeverityLevels = (filters?.severities ??
    meta?.severityLevels ??
    []) as {
    id: string;
    label: string;
    description?: string;
  }[];

  const SEVERITY_LABEL_MAP: Record<string, string> = {
    [CaseSeverity.LOW]: CaseSeverityLevel.S4,
    [CaseSeverity.MEDIUM]: CaseSeverityLevel.S3,
    [CaseSeverity.HIGH]: CaseSeverityLevel.S2,
    [CaseSeverity.CRITICAL]: CaseSeverityLevel.S1,
    [CaseSeverity.CATASTROPHIC]: CaseSeverityLevel.S0,
  };

  const filteredBase = excludeS0
    ? baseSeverityLevels.filter((level) => !isS0SeverityLabel(level.label))
    : baseSeverityLevels;
  const filteredExtra = (extraSeverityLevels ?? []).filter(
    (extra) =>
      !filteredBase.some((level) => level.id === extra.id) &&
      (!excludeS0 || !isS0SeverityLabel(extra.label)),
  );
  const severityLevels = [
    ...filteredBase,
    ...filteredExtra,
  ].map((level) => ({
    ...level,
    label: SEVERITY_LABEL_MAP[level.label] ?? level.label,
  }));

  return (
    <Paper sx={{ p: 3 }}>
      {/* section header container */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 3,
        }}
      >
        <Typography variant="h6">
          {isSecurityReport ? "Report Details" : "Case Details"}
        </Typography>
        {!isRelatedCaseMode && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Tooltip
              title={
                isEditing
                  ? "Click here to stop modifying case details"
                  : "Click here to modify case details"
              }
              placement="top"
              arrow
            >
              <IconButton
                onClick={() => setIsEditing(!isEditing)}
                aria-label={
                  isEditing ? "Stop editing case details" : "Edit case details"
                }
                color={isEditing ? "primary" : "default"}
              >
                <PencilLine size={18} />
              </IconButton>
            </Tooltip>
          </Box>
        )}
      </Box>

      {/* main form fields container */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        {/* Related case (parent case number) - read-only when creating from parent */}
        {relatedCaseNumber != null && relatedCaseNumber !== "" && (
          <Box>
            <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
              <Typography variant="caption">Related case</Typography>
            </Box>
            <TextField
              fullWidth
              size="small"
              value={relatedCaseNumber}
              disabled
            />
          </Box>
        )}

        {/* issue title field wrapper */}
        <Box>
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">
              Title{" "}
              {!isTitleDisabled && (
                <Box component="span" sx={{ color: "warning.main" }}>
                  *
                </Box>
              )}
            </Typography>
            {!isRelatedCaseMode && (
              <Chip
                label="Generated from chat"
                size="small"
                variant="outlined"
                icon={<Sparkles size={10} />}
                sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
              />
            )}
          </Box>
          <Box>
            <TextField
              id="title"
              fullWidth
              multiline
              minRows={1}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={
                isRelatedCaseMode
                  ? "Enter related case title"
                  : "Enter issue title"
              }
              disabled={isLoading || titleReadOnly}
            />
          </Box>
        </Box>

        {/* case description field wrapper */}
        <Box>
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">
              Description{" "}
              <Box component="span" sx={{ color: "warning.main" }}>
                *
              </Box>
            </Typography>
            {!isRelatedCaseMode && (
              <Chip
                label="From conversation"
                size="small"
                variant="outlined"
                icon={<Sparkles size={10} />}
                sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
              />
            )}
          </Box>
          <Box>
            <Editor
              key={
                isRelatedCaseMode
                  ? `related-${relatedCaseNumber ?? "new"}`
                  : "default"
              }
              onAttachmentClick={
                isSecurityReport ? undefined : onAttachmentClick
              }
              attachments={isSecurityReport ? [] : attachments}
              onAttachmentRemove={
                isSecurityReport ? undefined : onAttachmentRemove
              }
              disabled={!effectiveEditing}
              value={description}
              onChange={setDescription}
              maxHeight="210px"
            />
          </Box>
          {!isRelatedCaseMode && (
            <Typography
              variant="caption"
              color="text.disabled"
              align="right"
              sx={{
                mt: 1,
                display: "block",
                fontStyle: "italic",
                fontSize: "0.7rem",
              }}
            >
              This includes all the information you shared with Novera
            </Typography>
          )}
        </Box>

        {isSecurityReport && (
          <Box>
            <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
              <Typography variant="caption">
                Attach Security Report{" "}
                <Box component="span" sx={{ color: "warning.main" }}>
                  *
                </Box>
              </Typography>
            </Box>

            <Paper
              role={effectiveEditing ? "button" : undefined}
              tabIndex={0}
              aria-disabled={!effectiveEditing}
              onClick={() => {
                if (!effectiveEditing) return;
                onAttachmentClick?.();
              }}
              onKeyDown={(e) => {
                if (!effectiveEditing) return;
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onAttachmentClick?.();
                }
              }}
              sx={{
                border: 1,
                borderColor: effectiveEditing ? "divider" : "action.disabled",
                p: 2,
                bgcolor: effectiveEditing
                  ? "action.hover"
                  : "action.disabledBackground",
                cursor: effectiveEditing ? "pointer" : "not-allowed",
                transition: "border-color 0.2s ease",
                "&:hover": {
                  borderColor: effectiveEditing
                    ? "warning.main"
                    : "action.disabled",
                },
                "&:focus-visible": {
                  outline: "2px solid",
                  outlineColor: effectiveEditing
                    ? "warning.main"
                    : "action.disabled",
                },
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                <Box
                  sx={{
                    p: 1,
                    bgcolor: "background.paper",
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 1,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                >
                  <Upload size={18} />
                </Box>

                <Box sx={{ minWidth: 0, flex: 1 }}>
                  <Typography
                    sx={{
                      color: effectiveEditing
                        ? "warning.main"
                        : "text.disabled",
                      fontSize: "0.875rem",
                      fontWeight: 500,
                    }}
                  >
                    Upload files
                  </Typography>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ mt: 1, display: "block" }}
                  >
                    PDF, DOCX, TXT, CSV or other formats • Max 15MB per file
                  </Typography>
                </Box>
              </Box>
            </Paper>

            {attachments.length > 0 && (
              <Box
                sx={{
                  mt: 1.5,
                  display: "flex",
                  flexDirection: "column",
                  gap: 1,
                }}
              >
                {attachments.map((file, index) => (
                  <Box
                    key={`${file.name}-${file.size}-${index}`}
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      px: 1.5,
                      py: 1,
                      border: 1,
                      borderColor: "divider",
                      borderRadius: 1,
                      bgcolor: "background.paper",
                    }}
                  >
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      <File size={16} />
                      <Typography variant="body2" sx={{ minWidth: 0 }} noWrap>
                        {file.name}
                      </Typography>
                    </Box>
                    <IconButton
                      aria-label={`remove security report file ${file.name}`}
                      size="small"
                      onClick={() => onAttachmentRemove?.(index)}
                    >
                      <X size={14} />
                    </IconButton>
                  </Box>
                ))}
              </Box>
            )}
          </Box>
        )}

        {/* issue type and severity grid container */}
        {!isSecurityReport && (
          <Grid container spacing={3}>
            <Grid size={{ xs: 12 }}>
              <Box
                sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}
              >
                <Typography variant="caption">
                  Issue Type{" "}
                  <Box component="span" sx={{ color: "warning.main" }}>
                    *
                  </Box>
                </Typography>
                {!isRelatedCaseMode && (
                  <Chip
                    label="AI classified"
                    size="small"
                    variant="outlined"
                    icon={<Sparkles size={10} />}
                    sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
                  />
                )}
              </Box>
              <FormControl
                fullWidth
                size="small"
                disabled={isLoading || !effectiveEditing}
              >
                <Select
                  id="issue-type-select"
                  value={issueType}
                  onChange={(e) => setIssueType(e.target.value)}
                  displayEmpty
                  renderValue={(value) =>
                    value === "" ? "Select Issue Type..." : value
                  }
                >
                  <MenuItem value="" disabled>
                    Select Issue Type...
                  </MenuItem>
                  {issueTypes
                    .filter((type: unknown) => {
                      const label =
                        typeof type === "string"
                          ? type
                          : (type as { label?: string }).label;
                      return label != null && String(label).trim() !== "";
                    })
                    .map((type: unknown) => {
                      const label =
                        typeof type === "string"
                          ? type
                          : ((type as { label?: string }).label as string);
                      return (
                        <MenuItem key={label} value={label}>
                          {label}
                        </MenuItem>
                      );
                    })}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12 }}>
              <Box
                sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}
              >
                <Typography variant="caption">
                  Severity Level{" "}
                  <Box component="span" sx={{ color: "warning.main" }}>
                    *
                  </Box>
                </Typography>
                {!isRelatedCaseMode && (
                  <Chip
                    label="AI assessed"
                    size="small"
                    variant="outlined"
                    icon={<Sparkles size={10} />}
                    sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
                  />
                )}
              </Box>
              <FormControl
                fullWidth
                size="small"
                disabled={isLoading || !effectiveEditing}
              >
                <Select
                  id="severity-level-select"
                  value={severity}
                  onChange={(e) => setSeverity(e.target.value)}
                  displayEmpty
                  renderValue={(value) => {
                    if (value === "") {
                      return "Select Severity Level...";
                    }
                    const selectedLevel = severityLevels.find(
                      (level) => level.id === value,
                    );
                    if (!selectedLevel) {
                      return value as string;
                    }
                    return (
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 1.5 }}
                      >
                        <Box
                          sx={{
                            width: 10,
                            height: 10,
                            borderRadius: "50%",
                            bgcolor: getSeverityColor(selectedLevel.label),
                            flexShrink: 0,
                          }}
                        />
                        <Typography variant="body2">
                          {selectedLevel.label}
                        </Typography>
                      </Box>
                    );
                  }}
                >
                  <MenuItem value="" disabled>
                    Select Severity Level...
                  </MenuItem>
                  {severityLevels.map((lvl) => (
                    <MenuItem key={lvl.id} value={lvl.id}>
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 1.5 }}
                      >
                        <Box
                          sx={{
                            width: 12,
                            height: 12,
                            borderRadius: "50%",
                            bgcolor: getSeverityColor(lvl.label),
                          }}
                        />
                        <Box
                          sx={{
                            display: "flex",
                            flexDirection: "column",
                            minWidth: 0,
                          }}
                        >
                          <Typography variant="body2">{lvl.label}</Typography>
                          {lvl.description && (
                            <Typography
                              variant="caption"
                              color="text.secondary"
                              sx={{
                                display: "block",
                                lineHeight: 1.2,
                                whiteSpace: "nowrap",
                                overflow: "hidden",
                                textOverflow: "ellipsis",
                              }}
                            >
                              {lvl.description}
                            </Typography>
                          )}
                        </Box>
                      </Box>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          </Grid>
        )}
      </Box>
    </Paper>
  );
}
