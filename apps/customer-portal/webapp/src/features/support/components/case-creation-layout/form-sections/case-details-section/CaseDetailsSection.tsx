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

import type { CaseDetailsSectionProps } from "@features/support/types/supportComponents";
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
  Typography,
} from "@wso2/oxygen-ui";
import { File, Sparkles, Upload, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  CaseSeverity,
  CaseSeverityLevel,
} from "@features/support/constants/supportConstants";
import {
  isS0SeverityLabel,
  getSeverityLegendColor,
} from "@features/dashboard/utils/dashboard";
import Editor from "@components/rich-text-editor/Editor";

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
  isSeverityDisabled = false,
}: CaseDetailsSectionProps): JSX.Element {
  const { showError } = useErrorBanner();
  const titleReadOnly = isTitleDisabled;
  const titleLength = title.trim().length;
  const isTitleTooLong = titleLength > 160;
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
  const restrictedBase = isSeverityDisabled
    ? filteredBase.filter((level) => level.label === CaseSeverity.LOW)
    : filteredBase;
  const filteredExtra = (extraSeverityLevels ?? []).filter(
    (extra) =>
      !restrictedBase.some((level) => level.id === extra.id) &&
      (!excludeS0 || !isS0SeverityLabel(extra.label)) &&
      (!isSeverityDisabled || extra.label === CaseSeverity.LOW),
  );
  const severityLevels = [...restrictedBase, ...filteredExtra].map((level) => ({
    ...level,
    color: getSeverityLegendColor(level.label),
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
              error={isTitleTooLong}
              helperText={
                isTitleTooLong
                  ? "Title must be 160 characters or fewer."
                  : undefined
              }
            />
            <Typography
              variant="caption"
              color={isTitleTooLong ? "error.main" : "text.secondary"}
              align="right"
              sx={{ mt: 0.5, display: "block" }}
            >
              {titleLength}/160
            </Typography>
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

          <Editor
            key={
              isRelatedCaseMode
                ? `related-${relatedCaseNumber ?? "new"}`
                : "default"
            }
            onAttachmentClick={isSecurityReport ? undefined : onAttachmentClick}
            attachments={isSecurityReport ? [] : attachments}
            onAttachmentRemove={
              isSecurityReport ? undefined : onAttachmentRemove
            }
            disabled={false}
            value={description}
            onChange={setDescription}
            maxHeight="210px"
            onInlineImageTypeError={() =>
              showError(
                "Only jpg, jpeg, png, and webp images can be inserted inline.",
              )
            }
          />

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
              role="button"
              tabIndex={0}
              aria-disabled={false}
              onClick={() => {
                onAttachmentClick?.();
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onAttachmentClick?.();
                }
              }}
              sx={{
                border: 1,
                borderColor: "divider",
                p: 2,
                bgcolor: "action.hover",
                cursor: "pointer",
                transition: "border-color 0.2s ease",
                "&:hover": {
                  borderColor: "warning.main",
                },
                "&:focus-visible": {
                  outline: "2px solid",
                  outlineColor: "warning.main",
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
                      color: "warning.main",
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
                    PDF, DOCX, TXT, CSV or other formats • Max 10MB per file
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
              <FormControl fullWidth size="small" disabled={isLoading}>
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
              {isSeverityDisabled ? (
                (() => {
                  const selectedLevel = severityLevels.find(
                    (level) => level.id === severity,
                  );
                  return (
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 1.5,
                        px: 1.5,
                        py: 1,
                        border: 1,
                        borderColor: "divider",
                        borderRadius: 1,
                        bgcolor: "action.hover",
                        minHeight: 40,
                      }}
                    >
                      <Box
                        sx={{
                          width: 10,
                          height: 10,
                          borderRadius: "50%",
                          bgcolor:
                            selectedLevel?.color ??
                            getSeverityLegendColor(CaseSeverityLevel.S4),
                          flexShrink: 0,
                        }}
                      />
                      <Typography variant="body2" color="text.primary">
                        {selectedLevel?.label ?? CaseSeverityLevel.S4}
                      </Typography>
                    </Box>
                  );
                })()
              ) : (
                <FormControl fullWidth size="small" disabled={isLoading}>
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
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 1.5,
                          }}
                        >
                          <Box
                            sx={{
                              width: 10,
                              height: 10,
                              borderRadius: "50%",
                              bgcolor: selectedLevel.color,
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
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 1.5,
                          }}
                        >
                          <Box
                            sx={{
                              width: 12,
                              height: 12,
                              borderRadius: "50%",
                              bgcolor: lvl.color,
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
              )}
            </Grid>
          </Grid>
        )}
      </Box>
    </Paper>
  );
}
