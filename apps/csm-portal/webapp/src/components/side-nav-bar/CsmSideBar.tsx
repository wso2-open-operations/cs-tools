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

import { Box, Link, Sidebar, Tooltip, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Link as NavigateLink, useLocation } from "react-router";
import {
  CSM_NAV_ITEMS,
  isWipDisabled,
  navItemForPath,
} from "@config/csmNavItems";
import { preloadRoute } from "@utils/routePreloaders";

/** Tooltip for a disabled WIP item. Includes the label so the collapsed rail
 *  (which hides the label) still says which feature it is. */
const wipTooltip = (label: string): string =>
  `${label} — this section is still under construction`;

const COMPANY_NAME = "WSO2 LLC";
const TERMS_OF_SERVICE_URL = "https://wso2.com/terms-of-use/";
const PRIVACY_POLICY_URL = "https://wso2.com/privacy-policy/";

interface CsmSideBarProps {
  collapsed: boolean;
  expandedMenus?: Record<string, boolean>;
  onSelect?: (id: string) => void;
  onToggleExpand?: (id: string) => void;
}

function pickActiveId(pathname: string): string {
  if (pathname === "/" || pathname === "") return "dashboard";
  return navItemForPath(pathname)?.id ?? "dashboard";
}

export default function CsmSideBar({
  collapsed,
  expandedMenus,
  onSelect,
  onToggleExpand,
}: CsmSideBarProps): JSX.Element {
  const location = useLocation();
  const activeItem = pickActiveId(location.pathname);

  return (
    <Sidebar
      collapsed={collapsed}
      activeItem={activeItem}
      expandedMenus={expandedMenus}
      onSelect={onSelect}
      onToggleExpand={onToggleExpand}
    >
      <Sidebar.Nav>
        <Sidebar.Category>
          {CSM_NAV_ITEMS.map((item) => {
            const itemContent = (
              <Sidebar.Item id={item.id}>
                <Sidebar.ItemIcon>
                  <item.icon size={20} />
                </Sidebar.ItemIcon>
                {/* Plain string: Oxygen derives the collapsed-rail tooltip via
                    String(ItemLabel.children), so a wrapper element would render
                    as "[object Object]". */}
                <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
              </Sidebar.Item>
            );

            // WIP sections stay visible but disabled: no navigating Link, dimmed
            // and non-clickable (pointer events blocked on the inner box so no
            // click reaches Oxygen's select handler). The outer element is a
            // focusable div (tabIndex 0, aria-disabled) so keyboard users can
            // reach it and reveal the "work in progress" tooltip, which fires on
            // both hover and focus. Their routes render the coming-soon page
            // (see App.tsx's WipRouteGuard).
            if (isWipDisabled(item)) {
              return (
                <Tooltip
                  key={item.id}
                  title={wipTooltip(item.label)}
                  placement="right"
                >
                  <Box
                    aria-disabled
                    tabIndex={0}
                    sx={{ display: "block", cursor: "not-allowed" }}
                  >
                    <Box sx={{ opacity: 0.45, pointerEvents: "none" }}>
                      {itemContent}
                    </Box>
                  </Box>
                </Tooltip>
              );
            }

            return (
              <Link
                key={item.id}
                component={NavigateLink}
                to={item.path}
                color="inherit"
                underline="none"
                onMouseEnter={() => preloadRoute(item.path)}
              >
                {itemContent}
              </Link>
            );
          })}
        </Sidebar.Category>
      </Sidebar.Nav>

      {/* Legal footer lives at the bottom of the left rail so the main content
          area keeps its full height for meaningful work. Hidden when the rail
          is collapsed — the legal text won't fit the narrow rail. */}
      {!collapsed && (
        <Sidebar.Footer showDivider>
          <Box
            sx={{
              px: 2,
              py: 1.5,
              display: "flex",
              flexDirection: "column",
              gap: 0.5,
            }}
          >
            <Typography variant="caption" color="text.secondary">
              © {new Date().getFullYear()} {COMPANY_NAME}. All rights reserved.
            </Typography>
            <Box sx={{ display: "flex", flexWrap: "wrap", columnGap: 1.5, rowGap: 0.25 }}>
              <Link
                href={TERMS_OF_SERVICE_URL}
                target="_blank"
                rel="noopener noreferrer"
                variant="caption"
                color="text.secondary"
                underline="hover"
              >
                Terms & Conditions
              </Link>
              <Link
                href={PRIVACY_POLICY_URL}
                target="_blank"
                rel="noopener noreferrer"
                variant="caption"
                color="text.secondary"
                underline="hover"
              >
                Privacy Policy
              </Link>
            </Box>
          </Box>
        </Sidebar.Footer>
      )}
    </Sidebar>
  );
}
