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
  GlobalStyles,
  Header as HeaderUI,
  Link as OxygenLink,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useRef, useState, type JSX, type MouseEvent } from "react";
import { Link, Outlet, useLocation, useNavigate } from "react-router";
import BackToTop from "@components/BackToTop";
import { accessibleLinkSx } from "@config/a11yThemeOverrides";
import { opaquePaper } from "@config/colorUtils";
import SearchBox, { SEARCH_INPUT_ATTR } from "@features/kb/components/SearchBox";
import { useRouteChangeReset } from "@hooks/useRouteChangeReset";
import { browseUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";

const BRAND_LOGO_HEIGHT = {
  xs: 18,
  sm: 20,
  md: 24,
  lg: 20,
  xl: 24,
} as const;

export const MAIN_ID = "main-content";

const FOOTER_LINKS = [
  { href: "https://wso2.com/legal/", label: "WSO2 Legal" },
  {
    href: "https://wso2.com/privacy-policy/do-not-sell-my-personal-information/",
    label: "Do Not Sell My Personal Information",
  },
  { href: "https://wso2.com/modern-slavery-statement/", label: "Modern Slavery Statement" },
  { href: "https://wso2.com/contact/", label: "Report a Problem With This Page" },
] as const;

/** Pages with their own prominent search box; the header search is hidden there. */
const hasOwnSearch = (pathname: string): boolean =>
  pathname === "/" || pathname === "/knowledge-bases" || pathname.startsWith("/knowledge-bases/");

const isEditable = (target: EventTarget | null): boolean => {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName);
};

/**
 * Shell for every page of the public knowledge portal. The site is public:
 * no sign-in, no user menu, no navigation beyond the brand. A sticky header
 * with the brand and a search box on pages that lack their own, the page
 * content and a footer.
 */
export default function KBLayout(): JSX.Element {
  const mainRef = useRef<HTMLElement>(null);
  useRouteChangeReset(mainRef);
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const showHeaderSearch = !hasOwnSearch(pathname);

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

  // "/" focuses the first search box on the page, as on most documentation sites.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key !== "/" || event.metaKey || event.ctrlKey || event.altKey || isEditable(event.target)) return;
      const input = document.querySelector<HTMLInputElement>(`input[${SEARCH_INPUT_ATTR}]`);
      if (!input) return;
      event.preventDefault();
      input.focus();
      input.select();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
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
      <GlobalStyles
        styles={{
          // No global smooth scrolling: useRouteChangeReset must land at the top
          // instantly on navigation. The outline and Back to top ask for smooth
          // scrolling explicitly where it helps.
          "@media print": {
            ".kb-no-print": { display: "none !important" },
            body: { background: "#fff" },
          },
        }}
      />
      <OxygenLink
        href={`#${MAIN_ID}`}
        onClick={skipToContent}
        sx={[
          (theme) => ({
            position: "absolute",
            left: 8,
            top: -64,
            zIndex: theme.zIndex.tooltip + 1,
            px: 2,
            py: 1,
            borderRadius: 1,
            border: 1,
            borderColor: "divider",
            boxShadow: 3,
            fontWeight: 600,
            bgcolor: opaquePaper(theme, 1),
            "&:focus": { top: 8 },
          }),
          accessibleLinkSx,
        ]}
      >
        Skip to main content
      </OxygenLink>

      <Box
        component="div"
        className="kb-no-print"
        sx={{
          position: "sticky",
          top: 0,
          zIndex: (theme) => theme.zIndex.appBar,
          bgcolor: "background.paper",
          borderBottom: 1,
          borderColor: "divider",
        }}
      >
        <HeaderUI sx={{ width: "100%", maxWidth: "100%", bgcolor: "transparent", borderBottom: 0 }}>
          <OxygenLink
            component={Link}
            to="/"
            underline="none"
            color="inherit"
            aria-label="WSO2 Knowledge Base home"
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
          {showHeaderSearch && (
            <Box sx={{ width: { xs: 160, sm: 280, md: 360 }, mr: { xs: 1, sm: 2 } }}>
              <SearchBox
                key={pathname}
                label="Search articles"
                placeholder="Search articles"
                onSearch={(q) => navigate(browseUrl({ q: normalizeSearchQuery(q) }))}
              />
            </Box>
          )}
          <ColorSchemeToggle />
        </HeaderUI>
      </Box>

      <Box
        component="main"
        id={MAIN_ID}
        ref={mainRef}
        tabIndex={-1}
        sx={{ flex: 1, "&:focus": { outline: "none" } }}
      >
        <Outlet />
      </Box>

      <BackToTop targetId={MAIN_ID} />

      <Box
        component="footer"
        className="kb-no-print"
        sx={{ borderTop: 1, borderColor: "divider", mt: 10, py: 5, bgcolor: "background.paper" }}
      >
        <Container maxWidth="lg">
          <Stack
            direction={{ xs: "column", md: "row" }}
            spacing={{ xs: 3, md: 6 }}
            justifyContent="space-between"
            alignItems={{ xs: "flex-start", md: "center" }}
          >
            <Stack spacing={0.5} sx={{ minWidth: 0 }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
                WSO2 Knowledge Base
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Troubleshooting guides, how-tos and answers for WSO2 products.
              </Typography>
              <Typography variant="caption" color="text.secondary" sx={{ pt: 1 }}>
                ©{new Date().getFullYear()} WSO2 LLC
              </Typography>
            </Stack>
            <Box
              component="nav"
              aria-label="Legal and support"
              sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", rowGap: 1, columnGap: { xs: 2.5, md: 4 } }}
            >
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
                      color="text.secondary"
                      variant="body2"
                    >
                      {link.label}
                    </OxygenLink>
                  </li>
                ))}
              </Box>
            </Box>
          </Stack>
        </Container>
      </Box>
    </Box>
  );
}
