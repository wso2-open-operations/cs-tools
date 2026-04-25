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

import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import { CircleX, RotateCcw } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

interface ErrorStateProps {
  title?: string;
  description?: string;
  icon?: ReactNode;
  onRetry?: () => void;
  retryButtonText?: string;
}

export default function ErrorState({
  title = "Something went wrong",
  description = "We encountered an unexpected error while loading this content.",
  icon = <CircleX size={48} />,
  onRetry,
  retryButtonText = "Try Again",
}: ErrorStateProps) {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        py: 8,
        px: 2,
        textAlign: "center",
        borderRadius: 2,
      }}
    >
      <Stack spacing={2} alignItems="center">
        <Box
          sx={{
            color: "text.secondary",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            mb: 1,
            opacity: 0.6,
          }}
        >
          {icon}
        </Box>

        <Box>
          <Typography variant="h6" fontWeight="600" gutterBottom>
            {title}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 300 }}>
            {description}
          </Typography>
        </Box>
        {onRetry && (
          <Button
            variant="contained"
            color="secondary"
            onClick={onRetry}
            size="small"
            endIcon={<RotateCcw size={16} />}
          >
            {retryButtonText}
          </Button>
        )}
      </Stack>
    </Box>
  );
}
