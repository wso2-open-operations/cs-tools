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

import { useState, useEffect } from "react";

function readDarkMode(): boolean {
  return document.documentElement.getAttribute("data-color-scheme") === "dark";
}

/**
 * Observes the <html data-color-scheme> attribute via MutationObserver so
 * components can reactively respond to dark/light mode changes.
 */
export function useDarkMode(): boolean {
  const [dark, setDark] = useState<boolean>(readDarkMode);

  useEffect(() => {
    const observer = new MutationObserver(() => {
      setDark(readDarkMode());
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-color-scheme"],
    });

    return () => observer.disconnect();
  }, []);

  return dark;
}
