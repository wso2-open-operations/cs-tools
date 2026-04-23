// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Typography } from "@wso2/oxygen-ui";
import { Bot } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

/**
 * Three warning-colored pulsing dots for bot loading state.
 * Displays Novera header with icon and loading dots below (no bubble wrapper).
 *
 * @returns {JSX.Element} Loading indicator with Novera header.
 */
export default function LoadingDotsBubble(): JSX.Element {
  return (
    <Box
      role="status"
      aria-live="polite"
      aria-label="Novera is preparing a response"
      sx={{ maxWidth: "80%" }}
    >
      <Box sx={{ mb: 3 }}>
        {/* Header with icon and Novera label */}
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 3 }}>
          <Box
            sx={{
              width: 24,
              height: 24,
              borderRadius: "50%",
              background: "linear-gradient(135deg, #EA580C 0%, #F97316 100%)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
          >
            <Bot size={16} color="white" />
          </Box>
          <Typography
            variant="body2"
            sx={{ fontWeight: 600, color: "text.primary" }}
          >
            Novera
          </Typography>
        </Box>

        {/* Loading dots - no paper bubble wrapper */}
        <Box
          sx={{
            display: "flex",
            gap: 0.5,
            alignItems: "center",
          }}
        >
          <Box
            sx={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              bgcolor: "warning.main",
              animation: "loadingDotPulse 1.4s ease-in-out infinite both",
              "@keyframes loadingDotPulse": {
                "0%, 80%, 100%": { opacity: 0.3, transform: "scale(0.8)" },
                "40%": { opacity: 1, transform: "scale(1)" },
              },
            }}
          />
          <Box
            sx={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              bgcolor: "warning.main",
              animation: "loadingDotPulse 1.4s ease-in-out infinite both",
              animationDelay: "0.2s",
            }}
          />
          <Box
            sx={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              bgcolor: "warning.main",
              animation: "loadingDotPulse 1.4s ease-in-out infinite both",
              animationDelay: "0.4s",
            }}
          />
        </Box>
      </Box>
    </Box>
  );
}
