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
  Button,
  IconButton,
  InputAdornment,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Briefcase,
  Building2,
  ChartColumn,
  Clock,
  Cog,
  Compass,
  Headset,
  LifeBuoy,
  Megaphone,
  RefreshCw,
  Search,
  Shield,
  UserCog,
  UsersRound,
  X,
} from "@wso2/oxygen-ui-icons-react";
import {
  type ComponentType,
  type JSX,
  type MouseEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { type CsmNavNode, navNodeById } from "@config/csmNavItems";
import { enabledNavChildren } from "@config/featureFlags";
import HelpTopicSection from "@features/help/components/HelpTopicSection";
import {
  findTopicMatch,
  HELP_TOPIC_PLAIN_TEXT,
  type TopicSearchMatch,
} from "@features/help/utils/helpSearch";

/** Bare topic id (e.g. `"operations"`) from a `help.<id>` nav node id — the
 * key `HELP_TOPIC_CONTENT` and this page's `#<topic>` hash share. */
function bareTopicId(nodeId: string): string {
  return nodeId.replace(/^help\./, "");
}

/**
 * Per-topic sidebar icon, matching the icon its own top-level nav section
 * already uses elsewhere in the app (`csmNavItems.ts`) where a topic maps
 * 1:1 onto one (e.g. `support` -> the same `Headset` the Support rail item
 * uses) — a topic with no such section (Overview, Navigation &
 * personalization, People & project access) gets a sensible standalone
 * pick instead. Falls back to Help's own icon for any future topic added
 * here without an entry, rather than rendering with no icon at all.
 *
 * `settings` deliberately does NOT reuse the real Settings section's own
 * `Settings` gear icon, unlike every other entry here — that gear renders
 * visually identical to `operations`'s `Cog` in this icon set, and with
 * every topic's icon sitting in one dense list (unlike the main sidebar,
 * where the two are far apart and each always has its own visible label),
 * two indistinguishable gears right next to each other defeats the point of
 * having icons at all. `UserCog` (this topic covers user/role/group/team
 * management) is both visually distinct and still thematically apt.
 */
const TOPIC_ICONS: Record<string, ComponentType<{ size?: number | string }>> = {
  overview: BookOpen,
  "workspace-basics": Compass,
  dashboard: ChartColumn,
  support: Headset,
  operations: Cog,
  engagements: Briefcase,
  "security-center": Shield,
  updates: RefreshCw,
  "time-cards": Clock,
  announcements: Megaphone,
  customers: Building2,
  "people-access": UsersRound,
  settings: UserCog,
};

/**
 * The topic id to show first: whatever `#<topic>` a direct/bookmarked link
 * (`/help#operations`) carries, as long as it's still an enabled topic,
 * falling back to the first topic in nav order otherwise (a bare `/help`, or
 * one pointing at a topic this deployment has hidden).
 */
function resolveInitialTopicId(topics: CsmNavNode[]): string {
  const fromHash = window.location.hash.slice(1);
  if (topics.some((topic) => bareTopicId(topic.id) === fromHash)) return fromHash;
  return topics.length > 0 ? bareTopicId(topics[0].id) : "";
}

/**
 * Prev/Next footer links read as plain text links rather than filled
 * buttons — MUI's default "text" `Button` hover (a solid rounded-rect
 * background) reads too heavy for a footer nav pair. Bolding on hover
 * keeps the "this is clickable" cue without the button chrome or an
 * underline; safe from layout jank here specifically because each button
 * sits at one end of a `justify-content: space-between` row, so the small
 * width change from bolding grows it away from its own anchored edge
 * rather than shifting anything else.
 */
const PREV_NEXT_LINK_SX = {
  "&:hover": { backgroundColor: "transparent", fontWeight: 700 },
} as const;

/**
 * The real scrolling element for `el` (AppLayout's main content area scrolls
 * itself via `overflow: auto` — the window/document never does, since the
 * app shell is pinned to `100dvh` with `overflow: hidden`), or the document's
 * own scrolling element if `el` isn't in a scrollable ancestor. Mirrors
 * `findVerticalScrollAncestor` in `csm-cases/utils/permalinkScroll.ts`
 * (kept local rather than shared since this page's need — "scroll back to
 * top" — is much narrower than that helper's fragment-highlight machinery).
 */
function findScrollAncestor(el: HTMLElement): HTMLElement {
  let cur: HTMLElement | null = el.parentElement;
  while (cur && cur !== document.body) {
    const overflowY = window.getComputedStyle(cur).overflowY;
    if ((overflowY === "auto" || overflowY === "scroll") && cur.scrollHeight > cur.clientHeight) {
      return cur;
    }
    cur = cur.parentElement;
  }
  return (document.scrollingElement as HTMLElement | null) ?? document.documentElement;
}

/**
 * Docs-style Help: a sticky left-hand list of every enabled topic and a
 * content pane showing exactly one topic at a time, switching instantly on
 * click (no route change — the topic id lives in the `#<topic>` hash, so a
 * link like `/help#operations` still lands on the right topic). Replaces the
 * earlier single-scroll layout (all 13 topics stacked on one page behind a
 * short table-of-contents) — reported live as reading like "a wall of text";
 * showing one topic at a time, plus Prev/Next links between them, keeps each
 * screenful focused on what the reader actually clicked for.
 *
 * Topics are still declared once, in `csmNavItems.ts`'s `help` node, and
 * filtered here to the ones this deployment has enabled via
 * `CSM_PORTAL_FEATURE_OVERRIDES` (`enabledNavChildren`), same as every other
 * section's tab strip.
 */
export default function HelpPage(): JSX.Element {
  const helpSection = navNodeById("help");
  const topics = useMemo(
    () => (helpSection ? enabledNavChildren(helpSection) : []),
    [helpSection],
  );

  const [activeTopicId, setActiveTopicId] = useState<string>(() => resolveInitialTopicId(topics));
  const contentRef = useRef<HTMLDivElement>(null);
  const isFirstRender = useRef(true);

  // Narrows the sidebar list only — the underlying topic order (and so
  // Prev/Next, and the "X of N" counter below) always stays the full,
  // unfiltered set, so clearing the search box never changes where those
  // point. Matches full topic content, not just the label — a search for
  // "incidents" (a tab inside Operations, not a topic of its own) would
  // otherwise return nothing even though Operations covers it in depth.
  // Each result carries `match`, which is only more than "matched the
  // title" when the hit came from content, so the row below can show a
  // snippet explaining why an otherwise-unrelated-looking topic matched.
  const [filterQuery, setFilterQuery] = useState("");
  const filteredResults = useMemo(() => {
    return topics
      .map((topic) => {
        const id = bareTopicId(topic.id);
        const match = findTopicMatch(topic.label, HELP_TOPIC_PLAIN_TEXT[id] ?? "", filterQuery);
        return match ? { topic, match } : null;
      })
      .filter((result): result is { topic: CsmNavNode; match: TopicSearchMatch } => result !== null);
  }, [topics, filterQuery]);

  // Keeps the shown topic in sync with the URL's own hash — a bookmarked or
  // shared `/help#<topic>` link, and browser back/forward between topics
  // already visited, both change the hash without going through this page's
  // own click handlers.
  useEffect(() => {
    const onHashChange = (): void => {
      const id = window.location.hash.slice(1);
      if (topics.some((topic) => bareTopicId(topic.id) === id)) setActiveTopicId(id);
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, [topics]);

  // The other direction: pushes `activeTopicId` out to the URL once React has
  // committed it, so a click only ever needs to update state, never the URL
  // directly. Guarded by the equality check so this doesn't fight the effect
  // above in a loop, and so a hash that already matches (the initial render,
  // or an update that *originated* from `onHashChange` above) doesn't add a
  // redundant history entry.
  //
  // A *correction* — the current hash isn't a known, enabled topic at all
  // (a bare `/help` load with an empty hash, or one naming a disabled/typo'd
  // topic that `resolveInitialTopicId`/`onHashChange` already fell back away
  // from) — uses `replaceState` instead of a real push: pushing here would
  // leave a phantom history entry between "no topic" and the resolved one,
  // so leaving the page needs two Back presses instead of one. A genuine
  // topic-to-topic move (the current hash *is* a known topic, just not this
  // one) still pushes, since that's exactly what makes Back/Forward step
  // between previously-visited topics.
  useEffect(() => {
    const currentHash = window.location.hash.slice(1);
    if (currentHash === activeTopicId) return;
    const isKnownTopic = topics.some((topic) => bareTopicId(topic.id) === currentHash);
    if (!isKnownTopic) {
      window.history.replaceState(null, "", `#${activeTopicId}`);
      return;
    }
    window.location.hash = activeTopicId;
  }, [activeTopicId, topics]);

  // Scrolls the content pane back to its own top on every topic switch after
  // the first render — otherwise a short topic opened while scrolled halfway
  // down a long one would render mostly blank above the fold.
  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }
    if (contentRef.current) {
      // Optional call: jsdom's `document.scrollingElement` (the fallback
      // target when `contentRef` isn't nested in a scrollable ancestor,
      // which is always true under jsdom's layout-less rendering) doesn't
      // implement `scrollTo` — real browsers always do, on every element.
      findScrollAncestor(contentRef.current).scrollTo?.({ top: 0, behavior: "smooth" });
    }
  }, [activeTopicId]);

  // Handles every in-page topic link (sidebar entries, Prev/Next). Must
  // `preventDefault` rather than letting the browser follow the anchor's own
  // `href`: React re-renders synchronously within this same click, which
  // updates that anchor's `href` to point at the *next* topic after this one
  // (Prev/Next always reflects the topic *after* whichever one becomes
  // active) — the browser's default fragment-navigation reads `href` fresh
  // right after this handler returns, so left alone it follows the
  // already-updated (one-topic-too-far) value instead of the one actually
  // clicked. Verified in a real browser: clicking Next landed two topics
  // ahead until this fix. The URL itself is updated separately, by the
  // `activeTopicId`-driven effect above, once React has committed this
  // click's state change — not here, directly.
  const navigateToTopic = (id: string, event: MouseEvent): void => {
    event.preventDefault();
    setActiveTopicId(id);
  };

  const activeIndex = topics.findIndex((topic) => bareTopicId(topic.id) === activeTopicId);
  const activeTopic = activeIndex >= 0 ? topics[activeIndex] : undefined;
  const prevTopic = activeIndex > 0 ? topics[activeIndex - 1] : undefined;
  const nextTopic =
    activeIndex >= 0 && activeIndex < topics.length - 1 ? topics[activeIndex + 1] : undefined;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Help</Typography>

      <Box sx={{ display: "flex", gap: 3, alignItems: "flex-start", minWidth: 0 }}>
        <Box
          component="nav"
          aria-label="Help topics"
          sx={{
            width: 260,
            flexShrink: 0,
            position: "sticky",
            top: 0,
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            overflow: "hidden",
          }}
        >
          <Box sx={{ p: 1, borderBottom: 1, borderColor: "divider" }}>
            <TextField
              fullWidth
              size="small"
              placeholder="Search topics…"
              value={filterQuery}
              onChange={(event) => setFilterQuery(event.target.value)}
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={16} />
                    </InputAdornment>
                  ),
                  endAdornment: filterQuery ? (
                    <InputAdornment position="end">
                      <IconButton
                        size="small"
                        edge="end"
                        onClick={() => setFilterQuery("")}
                        aria-label="Clear topic search"
                      >
                        <X size={16} />
                      </IconButton>
                    </InputAdornment>
                  ) : undefined,
                },
              }}
            />
          </Box>
          {filteredResults.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
              No topics match &ldquo;{filterQuery}&rdquo;.
            </Typography>
          ) : (
            <List dense disablePadding>
              {filteredResults.map(({ topic, match }) => {
                const id = bareTopicId(topic.id);
                const selected = id === activeTopicId;
                const Icon = TOPIC_ICONS[id] ?? LifeBuoy;
                return (
                  <ListItemButton
                    key={topic.id}
                    component="a"
                    href={`#${id}`}
                    selected={selected}
                    aria-current={selected ? "page" : undefined}
                    onClick={(event) => navigateToTopic(id, event)}
                    sx={{ alignItems: "flex-start" }}
                  >
                    <ListItemIcon sx={{ minWidth: 32, mt: "2px", color: selected ? "primary.main" : "text.secondary" }}>
                      <Icon size={18} />
                    </ListItemIcon>
                    <ListItemText
                      primary={topic.label}
                      secondary={
                        !match.matchedInTitle && match.snippet ? (
                          <Typography
                            component="span"
                            variant="caption"
                            color="text.secondary"
                            sx={{ display: "block", overflowWrap: "anywhere" }}
                          >
                            {match.snippet.before}
                            <Box component="mark" sx={{ bgcolor: "transparent", color: "text.primary", fontWeight: 700 }}>
                              {match.snippet.match}
                            </Box>
                            {match.snippet.after}
                          </Typography>
                        ) : undefined
                      }
                      slotProps={{ primary: { style: { fontWeight: selected ? 600 : 400 } } }}
                    />
                  </ListItemButton>
                );
              })}
            </List>
          )}
        </Box>

        <Box ref={contentRef} sx={{ flex: 1, minWidth: 0 }}>
          {activeTopic ? (
            <>
              <Typography variant="caption" color="text.secondary">
                Topic {activeIndex + 1} of {topics.length}
              </Typography>
              <Box component="section" id={activeTopicId}>
                <HelpTopicSection topicId={activeTopicId} />
              </Box>
              <Box
                sx={{
                  display: "flex",
                  justifyContent: "space-between",
                  mt: 4,
                  pt: 2,
                  borderTop: 1,
                  borderColor: "divider",
                }}
              >
                {prevTopic ? (
                  <Button
                    component="a"
                    href={`#${bareTopicId(prevTopic.id)}`}
                    onClick={(event) => navigateToTopic(bareTopicId(prevTopic.id), event)}
                    startIcon={<ArrowLeft size={16} />}
                    size="small"
                    aria-label={`Previous topic: ${prevTopic.label}`}
                    sx={PREV_NEXT_LINK_SX}
                  >
                    {prevTopic.label}
                  </Button>
                ) : (
                  <span />
                )}
                {nextTopic ? (
                  <Button
                    component="a"
                    href={`#${bareTopicId(nextTopic.id)}`}
                    onClick={(event) => navigateToTopic(bareTopicId(nextTopic.id), event)}
                    endIcon={<ArrowRight size={16} />}
                    size="small"
                    aria-label={`Next topic: ${nextTopic.label}`}
                    sx={PREV_NEXT_LINK_SX}
                  >
                    {nextTopic.label}
                  </Button>
                ) : (
                  <span />
                )}
              </Box>
            </>
          ) : (
            <Typography color="text.secondary">No help topics are available.</Typography>
          )}
        </Box>
      </Box>
    </Box>
  );
}
