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

import { useLogger } from "@/hooks/useLogger";
import {
  createContext,
  useContext,
  useState,
  type JSX,
  type ReactNode,
} from "react";

interface MockConfigContextProps {
  isMockEnabled: boolean;
  setMockEnabled: (enabled: boolean) => void;
}

const MockConfigContext = createContext<MockConfigContextProps | undefined>(
  undefined,
);

export const MockConfigProvider = ({
  children,
}: {
  children: ReactNode;
}): JSX.Element => {
  const logger = useLogger();
  const [isMockEnabled, setIsMockEnabledState] = useState<boolean>(() => {
    if (
      typeof window === "undefined" ||
      typeof window.localStorage === "undefined"
    ) {
      return false;
    }

    try {
      const storedValue = window.localStorage.getItem("isMockEnabled");
      return storedValue !== null ? storedValue === "true" : false;
    } catch {
      return false;
    }
  });

  const setMockEnabled = (enabled: boolean) => {
    setIsMockEnabledState(enabled);

    if (
      typeof window === "undefined" ||
      typeof window.localStorage === "undefined"
    ) {
      return;
    }

    try {
      localStorage.setItem("isMockEnabled", String(enabled));
    } catch (error) {
      logger.error(
        "Failed to persist 'isMockEnabled' in localStorage.",
        error,
      );
    }
  };

  return (
    <MockConfigContext.Provider value={{ isMockEnabled, setMockEnabled }}>
      {children}
    </MockConfigContext.Provider>
  );
};

export const useMockConfig = (): MockConfigContextProps => {
  const context = useContext(MockConfigContext);
  if (!context) {
    throw new Error("useMockConfig must be used within a MockConfigProvider");
  }
  return context;
};
