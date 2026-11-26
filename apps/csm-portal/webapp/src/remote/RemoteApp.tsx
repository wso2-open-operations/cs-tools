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
import Prism from "prismjs";
import "../styles/print.css";
import AppTree from "../AppTree";
import { HostContext, type Host } from "@context/host/HostContext";

// Same global main.tsx sets for the standalone portal.
(window as unknown as { Prism: typeof Prism }).Prism = Prism;

/** The portal without its own sign-in, for rendering inside a host. */
export default function RemoteApp({ basename, host }: { basename: string; host: Host }): JSX.Element {
  return (
    <HostContext.Provider value={host}>
      <AppTree basename={basename} />
    </HostContext.Provider>
  );
}
