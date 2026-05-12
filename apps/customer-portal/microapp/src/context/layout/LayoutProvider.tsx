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
import { useCallback, useMemo, useState } from "react";

import { useLocation } from "react-router-dom";

import { LayoutContext, type LayoutContextType, type LayoutDeclaration } from "@context/layout";

import { DEFAULT_LAYOUT_CONFIG } from "@shared/constants";

export default function LayoutProvider({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation();
  const [declaration, setDeclaration] = useState<LayoutDeclaration>(DEFAULT_LAYOUT_CONFIG);

  const declareLayout = useCallback((next: Partial<LayoutDeclaration>) => {
    setDeclaration({
      ...DEFAULT_LAYOUT_CONFIG,
      ...next,
      visibility: { ...DEFAULT_LAYOUT_CONFIG.visibility, ...next.visibility },
      slots: { ...DEFAULT_LAYOUT_CONFIG.slots, ...next.slots },
    });
  }, []);

  const value: LayoutContextType = useMemo(() => ({ ...declaration, declareLayout }), [declaration]);

  return (
    <LayoutContext.Provider key={pathname} value={value}>
      {children}
    </LayoutContext.Provider>
  );
}
