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

import { Box, Button, Paper, Stack, Typography } from "@wso2/oxygen-ui";
import { ExternalLink, LifeBuoy } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { getSupportUrl } from "@config/apiConfig";
import { primaryTint } from "@config/colorUtils";

interface SupportCalloutProps {
  /** Tighter padding and stacked layout, for a sidebar. */
  compact?: boolean;
}

/**
 * "Still need help?" panel pointing at the support channel. Rendered only
 * when a support URL is configured, so the portal never shows a dead end it
 * cannot resolve.
 */
export default function SupportCallout({ compact }: SupportCalloutProps): JSX.Element | null {
  const supportUrl = getSupportUrl();
  if (!supportUrl) return null;
  return (
    <Paper
      component="aside"
      aria-labelledby="support-callout-heading"
      variant="outlined"
      className="kb-no-print"
      sx={(theme) => ({
        p: compact ? 2.5 : { xs: 3, md: 4 },
        borderRadius: 2,
        bgcolor: primaryTint(theme, 0.05),
      })}
    >
      <Stack
        direction={compact ? "column" : { xs: "column", sm: "row" }}
        spacing={compact ? 2 : 3}
        alignItems={compact ? "flex-start" : { xs: "flex-start", sm: "center" }}
      >
        <Box
          sx={(theme) => ({
            width: 44,
            height: 44,
            borderRadius: 1.5,
            flexShrink: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            bgcolor: primaryTint(theme, 0.14),
            color: theme.palette.primary.main,
            ...theme.applyStyles("light", { color: theme.palette.primary.dark }),
          })}
        >
          <LifeBuoy size={22} aria-hidden />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography id="support-callout-heading" variant="h6" component="h2" sx={{ fontWeight: 700, lineHeight: 1.3 }}>
            Still need help?
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            If these articles did not answer your question, the WSO2 support team can.
          </Typography>
        </Box>
        <Button
          variant="outlined"
          href={supportUrl}
          target="_blank"
          rel="noopener noreferrer"
          endIcon={<ExternalLink size={16} aria-hidden />}
          sx={{ flexShrink: 0, whiteSpace: "nowrap" }}
        >
          Contact support
        </Button>
      </Stack>
    </Paper>
  );
}
