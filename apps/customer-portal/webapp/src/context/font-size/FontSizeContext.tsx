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

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
  type JSX,
} from "react";
import {
  getFontSize,
  setFontSize,
  type FontSizeOption,
} from "@features/settings/utils/settingsStorage";

export type { FontSizeOption };

export const FONT_SIZE_PX: Record<FontSizeOption, string> = {
  small: "13px",
  medium: "16px",
  large: "18px",
  xlarge: "20px",
};

interface FontSizeContextType {
  fontSize: FontSizeOption;
  setFontSizeOption: (size: FontSizeOption) => void;
}

const FontSizeContext = createContext<FontSizeContextType | undefined>(undefined);

/**
 * Provides font size preference to the component tree.
 * Reads the initial value from localStorage, applies it to the HTML root element,
 * and persists changes so they survive page reloads.
 *
 * @param {{ children: ReactNode }} props - Component props.
 * @returns {JSX.Element} The provider wrapping its children.
 */
export function FontSizeProvider({
  children,
}: {
  children: ReactNode;
}): JSX.Element {
  const [fontSize, setFontSizeState] = useState<FontSizeOption>(getFontSize);

  useEffect(() => {
    document.documentElement.style.fontSize = FONT_SIZE_PX[fontSize];
  }, [fontSize]);

  const setFontSizeOption = useCallback((size: FontSizeOption) => {
    setFontSizeState(size);
    setFontSize(size);
  }, []);

  return (
    <FontSizeContext.Provider value={{ fontSize, setFontSizeOption }}>
      {children}
    </FontSizeContext.Provider>
  );
}

/**
 * Returns the current font size preference and a setter from the nearest FontSizeProvider.
 * Throws if called outside of a FontSizeProvider.
 *
 * @returns {FontSizeContextType} The font size context value.
 */
export function useFontSize(): FontSizeContextType {
  const context = useContext(FontSizeContext);
  if (context === undefined) {
    throw new Error("useFontSize must be used within a FontSizeProvider");
  }
  return context;
}
