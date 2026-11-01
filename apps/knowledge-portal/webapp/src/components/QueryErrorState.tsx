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
import {
  AlertTriangle,
  FileQuestion,
  ServerCrash,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";
import type { ElementType, JSX } from "react";
import { BackendApiError } from "@api/backend/client";

interface QueryErrorStateProps {
  message: string;
  /** The raw error, used to pick the icon and title from the HTTP status. */
  error?: unknown;
  /** Renders a "Try again" button. Omit for errors retrying cannot fix. */
  onRetry?: () => void;
  /** Heading level of the title. Use "h1" when this is the page's main content. */
  headingComponent?: ElementType;
}

function resolvePresentation(error: unknown): { Icon: LucideIcon; title: string } {
  if (error instanceof BackendApiError) {
    if (error.status === 404) return { Icon: FileQuestion, title: "Not found" };
    if (error.status >= 400 && error.status < 500) {
      return { Icon: AlertTriangle, title: "Something's not right" };
    }
  }
  return { Icon: ServerCrash, title: "Something went wrong" };
}

/** Inline error state for a failed data load. Announced to assistive tech. */
export default function QueryErrorState({
  message,
  error,
  onRetry,
  headingComponent = "h2",
}: QueryErrorStateProps): JSX.Element {
  const { Icon, title } = resolvePresentation(error);
  return (
    <Box role="alert" sx={{ display: "flex", justifyContent: "center", px: 3, py: 4 }}>
      <Stack spacing={1.5} alignItems="center" sx={{ maxWidth: 420 }}>
        <Icon size={36} strokeWidth={1.5} style={{ opacity: 0.6 }} aria-hidden />
        <Typography variant="subtitle1" component={headingComponent} fontWeight={700}>
          {title}
        </Typography>
        <Typography variant="body2" color="text.secondary" textAlign="center">
          {message}
        </Typography>
        {onRetry && (
          <Button size="small" variant="outlined" onClick={onRetry}>
            Try again
          </Button>
        )}
      </Stack>
    </Box>
  );
}
