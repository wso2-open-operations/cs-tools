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
  Paper,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { FileText, Sparkles } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

interface EscalationBannerProps {
  visible: boolean;
  onCreateCase: () => void;
  isLoading?: boolean;
  /** When true, Create Case button is disabled (e.g. product options failed to load). */
  isCreateCaseDisabled?: boolean;
}

const NOVERA_PROSE =
  "Thank you for describing the issue. To provide more targeted assistance:\n\n1. What WSO2 product and version are you using?\n2. Is this in a production or non-production environment?\n3. Can you provide more details about when this issue occurs?";

/**
 * Renders an escalation banner for creating support cases.
 *
 * Displays Novera badge, guidance text, and a Create Case button with
 * "Skip the chat and create a support case now" option.
 *
 * @returns The EscalationBanner JSX element or null if not visible.
 */
export default function EscalationBanner({
  visible,
  onCreateCase,
  isLoading = false,
  isCreateCaseDisabled = false,
}: EscalationBannerProps): JSX.Element | null {
  if (!visible) return null;

  return (
    <Paper
      sx={{
        mb: 2,
        py: 2,
        px: 2,
        maxWidth: 672,
      }}
    >
      <Stack spacing={2}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Box
            sx={{
              width: 24,
              height: 24,
              borderRadius: "50%",
              background: (theme) =>
                `linear-gradient(to bottom right, ${theme.palette.warning.main}, ${theme.palette.warning.dark})`,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
          >
            <Sparkles size={14} color="white" />
          </Box>
          <Typography variant="body2" fontWeight={500}>
            Novera
          </Typography>
        </Box>

        <Typography
          variant="body2"
          color="text.secondary"
          sx={{ whiteSpace: "pre-line", lineHeight: 1.6 }}
        >
          {NOVERA_PROSE}
        </Typography>

        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          <Button
            variant="contained"
            size="small"
            color="warning"
            onClick={onCreateCase}
            loading={isLoading}
            loadingPosition="start"
            disabled={isCreateCaseDisabled}
            startIcon={<FileText size={14} />}
          >
            Create Case
          </Button>
          <Typography variant="caption" color="text.secondary">
            Skip the chat and create a support case now
          </Typography>
        </Box>
      </Stack>
    </Paper>
  );
}
