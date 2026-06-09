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

/* eslint-disable react-refresh/only-export-components -- Provider component and its useXxx hook are colocated per the repo's context idiom (fast-refresh DX only) */

import {
  createContext,
  useContext,
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";

interface ErrorPageContextType {
  isErrorPageDisplayed: boolean;
  setIsErrorPageDisplayed: (isDisplayed: boolean) => void;
  isProjectSuspended: boolean;
  setIsProjectSuspended: (isSuspended: boolean) => void;
}

const ErrorPageContext = createContext<ErrorPageContextType | undefined>(undefined);

export function useErrorPageContext(): ErrorPageContextType {
  const context = useContext(ErrorPageContext);
  if (!context) {
    throw new Error(
      "useErrorPageContext must be used within ErrorPageProvider"
    );
  }
  return context;
}

interface ErrorPageProviderProps {
  children: ReactNode;
}

export function ErrorPageProvider({
  children,
}: ErrorPageProviderProps): JSX.Element {
  const [isErrorPageDisplayed, setIsErrorPageDisplayed] = useState(false);
  const [isProjectSuspended, setIsProjectSuspended] = useState(false);

  const value = useMemo(
    () => ({
      isErrorPageDisplayed,
      setIsErrorPageDisplayed,
      isProjectSuspended,
      setIsProjectSuspended,
    }),
    [isErrorPageDisplayed, isProjectSuspended],
  );

  return (
    <ErrorPageContext.Provider value={value}>
      {children}
    </ErrorPageContext.Provider>
  );
}

