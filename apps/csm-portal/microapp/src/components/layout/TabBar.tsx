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

import { useLayoutEffect, useRef } from "react";
import { Link, useLocation } from "react-router-dom";
import type { OxygenTheme } from "@wso2/oxygen-ui/styles/OxygenThemeBase";
import { BottomNavigation, BottomNavigationAction, Box, useTheme } from "@wso2/oxygen-ui";
import { Cog, Headset, House, LayoutGrid } from "@wso2/oxygen-ui-icons-react";

// Mirrors a subset of the webapp's CSM_NAV_ITEMS (apps/csm-portal/webapp/src/config/csmNavItems.ts):
// Home (Dashboard), Support (Cases), Operations get their own tab; Time Cards/Security
// Center/Updates/Engagements/Settings/Profile are grouped under More; Customers isn't in the
// mobile nav.
function activeTabFor(pathname: string): string | false {
  if (pathname === "/") return "home";
  if (pathname.startsWith("/support") || pathname.startsWith("/cases/")) return "support";
  if (pathname.startsWith("/operations")) return "operations";
  if (pathname.startsWith("/more") || pathname.startsWith("/profile")) return "more";
  return false;
}

export function TabBar() {
  const ref = useRef<HTMLDivElement>(null);
  const location = useLocation();
  const activeTab = activeTabFor(location.pathname);
  const theme = useTheme<OxygenTheme>();

  useLayoutEffect(() => {
    if (!ref.current) return;

    const observer = new ResizeObserver(([entry]) => {
      document.documentElement.style.setProperty("--tab-bar-height", `${entry.contentRect.height}px`);
    });

    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);

  return (
    <Box
      ref={ref}
      position="fixed"
      bgcolor={theme.vars.palette.background.default}
      bottom={0}
      left={0}
      right={0}
      pt={1}
      pb={4}
    >
      {/* MUI's BottomNavigation paints its own background.paper by default, which composites
          differently over dark-mode's near-transparent paper token than the solid
          background.default this bar's wrapping Box uses above — visible as a seam between the
          Box's padding and this component's own area. Transparent here so only the Box's single
          color shows. */}
      <BottomNavigation value={activeTab} showLabels sx={{ bgcolor: "transparent" }}>
        <BottomNavigationAction component={Link} to="/" value="home" label="Home" icon={<House />} disableRipple />
        <BottomNavigationAction
          component={Link}
          to="/support"
          value="support"
          label="Support"
          icon={<Headset />}
          disableRipple
        />
        <BottomNavigationAction
          component={Link}
          to="/operations"
          value="operations"
          label="Operations"
          icon={<Cog />}
          disableRipple
        />
        <BottomNavigationAction
          component={Link}
          to="/more"
          value="more"
          label="More"
          icon={<LayoutGrid />}
          disableRipple
        />
      </BottomNavigation>
    </Box>
  );
}
