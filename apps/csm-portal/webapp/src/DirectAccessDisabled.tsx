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
import { type JSX } from "react";
import { appName } from "@config/appearanceConfig";
import { useAppMountedSignal } from "@utils/appMountedEvent";

/**
 * Shown in place of the portal when it is opened at its own address while
 * direct access is turned off and there is no host address to send the person
 * to. Rendered before any provider exists, so it uses plain elements and the
 * browser's own colours for light and dark.
 */
export default function DirectAccessDisabled(): JSX.Element {
  // Takes down the loading screen from index.html, which otherwise waits for
  // the portal itself to mount.
  useAppMountedSignal();
  return (
    <main
      style={{
        colorScheme: "light dark",
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "0.5rem",
        padding: "1.5rem",
        textAlign: "center",
        fontFamily: "system-ui, sans-serif",
      }}
    >
      <h1 style={{ fontSize: "1.25rem", margin: 0 }}>{appName()} is not available at this address</h1>
      <p style={{ margin: 0, maxWidth: "32rem" }}>
        Direct access is turned off. Open it from the application you normally use it in.
      </p>
    </main>
  );
}
