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
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { Printer, X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, type JSX } from "react";
import { generateUpdateLevelsReportPdf } from "@features/updates/utils/updateLevelsReportPdf";
import type { UpdateLevelsReportModalProps } from "@features/updates/types/updates";

/**
 * Modal that displays the Update Levels Report.
 * User can click "Download PDF" to generate and download the report via jspdf-autotable.
 *
 * @param {UpdateLevelsReportModalProps} props - open, reportData, onClose.
 * @returns {JSX.Element | null} The report modal or null when reportData is missing.
 */
export default function UpdateLevelsReportModal({
  open,
  reportData,
  onClose,
}: UpdateLevelsReportModalProps): JSX.Element | null {
  const handleDownloadPdf = useCallback(() => {
    if (!reportData) return;
    generateUpdateLevelsReportPdf(reportData);
  }, [reportData]);

  if (!reportData) {
    return null;
  }

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
            <Typography variant="body2" component="p">
              Product: {reportData.productName}
            </Typography>
            <Typography variant="body2" component="p">
              Version: {reportData.productVersion}
            </Typography>
            <Typography variant="body2" component="p">
              Update Level Range: {reportData.startLevel} to {reportData.endLevel}
            </Typography>
            <Typography variant="body2" component="p">
              View Mode: All Updates
            </Typography>
            <Typography variant="body2" component="p">
              Total Updates: {reportData.totalUpdates}
            </Typography>
            <Typography variant="body2" component="p">
              Update Levels: {reportData.levelCount} levels ({reportData.levelsRange})
            </Typography>
          </Box>

          <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 1 }}>
            Update Levels Summary
          </Typography>
          <Box sx={{ mb: 2, "& p": { m: 0, fontSize: "0.875rem" } }}>
            <Typography variant="body2" component="p">
              Security Updates: {reportData.securityCount}
            </Typography>
            <Typography variant="body2" component="p">
              Regular Updates: {reportData.regularCount}
            </Typography>
            <Typography variant="body2" component="p">
              Mixed Updates: {reportData.mixedCount}
            </Typography>
            <Typography variant="body2" component="p">
              Applied Updates: N/A
            </Typography>
            <Typography variant="body2" component="p">
              Pending Updates: {reportData.levelCount}
            </Typography>
          </Box>

          <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 1 }}>
            Update Levels Details
          </Typography>
          <TableContainer component={Paper} variant="outlined" sx={{ mb: 2 }}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell sx={{ fontWeight: 600, fontSize: "0.75rem" }}>Level</TableCell>
                  <TableCell sx={{ fontWeight: 600, fontSize: "0.75rem" }}>Type</TableCell>
                  <TableCell sx={{ fontWeight: 600, fontSize: "0.75rem" }}>Updates</TableCell>
                  <TableCell sx={{ fontWeight: 600, fontSize: "0.75rem" }}>Release Date</TableCell>
                  <TableCell sx={{ fontWeight: 600, fontSize: "0.75rem" }}>Applied</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {reportData.tableRows.map((row) => (
                  <TableRow key={row.levelKey}>
                    <TableCell>{row.levelKey}</TableCell>
                    <TableCell>{row.typeLabel}</TableCell>
                    <TableCell>{row.updatesCount}</TableCell>
                    <TableCell>{row.releaseDate}</TableCell>
                    <TableCell>{row.applied}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>

          <Typography variant="caption" color="text.secondary">
            {reportData.productName} {reportData.productVersion} - Update Levels Report
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions
        className="print-hide"
        sx={{
          "@media print": { display: "none !important" },
        }}
      >
        <Button variant="outlined" color="warning" startIcon={<Printer size={18} />} onClick={handleDownloadPdf}>
          Download PDF
        </Button>
        <Button variant="contained" color="warning" onClick={onClose}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}
