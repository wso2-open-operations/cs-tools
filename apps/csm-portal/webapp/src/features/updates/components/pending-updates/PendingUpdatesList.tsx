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
  Chip,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { PendingUpdatesListProps } from "@features/updates/types/updates";
import { getUpdateTypeChipColor } from "@features/updates/utils/updates";
import EmptyState from "@components/empty-state/EmptyState";
import error500Svg from "@assets/error/error-500.svg";
import {
  PENDING_UPDATES_LIST_EMPTY_DESCRIPTION,
  PENDING_UPDATES_LIST_ERROR_MESSAGE,
  PENDING_UPDATES_TABLE_VIEW_BUTTON_LABEL,
} from "@features/updates/constants/updatesConstants";

/**
 * Displays pending update levels as a simple table.
 * Each row shows the update level number, update type chip, and a View button.
 *
 * @param {PendingUpdatesListProps} props - Response data, error state, and view callback.
 * @returns {JSX.Element} The rendered pending updates table.
 */
export function PendingUpdatesList({
  data,
  isError,
  onView,
}: PendingUpdatesListProps): JSX.Element {
  const theme = useTheme();

  if (isError) {
    return (
      <Box
        sx={{
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          flexDirection: "column",
          minHeight: "55vh",
        }}
      >
        <img src={error500Svg} alt="" aria-hidden="true" style={{ width: 300, height: "auto" }} />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          {PENDING_UPDATES_LIST_ERROR_MESSAGE}
        </Typography>
      </Box>
    );
  }

  const entries = data
    ? Object.entries(data).sort(([a], [b]) => Number(a) - Number(b))
    : [];

  const securityCount = entries.filter(
    ([, e]) => e.updateType === "security",
  ).length;
  const regularCount = entries.filter(
    ([, e]) => e.updateType === "regular",
  ).length;
  const mixedCount = entries.filter(([, e]) => e.updateType === "mixed").length;

  if (entries.length === 0) {
    return (
      <EmptyState description={PENDING_UPDATES_LIST_EMPTY_DESCRIPTION} />
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
        There are <strong>{entries.length}</strong> updates with{" "}
        <strong>{securityCount}</strong> security,{" "}
        <strong>{regularCount}</strong> regular
        {mixedCount > 0 && (
          <>
            , and <strong>{mixedCount}</strong> mixed
          </>
        )}{" "}
        updates.
      </Typography>

      <TableContainer component={Paper}>
        <Table
          sx={{
            minWidth: 650,
            width: "100%",
            tableLayout: "fixed",
          }}
        >
          <TableHead>
            <TableRow>
              <TableCell
                align="left"
                sx={{
                  fontWeight: 600,
                  textTransform: "uppercase",
                  fontSize: "0.75rem",
                  width: "33.33%",
                  textAlign: "left",
                }}
              >
                Update Level
              </TableCell>
              <TableCell
                align="center"
                sx={{
                  fontWeight: 600,
                  textTransform: "uppercase",
                  fontSize: "0.75rem",
                  width: "33.33%",
                  textAlign: "center",
                }}
              >
                Update Type
              </TableCell>
              <TableCell
                align="right"
                sx={{
                  fontWeight: 600,
                  textTransform: "uppercase",
                  fontSize: "0.75rem",
                  width: "33.33%",
                  textAlign: "right",
                }}
              >
                Details
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {entries.map(([levelKey, entry]) => {
              const chipColor = getUpdateTypeChipColor(entry.updateType);
              return (
                <TableRow
                  key={levelKey}
                  hover
                  sx={{
                    "&:last-child td, &:last-child th": { border: 0 },
                  }}
                >
                  <TableCell align="left" sx={{ textAlign: "left" }}>
                    <Typography variant="body2" color="text.primary">
                      {levelKey}
                    </Typography>
                  </TableCell>
                  <TableCell align="center" sx={{ textAlign: "center" }}>
                    <Box
                      sx={{
                        display: "flex",
                        justifyContent: "center",
                        width: "100%",
                      }}
                    >
                      <Chip
                        label={
                          entry.updateType.charAt(0).toUpperCase() +
                          entry.updateType.slice(1)
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
                  </TableCell>
                  <TableCell align="right" sx={{ textAlign: "right" }}>
                    <Button
                      variant="text"
                      size="small"
                      onClick={() => onView(levelKey)}
                      sx={{
                        color: "warning.main",
                        fontWeight: 500,
                        p: 0,
                        minWidth: 0,
                        textTransform: "none",
                        "&:hover": {
                          bgcolor: "transparent",
                          textDecoration: "underline",
                          color: "warning.dark",
                        },
                      }}
                    >
                      {PENDING_UPDATES_TABLE_VIEW_BUTTON_LABEL}
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}
