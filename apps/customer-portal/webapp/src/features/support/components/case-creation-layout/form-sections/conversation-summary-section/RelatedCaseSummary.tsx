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

import type { RelatedCaseSummaryProps } from "@features/support/types/supportComponents";
import { Box, Divider, Paper, Typography } from "@wso2/oxygen-ui";
import { FileText } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import DOMPurify from "dompurify";

/**
 * Sidebar component showing related case details when creating a case from "Open Related Case".
 *
 * @param {RelatedCaseSummaryProps} props - Related case number, title, description.
 * @returns {JSX.Element} The related case summary sidebar.
 */
export function RelatedCaseSummary({
  number,
  title,
  description,
}: RelatedCaseSummaryProps): JSX.Element {
  const sanitizedDescription = DOMPurify.sanitize(description ?? "");

  return (
    <Paper sx={{ p: 3, position: "sticky", top: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 3 }}>
        <FileText size={18} />
        <Typography variant="h6">Related Case Details</Typography>
      </Box>

      <Divider sx={{ mb: 3 }} />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Box>
          <Typography variant="caption" color="text.secondary" display="block">
            Case Number
          </Typography>
          <Box
            sx={{
              bgcolor: "action.hover",
              px: 2,
              py: 1.5,
              mt: 0.5,
            }}
          >
            <Typography variant="body2">{number || "--"}</Typography>
          </Box>
        </Box>
        <Box>
          <Typography variant="caption" color="text.secondary" display="block">
            Title
          </Typography>
          <Box
            sx={{
              bgcolor: "action.hover",
              px: 2,
              py: 1.5,
              mt: 0.5,
            }}
          >
            <Typography variant="body2">{title || "--"}</Typography>
          </Box>
        </Box>
        <Box>
          <Typography variant="caption" color="text.secondary" display="block">
            Description
          </Typography>
          <Box
            sx={{
              bgcolor: "action.hover",
              px: 2,
              py: 1.5,
              mt: 0.5,
            }}
          >
            {description.trim() ? (
              <Box
                sx={{
                  typography: "body2",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                  "& p": { my: 0 },
                  "& a": { color: "primary.main", textDecoration: "underline" },
                }}
                dangerouslySetInnerHTML={{ __html: sanitizedDescription }}
              />
            ) : (
              <Typography variant="body2">--</Typography>
            )}
          </Box>
        </Box>
      </Box>
    </Paper>
  );
}
