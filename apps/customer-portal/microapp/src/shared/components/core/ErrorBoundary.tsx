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

// refer: https://react.dev/reference/react/Component#catching-rendering-errors-with-an-error-boundary
import * as React from "react";

interface ErrorBoundaryProps {
  children: React.ReactNode;
  fallback: React.ReactNode | ((error: unknown, resetErrorBoundary: () => void) => React.ReactNode);
  onError?: (error: unknown) => void;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: unknown;
}

export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: unknown, info: React.ErrorInfo): void {
    const ownerStack = React.captureOwnerStack?.();
    console.error(error, info.componentStack, ownerStack);
    this.props.onError?.(error);
  }

  resetErrorBoundary = () => {
    this.setState({ hasError: false, error: null });
  };

  render(): React.ReactNode {
    const { hasError, error } = this.state;
    const { fallback, children } = this.props;

    if (hasError && error) {
      if (typeof fallback === "function") {
        return fallback(error, this.resetErrorBoundary);
      }
      return fallback;
    }

    return children;
  }
}
