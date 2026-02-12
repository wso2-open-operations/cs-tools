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

import SuccessBanner from "@components/common/success-banner/SuccessBanner";
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

interface SuccessBannerContextType {
  /** Show the success banner with the given message. */
  showSuccess: (message: string) => void;
}

const SuccessBannerContext = createContext<
  SuccessBannerContextType | undefined
>(undefined);

interface SuccessBannerProviderProps {
  children: ReactNode;
}

/**
 * SuccessBannerProvider provides a global success banner above the footer.
 * Any component can call showSuccess(message) to display the banner.
 *
 * @param {SuccessBannerProviderProps} props - Provider props.
 * @returns {JSX.Element} The provider with embedded banner.
 */
export function SuccessBannerProvider({
  children,
}: SuccessBannerProviderProps): JSX.Element {
  const [message, setMessage] = useState<string | null>(null);
  const [key, setKey] = useState(0);

  const showSuccess = useCallback((msg: string) => {
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
  const contextValue = useMemo(() => ({ showSuccess }), [showSuccess]);

  return (
    <SuccessBannerContext.Provider value={contextValue}>
      {children}
      {visible && message && (
        <SuccessBanner message={message} onClose={dismiss} />
      )}
    </SuccessBannerContext.Provider>
  );
}

/**
 * Hook to access the success banner. Call showSuccess(message) to display the banner.
 *
 * @returns {SuccessBannerContextType} The success banner API.
 */
export function useSuccessBanner(): SuccessBannerContextType {
  const context = useContext(SuccessBannerContext);
  if (context === undefined) {
    throw new Error(
      "useSuccessBanner must be used within a SuccessBannerProvider",
    );
  }
  return context;
}
