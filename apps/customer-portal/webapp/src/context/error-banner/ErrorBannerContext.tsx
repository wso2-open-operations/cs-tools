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

import ErrorBanner from "@components/common/error-banner/ErrorBanner";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
  type JSX,
} from "react";

import { ERROR_BANNER_TIMEOUT_MS } from "@constants/errorBannerConstants";

interface ErrorBannerContextType {
  /** Show the error banner with the given user-facing message. */
  showError: (message: string) => void;
}

const ErrorBannerContext = createContext<ErrorBannerContextType | undefined>(
  undefined,
);

interface ErrorBannerProviderProps {
  children: ReactNode;
}

/**
 * ErrorBannerProvider provides a global error banner above the footer.
 * Any component can call showError(message) to display the given message.
 *
 * @param {ErrorBannerProviderProps} props - Provider props.
 * @returns {JSX.Element} The provider with embedded banner.
 */
export function ErrorBannerProvider({
  children,
}: ErrorBannerProviderProps): JSX.Element {
  const [message, setMessage] = useState<string | null>(null);
  const [key, setKey] = useState(0);

  const showError = useCallback((msg: string) => {
    setMessage(msg);
    setKey((prev) => prev + 1);
  }, []);

  const dismiss = useCallback(() => {
    setMessage(null);
  }, []);

  useEffect(() => {
    if (!message) return;

    const timeoutId = setTimeout(() => {
      dismiss();
    }, ERROR_BANNER_TIMEOUT_MS);

    return () => {
      clearTimeout(timeoutId);
    };
  }, [message, key, dismiss]);

  const visible = message !== null;
  const contextValue = useMemo(() => ({ showError }), [showError]);

  return (
    <ErrorBannerContext.Provider value={contextValue}>
      {children}
      {visible && message && (
        <ErrorBanner message={message} onClose={dismiss} />
      )}
    </ErrorBannerContext.Provider>
  );
}

/**
 * Hook to access the error banner. Call showError(message) to display the banner.
 *
 * @returns {ErrorBannerContextType} The error banner API.
 */
export function useErrorBanner(): ErrorBannerContextType {
  const context = useContext(ErrorBannerContext);
  if (context === undefined) {
    throw new Error(
      "useErrorBanner must be used within an ErrorBannerProvider",
    );
  }
  return context;
}
