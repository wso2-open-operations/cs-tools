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

import { Button } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useHost } from "@context/host/HostContext";

/**
 * The way back to the application the portal is loaded inside. Renders nothing
 * when the portal runs on its own. Lives in the header rather than the sidebar
 * so it is still there on the screens that hide the sidebar.
 */
export default function BackToHost(): JSX.Element | null {
  const host = useHost();
  if (!host) return null;
  return (
    <Button
      size="small"
      variant="text"
      color="inherit"
      startIcon={<ArrowLeft size={16} />}
      onClick={host.onExit}
      sx={{ flexShrink: 0, whiteSpace: "nowrap", mr: 1 }}
    >
      Back to {host.name}
    </Button>
  );
}
