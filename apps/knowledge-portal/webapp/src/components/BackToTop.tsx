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

import { Fab, Zoom } from "@wso2/oxygen-ui";
import { ArrowUp } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { prefersReducedMotion } from "@config/colorUtils";
import { useScrolledPast } from "@hooks/useScrolledPast";

interface BackToTopProps {
  /** Id of the main landmark; focus returns there so keyboard users land at the top too. */
  targetId: string;
  thresholdPx?: number;
}

/** Floating "back to top" control, shown once the page has been scrolled a screen or so. */
export default function BackToTop({ targetId, thresholdPx = 600 }: BackToTopProps): JSX.Element {
  const visible = useScrolledPast(thresholdPx);
  const onClick = (): void => {
    window.scrollTo({ top: 0, behavior: prefersReducedMotion() ? "auto" : "smooth" });
    document.getElementById(targetId)?.focus({ preventScroll: true });
  };
  return (
    <Zoom in={visible} unmountOnExit>
      <Fab
        className="kb-no-print"
        size="small"
        aria-label="Back to top"
        onClick={onClick}
        sx={{
          position: "fixed",
          right: { xs: 16, md: 28 },
          bottom: { xs: 16, md: 28 },
          zIndex: (theme) => theme.zIndex.speedDial,
          bgcolor: "background.paper",
          color: "text.primary",
          border: 1,
          borderColor: "divider",
          boxShadow: 3,
          "&:hover": { bgcolor: "background.paper", borderColor: "primary.main" },
        }}
      >
        <ArrowUp size={18} aria-hidden />
      </Fab>
    </Zoom>
  );
}
