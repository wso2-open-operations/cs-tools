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

import {
  Box,
  ColorSchemeToggle,
  Container,
  Header as HeaderUI,
  Link as OxygenLink,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useRef, useState, type JSX, type MouseEvent } from "react";
import { Link, Outlet } from "react-router";
import { accessibleLinkSx } from "@config/a11yThemeOverrides";
import { useRouteChangeReset } from "@hooks/useRouteChangeReset";

const BRAND_LOGO_HEIGHT = {
  xs: 18,
  sm: 20,
  md: 24,
  lg: 20,
  xl: 24,
} as const;

const MAIN_ID = "main-content";

const FOOTER_LINKS = [
  { href: "https://choreolabs.com/legal/", label: "Choreo Labs Legal" },
  {
    href: "https://choreolabs.com/california-privacy/#opt-out",
    label: "Do Not Sell My Personal Information",
  },
  { href: "https://choreolabs.com/modern-slavery-statement/", label: "Modern Slavery Statement" },
  { href: "https://choreolabs.com/contact/", label: "Report a Problem With This Page" },
] as const;

/**
 * Shell for every page of the public knowledge portal. The site is public:
 * no sign-in, no user menu. Just the brand, the page content and a footer.
 */
export default function KBLayout(): JSX.Element {
  const mainRef = useRef<HTMLElement>(null);
  useRouteChangeReset(mainRef);

  // Oxygen doesn't expose a themed logo asset yet, so the colour scheme is
  // read off the document attribute directly.
  const [isDarkMode, setIsDarkMode] = useState<boolean>(
    document.documentElement.getAttribute("data-color-scheme") === "dark",
  );

  useEffect(() => {
    const observer = new MutationObserver(() => {
      setIsDarkMode(
        document.documentElement.getAttribute("data-color-scheme") === "dark",
      );
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-color-scheme"],
    });
    return () => observer.disconnect();
  }, []);

  const logoSrc = isDarkMode ? "/logo-white.svg" : "/logo-dark.svg";

  // A plain "#main-content" link would add a hash to the URL; focusing the
  // landmark directly keeps the URL clean and works with the router.
  const skipToContent = (event: MouseEvent<HTMLAnchorElement>): void => {
    event.preventDefault();
    mainRef.current?.focus();
    mainRef.current?.scrollIntoView();
  };

  return (
    <Box sx={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <OxygenLink
        href={`#${MAIN_ID}`}
        onClick={skipToContent}
        sx={[
          {
            position: "absolute",
            left: 8,
            top: -64,
            zIndex: (theme) => theme.zIndex.tooltip + 1,
            px: 2,
            py: 1,
            borderRadius: 1,
            bgcolor: "background.paper",
            "&:focus": { top: 8 },
          },
          accessibleLinkSx,
        ]}
      >
        Skip to main content
      </OxygenLink>

      <HeaderUI sx={{ width: "100%", maxWidth: "100%" }}>
        <OxygenLink
          component={Link}
          to="/"
          underline="none"
          color="inherit"
          sx={{ display: "inline-flex", flexShrink: 0 }}
        >
          <HeaderUI.Brand sx={{ flexShrink: 0, gap: { xs: 0.75, sm: 1, md: 1.5 } }}>
            <HeaderUI.BrandLogo
              sx={{ display: "flex", alignItems: "center", flexShrink: 0 }}
            >
              <Box
                component="img"
                key={logoSrc}
                src={logoSrc}
                alt="WSO2"
                sx={{ height: BRAND_LOGO_HEIGHT, width: "auto", display: "block" }}
              />
            </HeaderUI.BrandLogo>
            <HeaderUI.BrandTitle
              sx={{
                whiteSpace: "nowrap",
                lineHeight: 1.2,
                flexShrink: 0,
                fontSize: { xs: "0.8125rem", sm: "0.875rem" },
              }}
            >
              Knowledge Base
            </HeaderUI.BrandTitle>
          </HeaderUI.Brand>
        </OxygenLink>
        <HeaderUI.Spacer />
        <ColorSchemeToggle />
      </HeaderUI>

      <Box
        component="main"
        id={MAIN_ID}
        ref={mainRef}
        tabIndex={-1}
        sx={{ flex: 1, "&:focus": { outline: "none" } }}
      >
        <Outlet />
      </Box>

      <Box
        component="footer"
        sx={{ borderTop: 2, borderColor: "primary.main", mt: 8, py: 4 }}
      >
        <Container maxWidth="lg">
          <Box
            component="nav"
            aria-label="Legal and support"
            sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", rowGap: 1, columnGap: { xs: 2.5, md: 4 } }}
          >
            <Typography variant="body2" color="text.secondary">
              ©{new Date().getFullYear()} Choreo Labs LLC
            </Typography>
            <Box
              component="ul"
              sx={{
                listStyle: "none",
                m: 0,
                p: 0,
                display: "flex",
                flexWrap: "wrap",
                rowGap: 1,
                columnGap: { xs: 2.5, md: 4 },
              }}
            >
              {FOOTER_LINKS.map((link) => (
                <li key={link.href}>
                  <OxygenLink
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    underline="hover"
                    color="text.primary"
                    variant="body2"
                  >
                    {link.label}
                  </OxygenLink>
                </li>
              ))}
            </Box>
          </Box>
        </Container>
      </Box>
    </Box>
  );
}
