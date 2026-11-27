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

import { Box, Button, Divider, IconButton } from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useHost } from "@context/host/HostContext";

/**
 * The way back to the application the portal is loaded inside, with a button
 * beside it that opens that application's launcher when it offers one. Renders
 * nothing when the portal runs on its own. Lives at the start of the header
 * rather than in the sidebar so it is still there on the screens that hide the
 * sidebar.
 */
export default function BackToHost(): JSX.Element | null {
  const host = useHost();
  if (!host) return null;
  const toggleAppSwitcher = host.onToggleAppSwitcher;

  return (
    <>
      <Box
        role="group"
        aria-label={host.name}
        sx={{
          display: "inline-flex",
          alignItems: "center",
          flexShrink: 0,
          bgcolor: "action.hover",
          border: 1,
          borderColor: "divider",
          borderRadius: 2,
          overflow: "hidden",
        }}
      >
        <Button
          color="inherit"
          aria-label={`Back to ${host.name}`}
          startIcon={<ArrowLeft size={16} />}
          onClick={host.onExit}
          sx={{ whiteSpace: "nowrap", borderRadius: 0, textTransform: "none", px: 1.5 }}
        >
          {host.name}
        </Button>
        {toggleAppSwitcher && (
          <>
            <Divider orientation="vertical" flexItem sx={{ my: 0.75 }} />
            <IconButton
              color="inherit"
              aria-label="Switch applications"
              aria-haspopup="dialog"
              aria-expanded={host.appSwitcherOpen ?? false}
              onClick={(event) => toggleAppSwitcher(event.currentTarget)}
              sx={{ borderRadius: 0, minWidth: 36 }}
            >
              <ChevronDown size={18} />
            </IconButton>
          </>
        )}
      </Box>
      <Divider orientation="vertical" sx={{ height: 28, mx: 1.5 }} />
    </>
  );
}
