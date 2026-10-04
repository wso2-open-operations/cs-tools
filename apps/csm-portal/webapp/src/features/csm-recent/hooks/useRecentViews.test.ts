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

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Controllable stand-in for the real (async) ID-token decode, so tests can
// drive exactly when the `userid` claim "resolves" relative to a record/read,
// instead of depending on real sign-in timing.
let mockUserid: string | undefined;
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => (mockUserid ? { userid: mockUserid } : undefined),
}));

import {
  clearRecentViews,
  renameRecentView,
  toggleRecentViewPin,
  useRecentViews,
  useRecordRecentView,
  type RecentView,
} from "./useRecentViews";

const entry = (id: string): Omit<RecentView, "visitedAt" | "pinned"> => ({
  kind: "case",
  id,
  title: `Case ${id}`,
  href: `/cases/${id}`,
});

beforeEach(() => {
  localStorage.clear();
  mockUserid = undefined;
});

describe("useRecentViews + useRecordRecentView", () => {
  it("starts empty", () => {
    const { result } = renderHook(() => useRecentViews());
    expect(result.current).toEqual([]);
  });

  it("drops persisted entries with an unknown kind", () => {
    // No identity has been synced in this test (`useSyncRecentViewsIdentity`
    // isn't mounted), so reads/writes fall back to the "pending" bucket.
    localStorage.setItem(
      "csm.recentViews.v1.pending",
      JSON.stringify([
        { kind: "case", id: "1", title: "Case 1", href: "/cases/1", visitedAt: "t" },
        { kind: "bogus", id: "2", title: "X", href: "/x", visitedAt: "t" },
      ]),
    );
    const { result } = renderHook(() => useRecentViews());
    expect(result.current.map((v) => v.id)).toEqual(["1"]);
  });

  it("records a visit and exposes it to readers in the same tab", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());

    act(() => recorder.result.current(entry("1")));

    expect(reader.result.current).toHaveLength(1);
    expect(reader.result.current[0].id).toBe("1");
    expect(reader.result.current[0].visitedAt).toBeTruthy();
  });

  it("strips HTML tags from title/subtitle/caseHit text before storing", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());

    act(() =>
      recorder.result.current({
        kind: "case",
        id: "1",
        title: "<b>Case</b> 1 <script>alert(1)</script>",
        subtitle: "Acme <i>Corp</i>",
        href: "/cases/1",
        caseHit: {
          subject: "<img src=x onerror=alert(1)>Urgent",
          severity: "S3",
          state: "open",
          assigneeName: "<b>Jane</b> Doe",
        },
      }),
    );

    const stored = reader.result.current[0];
    // DOMPurify drops a <script> element's content along with its tags,
    // not just the tag markup.
    expect(stored.title).toBe("Case 1 ");
    expect(stored.subtitle).toBe("Acme Corp");
    expect(stored.caseHit?.subject).toBe("Urgent");
    expect(stored.caseHit?.assigneeName).toBe("Jane Doe");
  });

  it("preserves plain text with angle brackets that aren't HTML tags", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());

    act(() =>
      recorder.result.current({
        ...entry("1"),
        title: "Error when x < y > z",
      }),
    );

    expect(reader.result.current[0].title).toBe("Error when x < y > z");
  });

  it("de-dupes by kind+id and bumps the entry to the top", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());

    act(() => recorder.result.current(entry("1")));
    act(() => recorder.result.current(entry("2")));
    act(() => recorder.result.current(entry("1")));

    expect(reader.result.current.map((v) => v.id)).toEqual(["1", "2"]);
  });

  it("caps the list at 12 entries, keeping the most recent", () => {
    const recorder = renderHook(() => useRecordRecentView());
    for (let i = 0; i < 15; i++) {
      act(() => recorder.result.current(entry(String(i))));
    }
    const reader = renderHook(() => useRecentViews());
    expect(reader.result.current).toHaveLength(12);
    // Most recent first: 14 down to 3.
    expect(reader.result.current[0].id).toBe("14");
    expect(reader.result.current.at(-1)?.id).toBe("3");
  });

  it("clearRecentViews empties the list", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());
    act(() => recorder.result.current(entry("1")));
    expect(reader.result.current).toHaveLength(1);

    act(() => clearRecentViews());
    expect(reader.result.current).toEqual([]);
  });

  describe("pinning", () => {
    it("toggles pinned state by kind+id", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));
      expect(reader.result.current[0].pinned).toBeFalsy();

      act(() => toggleRecentViewPin("case", "1"));
      expect(reader.result.current[0].pinned).toBe(true);

      act(() => toggleRecentViewPin("case", "1"));
      expect(reader.result.current[0].pinned).toBe(false);
    });

    it("never evicts pinned entries when the recency cap is exceeded", () => {
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("keep")));
      act(() => toggleRecentViewPin("case", "keep"));
      // Push well past the 12-entry unpinned cap.
      for (let i = 0; i < 15; i++) {
        act(() => recorder.result.current(entry(String(i))));
      }
      const reader = renderHook(() => useRecentViews());
      const ids = reader.result.current.map((v) => v.id);
      // The pinned entry survives; unpinned are still capped at 12.
      expect(ids).toContain("keep");
      expect(ids.filter((id) => id !== "keep")).toHaveLength(12);
    });

    it("preserves the pinned flag when an entry is re-visited", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));
      act(() => toggleRecentViewPin("case", "1"));
      expect(reader.result.current[0].pinned).toBe(true);

      act(() => recorder.result.current(entry("1")));
      expect(reader.result.current[0].pinned).toBe(true);
    });

    it("clearRecentViews keeps pinned entries", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("history")));
      act(() => recorder.result.current(entry("pinned")));
      act(() => toggleRecentViewPin("case", "pinned"));

      act(() => clearRecentViews());
      expect(reader.result.current.map((v) => v.id)).toEqual(["pinned"]);
    });
  });

  describe("renameRecentView", () => {
    it("updates the title of the matching kind+id entry", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));
      act(() => toggleRecentViewPin("case", "1"));

      act(() => renameRecentView("case", "1", "My renamed tab"));

      expect(reader.result.current[0].title).toBe("My renamed tab");
      // Pinned state survives a rename.
      expect(reader.result.current[0].pinned).toBe(true);
    });

    it("is a no-op for a blank/whitespace-only name", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));

      act(() => renameRecentView("case", "1", "   "));

      expect(reader.result.current[0].title).toBe("Case 1");
    });

    it("is a no-op for an untracked kind+id", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));

      act(() => renameRecentView("case", "does-not-exist", "New name"));

      expect(reader.result.current).toHaveLength(1);
      expect(reader.result.current[0].title).toBe("Case 1");
    });

    it("strips HTML tags from the new title before storing", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));

      act(() => renameRecentView("case", "1", "<b>Bold</b> name"));

      expect(reader.result.current[0].title).toBe("Bold name");
    });
  });

  describe("the incident/change_request/problem kinds", () => {
    it("records and reads back a visit for each new entity kind, keyed independently of case entries", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());

      act(() =>
        recorder.result.current({
          kind: "incident",
          id: "inc-1",
          title: "INC0001234 · Prod outage",
          href: "/operations/incidents/inc-1",
        }),
      );
      act(() =>
        recorder.result.current({
          kind: "change_request",
          id: "cr-1",
          title: "CHG0005 · Upgrade cluster",
          href: "/operations/change-requests/cr-1",
        }),
      );
      act(() =>
        recorder.result.current({
          kind: "problem",
          id: "prb-1",
          title: "PRB0009 · Recurring timeout",
          href: "/operations/problems/prb-1",
        }),
      );

      const kinds = reader.result.current.map((v) => v.kind);
      expect(kinds).toEqual(["problem", "change_request", "incident"]);
    });

    it("de-dupes by kind+id even when a case and an incident happen to share the same id", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());

      act(() => recorder.result.current(entry("shared-id")));
      act(() =>
        recorder.result.current({
          kind: "incident",
          id: "shared-id",
          title: "Incident with the same id",
          href: "/operations/incidents/shared-id",
        }),
      );

      expect(reader.result.current).toHaveLength(2);
      expect(reader.result.current.map((v) => v.kind).sort()).toEqual([
        "case",
        "incident",
      ]);
    });

    it("can be pinned like any other kind", () => {
      const reader = renderHook(() => useRecentViews());
      const recorder = renderHook(() => useRecordRecentView());
      act(() =>
        recorder.result.current({
          kind: "change_request",
          id: "cr-1",
          title: "CHG0005",
          href: "/operations/change-requests/cr-1",
        }),
      );

      act(() => toggleRecentViewPin("change_request", "cr-1"));
      expect(reader.result.current[0].pinned).toBe(true);
    });
  });

  describe("record-then-read within a single session (identity-resolution timing)", () => {
    it("a same-session record survives navigating away and shows up to an already-mounted reader as soon as identity settles — no reload, no extra write needed", () => {
      // Mirrors `RecentViewsButton`: mounted once (e.g. in the persistent
      // header) for the whole session, well before this test's "case page"
      // ever mounts.
      const reader = renderHook(() => useRecentViews());
      expect(reader.result.current).toEqual([]);

      // The case detail page mounts and records the visit while the ID-token
      // decode is still in flight for both instances (fresh tab / fresh
      // sign-in) — the write lands in the "pending" bucket.
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));

      // The user navigates away before identity ever resolved during this
      // visit — the case page (and its recorder hook) unmounts.
      recorder.unmount();

      // Identity resolves shortly after (e.g. the decode completes just
      // after navigation). The reader is the one whose own effect settles
      // it here — it must reflect the recorded visit immediately in the same
      // pass that resolves identity, not merely on the next unrelated event.
      mockUserid = "u1";
      act(() => reader.rerender());

      expect(reader.result.current.map((v) => v.id)).toEqual(["1"]);
    });

    it("opening the recent-views panel after navigating away (a fresh mount of the reader) sees a visit recorded and settled earlier in the same session", () => {
      // Identity already resolved earlier in the session (e.g. at app boot).
      mockUserid = "u1";
      const recorder = renderHook(() => useRecordRecentView());
      act(() => recorder.result.current(entry("1")));

      // Navigate away: the case page (recorder) unmounts.
      recorder.unmount();

      // The engineer opens the "Recently viewed" panel afterwards — a fresh
      // mount of the reader hook, not one that was live during the write.
      const reader = renderHook(() => useRecentViews());
      expect(reader.result.current.map((v) => v.id)).toEqual(["1"]);
    });

    it("a visit recorded by an instance whose own identity hasn't resolved yet still lands in the bucket an already-resolved reader is watching", () => {
      // Some other, already-mounted component (e.g. the header) resolved
      // identity earlier in the session.
      mockUserid = "u1";
      const reader = renderHook(() => useRecentViews());
      expect(reader.result.current).toEqual([]);

      // A brand-new recorder instance mounts whose own ID-token decode
      // hasn't completed yet (simulated by resetting the mock claim for its
      // render only) — the record call must still resolve to the same
      // already-known bucket the reader is watching, not a stale "pending"
      // one, because the write forces its own instance to a definitive key
      // once known, and otherwise fall back to whatever is already resolved.
      mockUserid = undefined;
      const recorder = renderHook(() => useRecordRecentView());
      mockUserid = "u1";
      act(() => recorder.result.current(entry("2")));

      expect(reader.result.current.map((v) => v.id)).toEqual(["2"]);
    });
  });
});
