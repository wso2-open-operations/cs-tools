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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Typography,
} from "@wso2/oxygen-ui";
import { Printer, X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useState, type JSX } from "react";
import {
  generateUpdateLevelsReportPdf,
  isSafeHttpUrl,
  parseBugFixes,
  parseDescriptionSections,
} from "@features/updates/utils/updateLevelsReportPdf";
import type { UpdateLevelsReportModalProps, UpdateDescriptionLevel } from "@features/updates/types/updates";

function isInstructionsNonEmpty(text: string | null | undefined): boolean {
  if (!text) return false;
  const t = text.trim().toLowerCase();
  return t !== "" && t !== "n/a" && t !== "na";
}

function formatTimestamp(ts: number): string {
  const d = new Date(ts);
  const months = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
  return `${months[d.getUTCMonth()]} ${d.getUTCDate()}, ${d.getUTCFullYear()}`;
}

function UpdateDescriptionItem({ item }: { item: UpdateDescriptionLevel }): JSX.Element {
  const parsed = parseDescriptionSections(item.description);
  const displayDesc = parsed.generalDescription || parsed.implementationDetails || parsed.impact;
  const bugFixUrls = parseBugFixes(item.bugFixes);
  return (
    <Box sx={{ pl: 2, py: 0.5, borderLeft: "2px solid", borderColor: "divider" }}>
      {displayDesc && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 0.5 }}>
          {displayDesc}
        </Typography>
      )}
      {bugFixUrls.length > 0 && (
        <Typography variant="caption" color="text.secondary">
          Bug fixes:{" "}
          {bugFixUrls.map((url, i) => (
            <span key={i}>
              {i > 0 && ", "}
              {isSafeHttpUrl(url) ? (
                <a href={url} target="_blank" rel="noreferrer" style={{ color: "#1976d2", textDecoration: "underline" }}>{url}</a>
              ) : (
                url
              )}
            </span>
          ))}
        </Typography>
      )}
    </Box>
  );
}

/**
 * Modal that displays the Update Levels Report split into Security and Regular sections.
 * Each level is a clickable link that navigates to its detail page.
 */
export default function UpdateLevelsReportModal({
  open,
  reportData,
  onClose,
  onView,
  rawData,
}: UpdateLevelsReportModalProps): JSX.Element | null {
  const [isDownloading, setIsDownloading] = useState(false);

  const handleDownloadPdf = useCallback(() => {
    if (!reportData) return;
    setIsDownloading(true);
    setTimeout(() => {
      try {
        generateUpdateLevelsReportPdf(reportData);
      } finally {
        setIsDownloading(false);
      }
    }, 0);
  }, [reportData]);

  const handleLevelClick = useCallback(
    (levelKey: string) => {
      onClose();
      onView?.(levelKey);
    },
    [onClose, onView],
  );

  if (!reportData) {
    return null;
  }

  const entries = rawData ? Object.entries(rawData).sort(([a], [b]) => Number(a) - Number(b)) : [];
  const securityEntries = entries.filter(([, e]) => e.updateType === "security");
  const regularEntries = entries.filter(([, e]) => e.updateType !== "security");

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        Update Levels Report
        <IconButton
          aria-label="Close"
          onClick={onClose}
          size="small"
          className="print-hide"
          sx={{ "@media print": { display: "none !important" } }}
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>
      <DialogContent>
        <Box id="update-levels-report-content">
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Generated on: {reportData.generatedStr}
          </Typography>

          <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 1 }}>
            Search Criteria
          </Typography>
          <Box sx={{ mb: 2, "& p": { m: 0, fontSize: "0.875rem" } }}>
            <Typography variant="body2" component="p">Product: {reportData.productName}</Typography>
            <Typography variant="body2" component="p">Version: {reportData.productVersion}</Typography>
            <Typography variant="body2" component="p">
              Update Level Range: {reportData.startLevel} to {reportData.endLevel}
            </Typography>
            <Typography variant="body2" component="p">Total Updates: {reportData.totalUpdates}</Typography>
            <Typography variant="body2" component="p">
              Update Levels: {reportData.levelCount} levels ({reportData.levelsRange})
            </Typography>
          </Box>

          <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 1 }}>
            Update Levels Summary
          </Typography>
          <Box sx={{ mb: 3, "& p": { m: 0, fontSize: "0.875rem" } }}>
            <Typography variant="body2" component="p">Security Updates Levels: {reportData.securityCount}</Typography>
            <Typography variant="body2" component="p">Regular Updates Levels: {reportData.regularCount}</Typography>
            <Typography variant="body2" component="p">Mixed Updates Levels: {reportData.mixedCount}</Typography>
          </Box>

          <Divider sx={{ mb: 3 }} />

          {/* Security Updates Section */}
          <Typography variant="subtitle2" fontWeight={700} color="error.main" sx={{ mb: 1.5 }}>
            Security Updates ({securityEntries.length} {securityEntries.length === 1 ? "level" : "levels"})
          </Typography>
          {securityEntries.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ mb: 3, pl: 1 }}>
              No security updates in this range.
            </Typography>
          ) : (
            <Box sx={{ mb: 3, display: "flex", flexDirection: "column", gap: 2 }}>
              {securityEntries.map(([levelKey, entry]) => {
                const firstDesc = entry.updateDescriptionLevels[0];
                const date = firstDesc?.timestamp ? formatTimestamp(firstDesc.timestamp) : "N/A";
                return (
                  <Box key={levelKey}>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}>
                      <Button
                        variant="text"
                        size="small"
                        color="error"
                        onClick={() => handleLevelClick(levelKey)}
                        sx={{ textTransform: "none", fontWeight: 600, p: 0, minWidth: 0 }}
                      >
                        Level {levelKey}
                      </Button>
                      <Typography variant="caption" color="text.secondary">
                        — {date}
                      </Typography>
                    </Box>
                    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
                      {entry.updateDescriptionLevels.map((item, i) => (
                        <UpdateDescriptionItem key={i} item={item} />
                      ))}
                    </Box>
                  </Box>
                );
              })}
            </Box>
          )}

          <Divider sx={{ mb: 3 }} />

          {/* Regular Updates Section */}
          <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 1.5 }}>
            Regular Updates ({regularEntries.length} {regularEntries.length === 1 ? "level" : "levels"})
          </Typography>
          {regularEntries.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2, pl: 1 }}>
              No regular updates in this range.
            </Typography>
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              {regularEntries.map(([levelKey, entry]) => {
                const firstDesc = entry.updateDescriptionLevels[0];
                const date = firstDesc?.timestamp ? formatTimestamp(firstDesc.timestamp) : "N/A";
                return (
                  <Box key={levelKey}>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}>
                      <Button
                        variant="text"
                        size="small"
                        color="warning"
                        onClick={() => handleLevelClick(levelKey)}
                        sx={{ textTransform: "none", fontWeight: 600, p: 0, minWidth: 0 }}
                      >
                        Level {levelKey}
                      </Button>
                      <Typography variant="caption" color="text.secondary">
                        — {date}
                      </Typography>
                    </Box>
                    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
                      {entry.updateDescriptionLevels.map((item, i) => (
                        <UpdateDescriptionItem key={i} item={item} />
                      ))}
                    </Box>
                  </Box>
                );
              })}
            </Box>
          )}

          <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: 3 }}>
            {reportData.productName} {reportData.productVersion} - Update Levels Report
          </Typography>

          <Divider sx={{ my: 3 }} />

          {/* WSO2-style Update Summary */}
          <Typography variant="subtitle1" fontWeight={700} sx={{ mb: 1.5 }}>
            Update Summary
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            {`This document summarizes the updates released for ${reportData.productName} ${reportData.productVersion} between update levels ${reportData.startLevel} and ${reportData.endLevel}. There are a total of ${reportData.totalUpdates} updates in this range${reportData.securityEntries.length > 0 ? `, of which ${reportData.securityEntries.length} are security updates` : ""}. It is highly recommended that you apply all these updates.`}
          </Typography>

          {reportData.securityEntries.length > 0 && (
            <Box sx={{ mb: 3 }}>
              <Typography variant="subtitle2" fontWeight={700} color="error.main" sx={{ mb: 1 }}>
                List of security updates
              </Typography>
              <Box component="ul" sx={{ pl: 3, m: 0 }}>
                {reportData.securityEntries.map((entry, i) => (
                  <Box component="li" key={i} sx={{ mb: 0.5 }}>
                    <Typography variant="body2">
                      {entry.formattedDate} (Update No: {entry.updateNumber})
                    </Typography>
                  </Box>
                ))}
              </Box>
            </Box>
          )}

          {reportData.entriesWithInstructions.length > 0 && (
            <Box sx={{ mb: 3 }}>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                Following updates contain instructions that must be followed when applying the update.
              </Typography>
              <Box component="ul" sx={{ pl: 3, m: 0 }}>
                {reportData.entriesWithInstructions.map((entry, i) => (
                  <Box component="li" key={i} sx={{ mb: 0.5 }}>
                    <Typography variant="body2">
                      {entry.formattedDate} (Update No: {entry.updateNumber})
                    </Typography>
                  </Box>
                ))}
              </Box>
            </Box>
          )}

          <Divider sx={{ my: 2 }} />

          <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 2 }}>
            Update Details
          </Typography>
          {reportData.allEntries.map((entry, i) => {
            const parsedDesc = parseDescriptionSections(entry.description);
            const bugFixUrls = parseBugFixes(entry.bugFixes);
            return (
              <Box key={i} sx={{ mb: 3 }}>
                <Box
                  sx={{
                    bgcolor: "warning.main",
                    color: "warning.contrastText",
                    px: 2,
                    py: 1,
                    borderRadius: 1,
                    mb: 1,
                  }}
                >
                  <Typography variant="subtitle2" fontWeight={700}>
                    {entry.formattedDate} - Update No: {entry.updateNumber}
                  </Typography>
                  <Typography variant="caption">
                    {entry.productName}-{entry.productVersion} ({entry.channel})
                  </Typography>
                </Box>
                {parsedDesc.generalDescription && (
                  <Box sx={{ mb: 1, pl: 1 }}>
                    <Typography variant="caption" fontWeight={700} display="block">
                      General Description:
                    </Typography>
                    <Typography variant="body2">{parsedDesc.generalDescription}</Typography>
                  </Box>
                )}
                {parsedDesc.implementationDetails && (
                  <Box sx={{ mb: 1, pl: 1 }}>
                    <Typography variant="caption" fontWeight={700} display="block">
                      Implementation Details:
                    </Typography>
                    <Typography variant="body2">{parsedDesc.implementationDetails}</Typography>
                  </Box>
                )}
                {parsedDesc.impact && (
                  <Box sx={{ mb: 1, pl: 1 }}>
                    <Typography variant="caption" fontWeight={700} display="block">
                      Impact:
                    </Typography>
                    <Typography variant="body2">{parsedDesc.impact}</Typography>
                  </Box>
                )}
                {bugFixUrls.length > 0 && (
                  <Box sx={{ mb: 1, pl: 1 }}>
                    <Typography variant="caption" fontWeight={700} display="block">
                      Bug Fixes:
                    </Typography>
                    <Box>
                      {bugFixUrls.map((url, j) => (
                        <Typography key={j} variant="body2">
                          {isSafeHttpUrl(url) ? (
                            <a href={url} target="_blank" rel="noreferrer" style={{ color: "#1976d2", textDecoration: "underline" }}>{url}</a>
                          ) : (
                            url
                          )}
                        </Typography>
                      ))}
                    </Box>
                  </Box>
                )}
                {isInstructionsNonEmpty(entry.instructions) && (
                  <Box sx={{ mb: 1 }}>
                    <Typography variant="caption" fontWeight={700} display="block" sx={{ pl: 1, mb: 0.5 }}>
                      Instructions:
                    </Typography>
                    <Box
                      component="pre"
                      sx={{
                        bgcolor: "grey.100",
                        border: "1px solid",
                        borderColor: "divider",
                        borderRadius: 1,
                        p: 1.5,
                        m: 0,
                        fontFamily: "monospace",
                        fontSize: "0.75rem",
                        whiteSpace: "pre-wrap",
                        wordBreak: "break-word",
                      }}
                    >
                      {entry.instructions?.trim()}
                    </Box>
                  </Box>
                )}
                {(entry.securityAdvisories?.length ?? 0) > 0 && (
                  <Box sx={{ pl: 1 }}>
                    <Typography
                      variant="caption"
                      fontWeight={700}
                      color="error.main"
                      display="block"
                      sx={{ mb: 0.5 }}
                    >
                      Security Advisories:
                    </Typography>
                    <Box component="ul" sx={{ pl: 2, m: 0 }}>
                      {entry.securityAdvisories.map((adv, j) => (
                        <Box component="li" key={j} sx={{ mb: 0.25 }}>
                          <Typography variant="caption">
                            <strong>{adv.id}</strong> ({adv.severity}): {adv.overview}
                          </Typography>
                        </Box>
                      ))}
                    </Box>
                  </Box>
                )}
              </Box>
            );
          })}
        </Box>
      </DialogContent>
      <DialogActions
        className="print-hide"
        sx={{ "@media print": { display: "none !important" } }}
      >
        <Button
          variant="outlined"
          color="warning"
          startIcon={isDownloading ? <CircularProgress size={16} color="inherit" /> : <Printer size={18} />}
          onClick={handleDownloadPdf}
          disabled={isDownloading}
        >
          {isDownloading ? "Downloading..." : "Download PDF"}
        </Button>
        <Button variant="contained" color="warning" onClick={onClose}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}
