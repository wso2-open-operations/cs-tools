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

import { Alert, Box } from "@wso2/oxygen-ui";
import { type JSX, useEffect, useRef, useState } from "react";
import { useCodeDashboardPortal } from "@features/csm-custom-dashboards/api/useCodeDashboardPortal";
import { checkDashboardSource } from "@features/csm-custom-dashboards/components/code-dashboard-sandbox/checkDashboardSource";
import { FRAME_DOCUMENT } from "@features/csm-custom-dashboards/components/code-dashboard-sandbox/codeDashboardRunner";

interface FrameMessage {
  type?: string;
  id?: number;
  fn?: string;
  args?: unknown;
  message?: string;
}

/**
 * Runs a code dashboard's `render(portal, root)` in a Web Worker inside a
 * sandboxed iframe (`sandbox="allow-scripts"` only; see codeDashboardRunner.ts)
 * and answers its `portal.*` calls from this page. Only the names
 * `useCodeDashboardPortal` returns are answered (all with real data); any
 * other call is refused. Code that fails `checkDashboardSource` is not run.
 * An error thrown by the code, or why it was not run, is shown above the frame.
 *
 * Mount it with `key={source}` so a new source gets a fresh iframe and state.
 */
export default function CodeDashboardFrame({ source }: { source: string }): JSX.Element {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [error, setError] = useState<string | null>(null);
  const portal = useCodeDashboardPortal();

  useEffect(() => {
    const onMessage = (event: MessageEvent<FrameMessage>): void => {
      const frame = frameRef.current?.contentWindow;
      if (!frame || event.source !== frame) return; // only our own iframe
      const msg = event.data ?? {};
      if (msg.type === "ready") {
        const refused = checkDashboardSource(source);
        if (refused) setError(refused);
        else frame.postMessage({ type: "run", source }, "*");
      } else if (msg.type === "error") {
        setError(msg.message ?? "The dashboard code failed.");
      } else if (msg.type === "call" && typeof msg.fn === "string") {
        const handler = Object.hasOwn(portal, msg.fn) ? portal[msg.fn] : undefined;
        if (!handler) {
          frame.postMessage({ type: "result", id: msg.id, error: `portal.${msg.fn} is not available` }, "*");
          return;
        }
        Promise.resolve()
          .then(() => handler(msg.args))
          .then(
            (value) => frame.postMessage({ type: "result", id: msg.id, value }, "*"),
            (err: unknown) => frame.postMessage({ type: "result", id: msg.id, error: String(err) }, "*"),
          );
      }
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, [source, portal]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1, flex: 1, minHeight: 0 }}>
      {error && <Alert severity="error">The dashboard code failed: {error}</Alert>}
      <Box
        component="iframe"
        ref={frameRef}
        title="Generated dashboard"
        sandbox="allow-scripts"
        srcDoc={FRAME_DOCUMENT}
        sx={{ flex: 1, minHeight: 0, width: "100%", border: 0, borderRadius: 1, bgcolor: "#fff" }}
      />
    </Box>
  );
}
