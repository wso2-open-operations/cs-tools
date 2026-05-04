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
  alpha,
  Box,
  Button,
  ButtonGroup,
  Card,
  CardContent,
  Chip,
  Divider,
  IconButton,
  Paper,
  Stack,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ExternalLink } from "@wso2/oxygen-ui-icons-react";
import { useState, useMemo, type JSX } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { usePostUpdateLevelsSearch } from "@features/updates/api/usePostUpdateLevelsSearch";
import { getUpdateTypeChipColor } from "@features/updates/utils/updates";
import type {
  SecurityAdvisory,
  UpdateDescriptionLevel,
} from "@features/updates/types/updates";
import PendingUpdatesListSkeleton from "@features/updates/components/pending-updates/PendingUpdatesListSkeleton";
import EmptyState from "@components/empty-state/EmptyState";
import error500Svg from "@assets/error/error-500.svg";
import { ROUTE_PREVIOUS_PAGE } from "@features/project-hub/constants/navigationConstants";

type FilterType = "all" | "security" | "regular";

function IllustrativeMessage({ message }: { message: string }): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        flexDirection: "column",
        py: 5,
      }}
    >
      <img
        src={error500Svg}
        alt=""
        aria-hidden="true"
        style={{ width: 200, height: "auto" }}
      />
      <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
        {message}
      </Typography>
    </Box>
  );
}

const FILTER_BUTTONS: { key: FilterType; label: string }[] = [
  { key: "all", label: "All" },
  { key: "security", label: "Security" },
  { key: "regular", label: "Regular" },
];

/** Returns true only for http/https URLs to prevent javascript: or data: URIs. */
function isSafeUrl(url: string): boolean {
  return /^https?:\/\//i.test(url.trim());
}

/**
 * Parses a JSON-stringified array string into a plain string array.
 * Returns empty array on parse failure.
 *
 * @param {string} raw - The raw JSON string.
 * @returns {string[]} Parsed string items.
 */
function parseJsonStringArray(raw: string): string[] {
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as string[]) : [];
  } catch {
    return [];
  }
}

/**
 * Renders a labelled section with pre-wrapped text, supporting `\n` as real line breaks.
 *
 * @param {{ title: string; content: string }} props - Section heading and raw content string.
 * @returns {JSX.Element} The rendered section.
 */
function UpdateSection({
  title,
  content,
}: {
  title: string;
  content: string;
}): JSX.Element {
  const lines = content.split("\n");
  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 0.75 }}>
        {title}
      </Typography>
      <Box>
        {lines.map((line, i) => (
          <Typography
            key={i}
            variant="body2"
            color="text.secondary"
            sx={{ lineHeight: 1.7 }}
          >
            {line || "\u00A0"}
          </Typography>
        ))}
      </Box>
    </Box>
  );
}

/**
 * Card showing all details for a single update description entry.
 *
 * @param {{ desc: UpdateDescriptionLevel }} props - The update description item.
 * @returns {JSX.Element} The rendered update detail card.
 */
function UpdateDetailCard({
  desc,
}: {
  desc: UpdateDescriptionLevel;
}): JSX.Element {
  const theme = useTheme();
  const chipColor = getUpdateTypeChipColor(desc.updateType);
  const bugFixes = parseJsonStringArray(desc.bugFixes);
  const filesModified = parseJsonStringArray(desc.filesModified);
  const filesAdded = parseJsonStringArray(desc.filesAdded);
  const filesRemoved = parseJsonStringArray(desc.filesRemoved);

  return (
    <Card variant="outlined">
      <CardContent sx={{ p: 3 }}>
        {/* Header row */}
        <Box
          sx={{
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            mb: 2,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="h6" fontWeight={600}>
              Update Number: {desc.updateNumber}
            </Typography>
            <Chip
              label={
                desc.updateType.charAt(0).toUpperCase() +
                desc.updateType.slice(1)
              }
              size="small"
              sx={{
                height: 22,
                fontSize: "0.72rem",
                fontWeight: 600,
                bgcolor: alpha(theme.palette[chipColor].main, 0.15),
                color: theme.palette[chipColor].dark,
                border: `1px solid ${alpha(theme.palette[chipColor].main, 0.35)}`,
              }}
            />
          </Box>
          <Box sx={{ textAlign: "right", flexShrink: 0 }}>
            <Typography variant="caption" color="text.secondary">
              Status
            </Typography>
            <Typography variant="body2" fontWeight={500}>
              Released
            </Typography>
          </Box>
        </Box>

        <Divider sx={{ mb: 2.5 }} />

        <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
          {/* Description */}
          {desc.description && (
            <UpdateSection title="Description" content={desc.description} />
          )}

          {/* Instructions */}
          {desc.instructions && desc.instructions !== "N/A" && (
            <UpdateSection title="Instructions" content={desc.instructions} />
          )}

          {/* Bug Fixes */}
          {bugFixes.length > 0 && bugFixes.some((f) => f !== "N/A") && (
            <Box>
              <Typography
                variant="subtitle2"
                fontWeight={700}
                sx={{ mb: 0.75 }}
              >
                Bug Fixes
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                {bugFixes
                  .filter((f) => f !== "N/A")
                  .map((fix, i) =>
                    isSafeUrl(fix) ? (
                      <Box
                        key={i}
                        component="a"
                        href={fix}
                        target="_blank"
                        rel="noopener noreferrer"
                        sx={{
                          display: "inline-flex",
                          alignItems: "center",
                          gap: 0.75,
                          color: "warning.main",
                          fontSize: "0.875rem",
                          textDecoration: "none",
                          "&:hover": {
                            textDecoration: "underline",
                            color: "warning.dark",
                          },
                        }}
                      >
                        <ExternalLink size={14} aria-hidden />
                        {fix}
                      </Box>
                    ) : (
                      <Typography
                        key={i}
                        variant="body2"
                        color="text.secondary"
                      >
                        {fix}
                      </Typography>
                    ),
                  )}
              </Box>
            </Box>
          )}

          {/* Updated Files */}
          {filesModified.length > 0 && (
            <Box>
              <Typography
                variant="subtitle2"
                fontWeight={700}
                sx={{ mb: 0.75 }}
              >
                Updated Files
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
                {filesModified.map((f, i) => (
                  <Typography
                    key={i}
                    variant="body2"
                    color="text.secondary"
                    sx={{ fontFamily: "monospace", fontSize: "0.8rem" }}
                  >
                    {f}
                  </Typography>
                ))}
              </Box>
            </Box>
          )}

          {/* Added Files */}
          {filesAdded.length > 0 && (
            <Box>
              <Typography
                variant="subtitle2"
                fontWeight={700}
                sx={{ mb: 0.75 }}
              >
                Added Files
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
                {filesAdded.map((f, i) => (
                  <Typography
                    key={i}
                    variant="body2"
                    color="text.secondary"
                    sx={{ fontFamily: "monospace", fontSize: "0.8rem" }}
                  >
                    {f}
                  </Typography>
                ))}
              </Box>
            </Box>
          )}

          {/* Removed Files */}
          {filesRemoved.length > 0 && (
            <Box>
              <Typography
                variant="subtitle2"
                fontWeight={700}
                sx={{ mb: 0.75 }}
              >
                Removed Files
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
                {filesRemoved.map((f, i) => (
                  <Typography
                    key={i}
                    variant="body2"
                    color="text.secondary"
                    sx={{ fontFamily: "monospace", fontSize: "0.8rem" }}
                  >
                    {f}
                  </Typography>
                ))}
              </Box>
            </Box>
          )}

          {/* Security Advisories */}
          {desc.securityAdvisories.length > 0 && (
            <Box>
              <Typography
                variant="subtitle2"
                fontWeight={700}
                sx={{ mb: 0.75 }}
              >
                Security Advisories
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                {desc.securityAdvisories.map((advisory: SecurityAdvisory) => (
                  <Box
                    key={advisory.id}
                    sx={{
                      p: 2,
                      bgcolor: alpha(theme.palette.error.main, 0.04),
                      border: `1px solid ${alpha(theme.palette.error.main, 0.2)}`,
                      borderRadius: 1,
                    }}
                  >
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 1,
                        mb: 0.5,
                      }}
                    >
                      <Typography
                        variant="subtitle2"
                        fontWeight={700}
                        color="error.main"
                      >
                        {advisory.id}
                      </Typography>
                      <Chip
                        label={advisory.severity}
                        size="small"
                        sx={{
                          height: 18,
                          fontSize: "0.68rem",
                          bgcolor: alpha(theme.palette.error.main, 0.12),
                          color: "error.dark",
                          border: `1px solid ${alpha(theme.palette.error.main, 0.25)}`,
                        }}
                      />
                    </Box>
                    <Typography
                      variant="body2"
                      fontWeight={600}
                      sx={{ mb: 0.5 }}
                    >
                      {advisory.overview}
                    </Typography>
                    <UpdateSection
                      title="Description"
                      content={advisory.description}
                    />
                    {advisory.impact && (
                      <Box sx={{ mt: 1.5 }}>
                        <UpdateSection
                          title="Impact"
                          content={advisory.impact}
                        />
                      </Box>
                    )}
                    {advisory.solution && (
                      <Box sx={{ mt: 1.5 }}>
                        <UpdateSection
                          title="Solution"
                          content={advisory.solution}
                        />
                      </Box>
                    )}
                    {advisory.notes && (
                      <Box sx={{ mt: 1.5 }}>
                        <UpdateSection title="Notes" content={advisory.notes} />
                      </Box>
                    )}
                    {advisory.credits && advisory.credits !== "-" && (
                      <Box sx={{ mt: 1.5 }}>
                        <UpdateSection
                          title="Credits"
                          content={advisory.credits}
                        />
                      </Box>
                    )}
                  </Box>
                ))}
              </Box>
            </Box>
          )}
        </Box>
      </CardContent>
    </Card>
  );
}

/**
 * UpdateLevelDetailsPage shows the full detail for a single update level.
 * Reads levelKey from route params and product info from URL search params.
 * Re-uses cached POST /updates/levels/search result via usePostUpdateLevelsSearch.
 *
 * @returns {JSX.Element} The rendered update level detail page.
 */
export default function UpdateLevelDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const { levelKey } = useParams<{
    projectId: string;
    levelKey: string;
  }>();
  const [searchParams] = useSearchParams();
  const [filter, setFilter] = useState<FilterType>("all");

  const productName = searchParams.get("productName") ?? "";
  const productBaseVersion = searchParams.get("productBaseVersion") ?? "";
  const startParam = searchParams.get("startingUpdateLevel");
  const endParam = searchParams.get("endingUpdateLevel");
  const startingUpdateLevel = Number(startParam ?? "0");
  const endingUpdateLevel = Number(endParam ?? "0");

  const searchRequest = useMemo(() => {
    if (
      !productName ||
      !productBaseVersion ||
      startParam === null ||
      endParam === null ||
      Number.isNaN(startingUpdateLevel) ||
      Number.isNaN(endingUpdateLevel)
    ) {
      return null;
    }
    return {
      productName,
      productVersion: productBaseVersion,
      startingUpdateLevel,
      endingUpdateLevel,
    };
  }, [
    productName,
    productBaseVersion,
    startParam,
    endParam,
    startingUpdateLevel,
    endingUpdateLevel,
  ]);

  const { data, isLoading, isError } = usePostUpdateLevelsSearch(searchRequest);

  const entry = levelKey && data ? data[levelKey] : undefined;

  const filteredDescriptions = useMemo(() => {
    if (!entry) return [];
    if (filter === "all") return entry.updateDescriptionLevels;
    return entry.updateDescriptionLevels.filter((d) => d.updateType === filter);
  }, [entry, filter]);

  const handleBack = () => {
    navigate(ROUTE_PREVIOUS_PAGE);
  };

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
        overflow: "hidden",
        width: "100%",
      }}
    >
      {/* Page header — same structure as PendingUpdatesPage */}
      <Paper
        variant="outlined"
        sx={{ p: 2, flexShrink: 0, zIndex: 10, borderRadius: 0 }}
      >
        <Stack direction="row" alignItems="center" gap={2}>
          <IconButton onClick={handleBack} size="small" aria-label="Back">
            <ArrowLeft size={20} />
          </IconButton>
          <Box>
            <Typography variant="h5" color="text.primary" fontWeight={600}>
              Update Level {levelKey} Details
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {productName}
              {productBaseVersion ? ` - Version ${productBaseVersion}` : ""}
            </Typography>
          </Box>
        </Stack>
      </Paper>

      {/* Content */}
      <Box sx={{ flex: 1, minHeight: 0, overflow: "auto", p: 3, pt: 2 }}>
        {isLoading ? (
          <PendingUpdatesListSkeleton />
        ) : isError ? (
          <IllustrativeMessage message="Failed to load updates. Please try again." />
        ) : !entry ? (
          <IllustrativeMessage message="Level not found." />
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            {/* Product info card */}
            <Card variant="outlined">
              <CardContent sx={{ p: 3 }}>
                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" },
                    gap: 3,
                  }}
                >
                  <Box>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      fontWeight={500}
                    >
                      Product Name
                    </Typography>
                    <Typography
                      variant="body1"
                      fontWeight={600}
                      sx={{ mt: 0.25 }}
                    >
                      {productName || "--"}
                    </Typography>
                  </Box>
                  <Box>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      fontWeight={500}
                    >
                      Product Version
                    </Typography>
                    <Typography
                      variant="body1"
                      fontWeight={600}
                      sx={{ mt: 0.25 }}
                    >
                      {productBaseVersion || "--"}
                    </Typography>
                  </Box>
                  <Box>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      fontWeight={500}
                    >
                      Released Update Level
                    </Typography>
                    <Typography
                      variant="body1"
                      fontWeight={600}
                      sx={{ mt: 0.25 }}
                    >
                      {levelKey}
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>

            {/* Filter buttons */}
            <Box>
              <ButtonGroup
                variant="outlined"
                size="small"
                aria-label="Filter update type"
              >
                {FILTER_BUTTONS.map(({ key, label }) => (
                  <Button
                    key={key}
                    variant={filter === key ? "contained" : "outlined"}
                    color="primary"
                    onClick={() => setFilter(key)}
                    sx={{ textTransform: "none", minWidth: 80 }}
                  >
                    {label}
                  </Button>
                ))}
              </ButtonGroup>
            </Box>

            {/* Update detail cards */}
            {filteredDescriptions.length === 0 ? (
              <EmptyState
                description={`No ${filter === "all" ? "" : filter + " "}updates for this level.`}
              />
            ) : (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
                {filteredDescriptions.map((desc) => (
                  <UpdateDetailCard key={desc.updateNumber} desc={desc} />
                ))}
              </Box>
            )}
          </Box>
        )}
      </Box>
    </Box>
  );
}
