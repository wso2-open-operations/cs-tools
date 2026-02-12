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
  Form,
  FormControl,
  Grid,
  IconButton,
  MenuItem,
  Paper,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { RichTextEditor } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/RichTextEditor";
import { PencilLine, Sparkles } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { CaseMetadataResponse } from "@models/responses";
import { getSeverityColor } from "@utils/casesTable";

interface CaseDetailsSectionProps {
  title: string;
  setTitle: (value: string) => void;
  description: string;
  setDescription: (value: string) => void;
  issueType: string;
  setIssueType: (value: string) => void;
  severity: string;
  setSeverity: (value: string) => void;
  metadata: any;
  filters?: CaseMetadataResponse;
  isLoading: boolean;
  storageKey?: string;
  extraIssueTypes?: string[];
  extraSeverityLevels?: { id: string; label: string; description?: string }[];
}

/**
 * Renders the Case Details section for case creation.
 *
 * @returns The Case Details section JSX element.
 */
export const CaseDetailsSection = ({
  title,
  setTitle,
  description,
  setDescription,
  issueType,
  setIssueType,
  severity,
  setSeverity,
  metadata,
  filters,
  isLoading,
  storageKey,
  extraIssueTypes,
  extraSeverityLevels,
}: CaseDetailsSectionProps): JSX.Element => {
  const [isEditing, setIsEditing] = useState(false);

  // TODO : Remove this after the mock interface removed.
  const baseIssueTypes = filters?.issueTypes || metadata?.issueTypes || [];
  const issueTypes = [
    ...baseIssueTypes,
    ...(extraIssueTypes ?? []).filter(
      (extra) =>
        !baseIssueTypes.some((base: any) => {
          const baseLabel = typeof base === "string" ? base : base.label;
          return baseLabel === extra;
        }),
    ),
  ];
  const baseSeverityLevels = (filters?.severities ||
    metadata?.severityLevels ||
    []) as {
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
        <IconButton>
          <PencilLine
            aria-label="Edit case details"
            size={18}
            onClick={() => setIsEditing(true)}
          />
        </IconButton>
      </Box>

      {/* main form fields container */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        {/* issue title field wrapper */}
        <Box>
          {/* issue title field label container */}
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
          <Form.ElementWrapper label="" name="title">
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
          </Form.ElementWrapper>
        </Box>

        {/* case description field wrapper */}
        <Box>
          {/* case description field label container */}
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
          <Form.ElementWrapper label="" name="description">
            <RichTextEditor
              value={description}
              onChange={setDescription}
              placeholder="Describe the issue in detail"
              disabled={isLoading || !isEditing}
              minHeight={200}
              storageKey={storageKey}
              data-testid="case-description-editor"
            />
          </Form.ElementWrapper>
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
          {/* issue type selection field wrapper */}
          <Grid size={{ xs: 12 }}>
            {/* issue type field label container */}
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
            <FormControl
              fullWidth
              size="small"
              disabled={isLoading || !isEditing}
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
                {issueTypes.map((type: any) => {
                  const label = typeof type === "string" ? type : type.label;
                  return (
                    <MenuItem key={label} value={label}>
                      {label}
                    </MenuItem>
                  );
                })}
              </Select>
            </FormControl>
          </Grid>
          {/* severity level selection field wrapper */}
          <Grid size={{ xs: 12 }}>
            {/* severity level field label container */}
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
            <FormControl
              fullWidth
              size="small"
              disabled={isLoading || !isEditing}
            >
              <Select
                id="severity-level-select"
                value={severity}
                onChange={(e) => setSeverity(e.target.value as string)}
                displayEmpty
                renderValue={(value) => {
                  if (value === "") {
                    return "Select Severity Level...";
                  }
                  const selectedLevel = severityLevels.find(
                    (level: any) => level.id === value,
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
                {severityLevels.map((lvl: any) => (
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
};
