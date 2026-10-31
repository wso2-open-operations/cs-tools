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

import { useAsgardeo } from "@asgardeo/react";
import { useEffect, useMemo, useState, type JSX, type MouseEvent } from "react";
import { useCsmPortalBanners, type PortalBannerView } from "@components/portal-banners/useCsmPortalBanners";
import { sanitizeAnnouncementHtml } from "@utils/sanitizeHtml";

const MAX_TIMEOUT_MS = 2 ** 31 - 1;

function dismissalKey(id: string): string {
  return `portal_banner_dismissed:${id}`;
}

function isDismissed(id: string): boolean {
  try {
    return localStorage.getItem(dismissalKey(id)) === "dismissed";
  } catch {
    return false;
  }
}

function persistDismissal(id: string): void {
  try {
    localStorage.setItem(dismissalKey(id), "dismissed");
  } catch {
    // ignore storage errors
  }
}

function parseTimestamp(value: string | null): number | null {
  if (!value) return null;
  const ms = Date.parse(value);
  return Number.isNaN(ms) ? null : ms;
}

function sanitizeBannerHtml(html: string): string {
  return sanitizeAnnouncementHtml(html);
}

// Saved HTML can carry a paragraph margin. The strip stays the same 3rem as an image banner.
const BANNER_STRIP_CSS = `
.portal-banner-strip{
  height:3rem !important;
  min-height:3rem !important;
  max-height:3rem !important;
  overflow:hidden !important;
  box-sizing:border-box !important;
}
.portal-banner-strip [data-banner="1"]{
  height:3rem !important;
  min-height:3rem !important;
  max-height:3rem !important;
  padding-top:0 !important;
  padding-bottom:0 !important;
  overflow:hidden !important;
  box-sizing:border-box !important;
}
.portal-banner-strip [data-banner-text] p,
.portal-banner-strip [data-banner-text] div{margin:0 !important}
`;

/** True while now is inside the banner's start and end. A missing bound is open. */
function useInWindow(startsAt: string | null, expiresAt: string | null): boolean {
  const startMs = useMemo(() => parseTimestamp(startsAt), [startsAt]);
  const expiryMs = useMemo(() => parseTimestamp(expiresAt), [expiresAt]);
  const [now, setNow] = useState(() => Date.now());
  const emptyWindow = startMs !== null && expiryMs !== null && startMs >= expiryMs;

  useEffect(() => {
    if (emptyWindow) return undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const schedule = (): void => {
      const current = Date.now();
      setNow((prev) =>
        [startMs, expiryMs].some((t) => t !== null && prev < t && t <= current) ? current : prev,
      );
      const upcoming = [startMs, expiryMs].filter((t): t is number => t !== null && t > current);
      if (upcoming.length === 0) return;
      timer = setTimeout(schedule, Math.min(Math.min(...upcoming) - current, MAX_TIMEOUT_MS));
    };
    schedule();
    return () => {
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [startMs, expiryMs, emptyWindow]);

  if (emptyWindow) return false;
  return (startMs === null || now >= startMs) && (expiryMs === null || now < expiryMs);
}

function BannerStrip({ banner }: { banner: PortalBannerView }): JSX.Element | null {
  const inWindow = useInWindow(banner.startsAt, banner.expiresAt);
  const [closed, setClosed] = useState(() => isDismissed(banner.id));
  const html = useMemo(() => sanitizeBannerHtml(banner.html), [banner.html]);

  if (!inWindow || closed || !html) return null;

  const handleClick = (event: MouseEvent<HTMLDivElement>): void => {
    if ((event.target as HTMLElement).closest("[data-close-banner]")) {
      persistDismissal(banner.id);
      setClosed(true);
    }
  };

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: close is handled by the button inside the HTML
    <div
      className="portal-banner-strip"
      onClick={handleClick}
      // biome-ignore lint/security/noDangerouslySetInnerHtml: HTML sanitized before insertion
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}

/** Renders banner HTML above the header. The caller supplies rows already meant for this portal. */
export function PortalBannerList({ banners }: { banners: PortalBannerView[] }): JSX.Element | null {
  if (banners.length === 0) return null;
  return (
    <>
      <style>{BANNER_STRIP_CSS}</style>
      {banners.map((banner) => (
        <BannerStrip key={banner.id} banner={banner} />
      ))}
    </>
  );
}

/** Published CSM banners, in stack order, above the header. */
export default function PortalBanners(): JSX.Element | null {
  const { isSignedIn, isLoading } = useAsgardeo();
  const query = useCsmPortalBanners(isSignedIn && !isLoading);
  if (!isSignedIn || isLoading || !query.data?.length) return null;
  return <PortalBannerList banners={query.data} />;
}
