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

import { Box, List, ListItem, ListItemButton, ListItemText, Typography } from "@wso2/oxygen-ui";
import type { JSX, MouseEvent } from "react";
import { prefersReducedMotion } from "@config/colorUtils";
import { useActiveHeading } from "@hooks/useActiveHeading";
import type { OutlineHeading } from "@utils/articleOutline";

interface TableOfContentsProps {
  headings: OutlineHeading[];
  /** Fewer headings than this and the outline is not worth the space. */
  minHeadings?: number;
}

/** "On this page" outline of an article, tracking the section being read. */
export default function TableOfContents({ headings, minHeadings = 2 }: TableOfContentsProps): JSX.Element | null {
  const active = useActiveHeading(headings.map((h) => h.id));
  if (headings.length < minHeadings) return null;

  const jumpTo = (event: MouseEvent<HTMLAnchorElement>, id: string): void => {
    const target = document.getElementById(id);
    if (!target) return; // let the browser handle the hash
    event.preventDefault();
    // Headings come from article HTML and are not focusable; make this one
    // programmatically focusable so the next Tab continues from the section
    // the reader chose, not from the outline.
    if (!target.hasAttribute("tabindex")) target.setAttribute("tabindex", "-1");
    target.scrollIntoView({ behavior: prefersReducedMotion() ? "auto" : "smooth", block: "start" });
    target.focus({ preventScroll: true });
    // Keep the router's own history state; only the fragment changes.
    window.history.replaceState(window.history.state, "", `#${id}`);
  };

  return (
    <Box component="nav" aria-labelledby="toc-heading" className="kb-no-print">
      <Typography
        id="toc-heading"
        variant="overline"
        component="h2"
        color="text.secondary"
        sx={{ display: "block", mb: 1, letterSpacing: 1 }}
      >
        On this page
      </Typography>
      <List component="ol" dense disablePadding sx={{ borderLeft: 1, borderColor: "divider" }}>
        {headings.map((heading) => {
          const isActive = heading.id === active;
          return (
            <ListItem key={heading.id} disablePadding sx={{ ml: "-1px" }}>
              <ListItemButton
                component="a"
                href={`#${heading.id}`}
                onClick={(event: MouseEvent<HTMLAnchorElement>) => jumpTo(event, heading.id)}
                aria-current={isActive ? "location" : undefined}
                sx={{
                  py: 0.5,
                  pl: heading.level === 3 ? 3 : 1.5,
                  pr: 1,
                  borderLeft: 2,
                  borderColor: isActive ? "primary.main" : "transparent",
                  borderRadius: 0,
                  "&:hover": { bgcolor: "transparent", "& .MuiListItemText-primary": { color: "text.primary" } },
                }}
              >
                <ListItemText
                  primary={heading.text}
                  slotProps={{
                    primary: {
                      variant: "body2",
                      sx: {
                        color: isActive ? "text.primary" : "text.secondary",
                        fontWeight: isActive ? 600 : 400,
                        lineHeight: 1.4,
                      },
                    },
                  }}
                />
              </ListItemButton>
            </ListItem>
          );
        })}
      </List>
    </Box>
  );
}
