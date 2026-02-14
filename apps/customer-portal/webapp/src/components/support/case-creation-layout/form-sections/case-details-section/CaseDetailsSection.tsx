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
  Typography,
} from "@wso2/oxygen-ui";
import { PencilLine, Sparkles } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { CaseMetadataResponse } from "@models/responses";
import { getSeverityColor } from "@utils/casesTable";

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
}: CaseDetailsSectionProps): JSX.Element {
  const [isEditing, setIsEditing] = useState(false);

  const meta = metadata as { issueTypes?: unknown[]; severityLevels?: { id: string; label: string; description?: string }[] } | undefined;
  const baseIssueTypes = filters?.issueTypes ?? meta?.issueTypes ?? [];
  const issueTypes = [
    ...baseIssueTypes,
    ...(extraIssueTypes ?? []).filter(
      (extra) =>
        !baseIssueTypes.some((base: unknown) => {
          const baseLabel =
            typeof base === "string" ? base : (base as { label?: string }).label;
          return baseLabel === extra;
        }),
    ),
  ];
  const baseSeverityLevels = (filters?.severities ?? meta?.severityLevels ?? []) as {
    id: string;
    label: string;
    description?: string;
  }[];
  const severityLevels = [
    ...baseSeverityLevels,
    ...(extraSeverityLevels ?? []).filter(
      (extra) => !baseSeverityLevels.some((level) => level.id === extra.id),
    ),
  ];

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
        <Typography variant="h6">Case Details</Typography>
        <IconButton
          onClick={() => setIsEditing(true)}
          aria-label="Edit case details"
        >
          <PencilLine size={18} />
        </IconButton>
      </Box>

      {/* main form fields container */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        {/* issue title field wrapper */}
        <Box>
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">
              Title{" "}
              <Box component="span" sx={{ color: "warning.main" }}>
                *
              </Box>
            </Typography>
            <Chip
              label="Generated from chat"
              size="small"
              variant="outlined"
              icon={<Sparkles size={10} />}
              sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
            />
          </Box>
          <Box>
            <TextField
              id="title"
              fullWidth
              multiline
              minRows={1}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Enter issue title"
              disabled={isLoading || !isEditing}
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
            <Chip
              label="From conversation"
              size="small"
              variant="outlined"
              icon={<Sparkles size={10} />}
              sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
            />
          </Box>
          <Box>
            <TextField
              fullWidth
              multiline
              minRows={6}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe the issue in detail"
              disabled={isLoading || !isEditing}
              data-testid="case-description-editor"
            />
          </Box>
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
        </Box>

        {/* issue type and severity grid container */}
        <Grid container spacing={3}>
          <Grid size={{ xs: 12 }}>
            <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
              <Typography variant="caption">
                Issue Type{" "}
                <Box component="span" sx={{ color: "warning.main" }}>
                  *
                </Box>
              </Typography>
              <Chip
                label="AI classified"
                size="small"
                variant="outlined"
                icon={<Sparkles size={10} />}
                sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
              />
            </Box>
            <FormControl fullWidth size="small" disabled={isLoading || !isEditing}>
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
                      typeof type === "string" ? type : (type as { label?: string }).label;
                    return label != null && String(label).trim() !== "";
                  })
                  .map((type: unknown) => {
                    const label =
                      typeof type === "string" ? type : (type as { label?: string }).label as string;
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
            <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
              <Typography variant="caption">
                Severity Level{" "}
                <Box component="span" sx={{ color: "warning.main" }}>
                  *
                </Box>
              </Typography>
              <Chip
                label="AI assessed"
                size="small"
                variant="outlined"
                icon={<Sparkles size={10} />}
                sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
              />
            </Box>
            <FormControl fullWidth size="small" disabled={isLoading || !isEditing}>
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
      </Box>
    </Paper>
  );
}
