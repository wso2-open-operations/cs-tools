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

import { Component, type ErrorInfo, type ReactNode } from "react";
import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";

interface AppErrorBoundaryProps {
  children: ReactNode;
}

interface AppErrorBoundaryState {
  hasError: boolean;
}

/**
 * Top-level error boundary so an uncaught render error degrades to a
 * recoverable "something went wrong" screen instead of unmounting the whole
 * app to a blank page. Class component by necessity: error boundaries have no
 * hook equivalent, which also means the context-based logger is unavailable
 * here — console is the only sink that cannot itself fail.
 */
export default class AppErrorBoundary extends Component<
  AppErrorBoundaryProps,
  AppErrorBoundaryState
> {
  state: AppErrorBoundaryState = { hasError: false };

  /** Flip to the fallback UI when a descendant throws during render. */
  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true };
  }

  /** Log the captured error and component stack to the console sink. */
  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error("Unhandled render error:", error, errorInfo.componentStack);
  }

  /** Render the children, or the recovery screen once an error is caught. */
  render(): ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }
    return (
      <Box
        sx={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          px: 3,
        }}
      >
        <Stack spacing={2} alignItems="center" sx={{ maxWidth: 480 }}>
          <Typography variant="h5">Something went wrong</Typography>
          <Typography variant="body1" color="text.secondary" textAlign="center">
            Something went wrong loading this page. Try reloading — your session
            will be kept.
          </Typography>
          <Button variant="contained" onClick={() => window.location.reload()}>
            Reload page
          </Button>
        </Stack>
      </Box>
    );
  }
}
