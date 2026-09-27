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

// Ported verbatim from apps/support-portal-lite/webapp's own
// features/spl/cases/utils/useInlineAttachmentImages.ts.
import { useEffect, useState } from "react";
import { useAttachmentDownload } from "../api/useAttachmentDownload";
import { extractInlineAttachmentIds, replaceInlineImageSources } from "./inlineImages";

// Shared across every comment/re-render so the same inline image is fetched at most
// once, even across "Show More" pagination re-fetches of the comment list.
const dataUrlCache = new Map<string, string | null>();
const inFlightRequests = new Map<string, Promise<string | null>>();

// Resolves ServiceNow inline-comment images (rendered as <img src=".../<id>.iix">) into
// data: URLs by fetching each referenced attachment through the existing
// /spl/attachments/{id}/download endpoint.
export function useInlineAttachmentImages(html: string, isUserAllowedtoDownloadAttachments: boolean) {
  const { fetchAttachmentDataUrl } = useAttachmentDownload();
  const [, setVersion] = useState(0);
  const [loading, setLoading] = useState(false);

  const ids = extractInlineAttachmentIds(html);
  const idsKey = ids.join(",");

  useEffect(() => {
    if (ids.length === 0 || !isUserAllowedtoDownloadAttachments) return;

    const idsToFetch = ids.filter((id) => !dataUrlCache.has(id));
    if (idsToFetch.length === 0) return;

    let cancelled = false;
    setLoading(true);

    const fetchOne = (id: string): Promise<string | null> => {
      const existing = inFlightRequests.get(id);
      if (existing) return existing;
      const promise = fetchAttachmentDataUrl(id).catch((error: unknown) => {
        console.error(`Failed to load inline attachment ${id}`, error);
        return null;
      });
      inFlightRequests.set(id, promise);
      return promise;
    };

    Promise.all(
      idsToFetch.map(async (id) => {
        const dataUrl = await fetchOne(id);
        dataUrlCache.set(id, dataUrl);
        inFlightRequests.delete(id);
      }),
    ).then(() => {
      if (!cancelled) {
        setLoading(false);
        setVersion((v) => v + 1);
      }
    });

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [idsKey, isUserAllowedtoDownloadAttachments]);

  if (ids.length === 0) {
    return { resolvedHtml: html, loading: false };
  }

  // Only carry over ids whose fetch has actually settled - an id missing from
  // this map is still in flight and should render as a loading skeleton, not
  // be conflated with a settled `null` (failed) result.
  const resolvedMap = new Map<string, string | null>();
  ids.forEach((id) => {
    if (dataUrlCache.has(id)) resolvedMap.set(id, dataUrlCache.get(id)!);
  });

  return {
    resolvedHtml: replaceInlineImageSources(html, resolvedMap, !isUserAllowedtoDownloadAttachments),
    loading,
  };
}
