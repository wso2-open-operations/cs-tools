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

import { type JSX, lazy, Suspense } from "react";
import type { Host } from "@context/host/HostContext";

/** What a host passes when it renders the portal. */
export interface CsmRemoteProps {
  /** Path the host mounts the portal under, e.g. "/csm". */
  basename: string;
  /** The host itself, so the portal can offer a way back to it. */
  host: Host;
}

// This file is what a host application loads (see `exposes` in
// vite.config.ts). It is fetched from this portal's own deployment, so its URL
// tells us where that is without the address being built in.
const remoteBase = new URL(import.meta.env.BASE_URL, new URL(import.meta.url).origin);

let configLoaded: Promise<void> | undefined;

/**
 * Loads this portal's own /config.js into the host's page.
 *
 * The portal reads `window.config` while its modules are evaluated, and inside
 * a host that object is the host's. config.js assigns `window.config` outright,
 * so the host's object is put back on top of the portal's keys afterwards: the
 * host keeps everything it had and the portal's keys are added alongside.
 */
function loadPortalConfig(): Promise<void> {
  configLoaded ??= new Promise<void>((resolve, reject) => {
    const hostConfig = window.config;
    const script = document.createElement("script");
    script.src = new URL("config.js", remoteBase).href;
    script.onload = () => {
      window.config = { ...window.config, ...hostConfig };
      resolve();
    };
    script.onerror = () => {
      window.config = hostConfig;
      reject(new Error(`Could not load ${script.src}`));
    };
    document.head.appendChild(script);
  });
  return configLoaded;
}

// Imported only once the config is in place, because the portal's config
// modules read it as soon as they are evaluated.
const RemoteApp = lazy(async () => {
  await loadPortalConfig();
  return import("./RemoteApp");
});

/**
 * The portal as a component for a host application to render. It fills the
 * whole page with its own header and sidebar, so the host must render it
 * outside its own frame, inside its `AsgardeoProvider`, with the user already
 * signed in.
 */
export default function RemoteEntry({ basename, host }: CsmRemoteProps): JSX.Element {
  return (
    <Suspense fallback={null}>
      <RemoteApp basename={basename} host={host} />
    </Suspense>
  );
}
