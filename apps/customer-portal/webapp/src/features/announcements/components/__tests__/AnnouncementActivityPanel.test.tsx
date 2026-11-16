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

import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AnnouncementActivityPanel from "@features/announcements/components/AnnouncementActivityPanel";

const mockUseComments = vi.hoisted(() => vi.fn());

vi.mock("@features/support/api/useGetCaseCommentsInfinite", () => ({
  default: mockUseComments,
}));

vi.mock("@utils/useDarkMode", () => ({
  useDarkMode: () => false,
}));

const DESCRIPTION = "<p>Scheduled downtime on Friday.</p><p>No action needed.</p>";

function comment(id: string, content: string, createdOn: string) {
  return {
    id,
    content,
    createdOn,
    createdBy: "creator@wso2.com",
    createdByFullName: "Announcement Creator",
    type: "comments",
    isEscalated: false,
  };
}

function givenComments(comments: ReturnType<typeof comment>[], hasNextPage = false) {
  mockUseComments.mockReturnValue({
    data: { pages: [{ comments, totalRecords: comments.length, offset: 0, limit: 10 }] },
    isLoading: false,
    isError: false,
    error: null,
    hasNextPage,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  });
}

describe("AnnouncementActivityPanel", () => {
  beforeEach(() => {
    mockUseComments.mockReset();
  });

  it("leaves out the origin comment that only repeats the description", () => {
    givenComments([comment("c1", DESCRIPTION, "2026-10-12T10:16:00Z")]);
    const { container } = render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.queryByText(/Scheduled downtime on Friday/)).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
  });

  it("leaves out an origin comment written with the Description label, keeping the updates above it", () => {
    givenComments([
      comment("c3", "<p>Update: postponed to October 11.</p>", "2026-10-13T09:00:00Z"),
      comment("c2", "<p>Reminder: maintenance is this weekend.</p>", "2026-10-12T12:00:00Z"),
      comment(
        "c1",
        `<b><u>Description</u></b><br><p></p>${DESCRIPTION}`,
        "2026-10-12T10:16:00Z",
      ),
    ]);
    render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.getByText(/postponed to October 11/)).toBeInTheDocument();
    expect(screen.getByText(/maintenance is this weekend/)).toBeInTheDocument();
    expect(screen.queryByText(/Scheduled downtime on Friday/)).not.toBeInTheDocument();
  });

  it("keeps later update comments and drops only the echoing origin one", () => {
    givenComments([
      comment("c2", "<p>Update: the window moved to Saturday.</p>", "2026-10-13T09:00:00Z"),
      comment("c1", DESCRIPTION, "2026-10-12T10:16:00Z"),
    ]);
    render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.getByText(/window moved to Saturday/)).toBeInTheDocument();
    expect(screen.queryByText(/Scheduled downtime on Friday/)).not.toBeInTheDocument();
  });

  it("keeps an oldest comment that quotes the description and adds its own text", () => {
    givenComments([
      comment(
        "c1",
        `<p>Correction:</p>${DESCRIPTION}<p>The window has moved to Saturday.</p>`,
        "2026-10-12T10:16:00Z",
      ),
    ]);
    render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.getByText(/moved to Saturday/)).toBeInTheDocument();
  });

  it("keeps the oldest comment when it differs from the description", () => {
    givenComments([comment("c1", "<p>Something else entirely.</p>", "2026-10-12T10:16:00Z")]);
    render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.getByText(/Something else entirely/)).toBeInTheDocument();
  });

  it("keeps the comment when the description is blank, since it may be the only copy", () => {
    givenComments([comment("c1", DESCRIPTION, "2026-10-12T10:16:00Z")]);
    render(<AnnouncementActivityPanel projectId="p1" caseId="a1" description="<p> </p>" />);
    expect(screen.getByText(/Scheduled downtime on Friday/)).toBeInTheDocument();
  });

  it("keeps the oldest comment when its timestamp ties with the next one, since the sort cannot tell which is the origin", () => {
    givenComments([
      comment("c2", "<p>Update: postponed to October 11.</p>", "2026-10-12T10:16:00Z"),
      comment("c1", DESCRIPTION, "2026-10-12T10:16:00Z"),
    ]);
    render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.getByText(/Scheduled downtime on Friday/)).toBeInTheDocument();
    expect(screen.getByText(/postponed to October 11/)).toBeInTheDocument();
  });

  it("keeps the oldest comment when the next one's timestamp cannot be parsed, since the order is then unknown", () => {
    // An unparsable timestamp sorts as the newest, so it is the next-oldest here.
    givenComments([
      comment("c2", "<p>Update: postponed to October 11.</p>", "not a date"),
      comment("c1", DESCRIPTION, "2026-10-12T10:16:00Z"),
    ]);
    render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.getByText(/Scheduled downtime on Friday/)).toBeInTheDocument();
    expect(screen.getByText(/postponed to October 11/)).toBeInTheDocument();
  });

  it("keeps every loaded comment while older pages remain, since the origin is not loaded yet", () => {
    // Without the last page the oldest loaded comment is not known to be the
    // origin, so even a lookalike must stay.
    givenComments([comment("c2", DESCRIPTION, "2026-10-13T09:00:00Z")], true);
    render(
      <AnnouncementActivityPanel projectId="p1" caseId="a1" description={DESCRIPTION} />,
    );
    expect(screen.getByText(/Scheduled downtime on Friday/)).toBeInTheDocument();
  });

  it("shows every comment when no description is passed", () => {
    givenComments([comment("c1", DESCRIPTION, "2026-10-12T10:16:00Z")]);
    render(<AnnouncementActivityPanel projectId="p1" caseId="a1" />);
    expect(screen.getByText(/Scheduled downtime on Friday/)).toBeInTheDocument();
  });

  describe("nested list wrapper", () => {
    const NESTED =
      '<ul><li value="1">Parent</li><li value="2" class="editor-list-item-nested"><ul><li value="1">Child</li></ul></li></ul>';

    it("hides the marker of the nested-list wrapper li but keeps normal bullets", () => {
      givenComments([comment("c1", NESTED, "2026-10-12T10:16:00Z")]);
      const { container } = render(
        <AnnouncementActivityPanel projectId="p1" caseId="a1" description="<p>other</p>" />,
      );
      const wrapper = container.querySelector("li.editor-list-item-nested");
      const normal = container.querySelector("li:not(.editor-list-item-nested)");
      expect(wrapper).not.toBeNull();
      expect(normal).not.toBeNull();
      expect(getComputedStyle(wrapper as Element).listStyleType).toBe("none");
      expect(getComputedStyle(normal as Element).listStyleType).not.toBe("none");
    });
  });
});
