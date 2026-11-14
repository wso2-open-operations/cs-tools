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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx). UserRefLink
// now resolves an unknown id through `useResolvedUserId`, which calls
// `POST /users/search` via this client.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import UserRefLink from "@components/UserRefLink";

const JANE_UUID = "00000000-0000-0000-0000-000000000000";
const JOHN_UUID = "11111111-1111-1111-1111-111111111111";

function renderWithProviders(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  postMock.mockReset();
  postMock.mockResolvedValue({ users: [] });
});

describe("UserRefLink", () => {
  it("never shows the circled W after a name", () => {
    renderWithProviders(
      <UserRefLink name={"Jane Doe \u24E6"} userId={JANE_UUID} />,
    );
    expect(screen.getByRole("link", { name: "Jane Doe" })).toBeInTheDocument();
    expect(screen.queryByText(/\u24E6/u)).not.toBeInTheDocument();
  });

  it("shows the support engineer icon only for an internal user", () => {
    const { unmount } = renderWithProviders(
      <UserRefLink name="Jane Doe" userId={JANE_UUID} isInternal />,
    );
    expect(screen.getByRole("img", { name: "Support engineer" })).toBeInTheDocument();
    unmount();
    renderWithProviders(
      <UserRefLink name={"John Roe \u24E6"} userId={JOHN_UUID} />,
    );
    expect(screen.queryByRole("img", { name: "Support engineer" })).not.toBeInTheDocument();
  });

  it("shows the icon beside a name with no link to follow", () => {
    renderWithProviders(<UserRefLink name="Jane Doe" isInternal />);
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Support engineer" })).toBeInTheDocument();
  });

  it("clears a hover underline after the hover behavior is disabled", () => {
    const view = renderWithProviders(
      <UserRefLink name="Jane Doe" userId={JANE_UUID} underlineOnHover />,
    );
    const link = screen.getByRole("link", { name: "Jane Doe" });
    fireEvent.mouseEnter(link);
    expect(link).toHaveStyle({ textDecoration: "underline" });

    view.rerender(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <UserRefLink name="Jane Doe" userId={JANE_UUID} underlineOnHover={false} />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.mouseLeave(screen.getByRole("link", { name: "Jane Doe" }));
    expect(screen.getByRole("link", { name: "Jane Doe" })).toHaveStyle({
      textDecoration: "none",
    });
  });

  it("renders a link to /people/<id> when an id is already known", async () => {
    renderWithProviders(
      <UserRefLink name="Jane Doe" email="jane.doe@example.com" userId={JANE_UUID} />,
    );
    const link = screen.getByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute("href", `/people/${JANE_UUID}`);
    // A known id short-circuits — no resolution lookup is ever made.
    await new Promise((r) => setTimeout(r, 20));
    expect(postMock).not.toHaveBeenCalled();
  });

  it("resolves an id from email and renders a link once resolution completes, when id is null", async () => {
    postMock.mockResolvedValue({
      users: [{ id: JANE_UUID, email: "jane.doe@example.com" }],
    });
    renderWithProviders(
      <UserRefLink name="Jane Doe" email="jane.doe@example.com" userId={null} />,
    );
    // No link yet — rendering is never blocked on the lookup.
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByRole("link", { name: "Jane Doe" })).toHaveAttribute(
        "href",
        `/people/${JANE_UUID}`,
      );
    });
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("renders plain text for a non-email author (e.g. 'system') and never makes a request", async () => {
    renderWithProviders(<UserRefLink name="System" email="system" userId={null} />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("System")).toBeInTheDocument();

    // Give any (incorrect) async resolution a chance to fire before asserting.
    await new Promise((r) => setTimeout(r, 20));
    expect(postMock).not.toHaveBeenCalled();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("stays plain text when the email doesn't resolve to any user (negative result)", async () => {
    postMock.mockResolvedValue({ users: [] });
    renderWithProviders(
      <UserRefLink name="Jane Doe" email="jane.doe@example.com" userId={null} />,
    );

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(1));
    // Still plain text — the empty result is a confirmed negative, not a
    // pending state.
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("falls back to today's behaviour when the backend sends no UserReference at all (userId prop omitted)", async () => {
    postMock.mockResolvedValue({
      users: [{ id: JANE_UUID, email: "jane.doe@example.com" }],
    });
    // Old-backend shape: only name/email are known, `userId` isn't passed at all.
    renderWithProviders(<UserRefLink name="Jane Doe" email="jane.doe@example.com" />);

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("link", { name: "Jane Doe" })).toHaveAttribute(
        "href",
        `/people/${JANE_UUID}`,
      );
    });
  });

  it("renders plain text (no link) when there's no email and no id", () => {
    renderWithProviders(<UserRefLink name="Jane Doe" />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("renders plain text when the email is an empty/whitespace string", () => {
    renderWithProviders(<UserRefLink name="Jane Doe" email="   " />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("batches several distinct actors' email lookups into a single request", async () => {
    postMock.mockResolvedValue({
      users: [
        { id: JANE_UUID, email: "jane.doe@example.com" },
        { id: JOHN_UUID, email: "john.smith@example.com" },
      ],
    });
    renderWithProviders(
      <>
        <UserRefLink name="Jane Doe" email="jane.doe@example.com" userId={null} />
        <UserRefLink name="John Smith" email="john.smith@example.com" userId={null} />
        {/* Same email as the first link — must dedupe, not add a third request. */}
        <UserRefLink name="Jane D." email="jane.doe@example.com" userId={null} />
      </>,
    );

    await waitFor(() => {
      expect(screen.getByRole("link", { name: "Jane Doe" })).toHaveAttribute(
        "href",
        `/people/${JANE_UUID}`,
      );
      expect(screen.getByRole("link", { name: "John Smith" })).toHaveAttribute(
        "href",
        `/people/${JOHN_UUID}`,
      );
      expect(screen.getByRole("link", { name: "Jane D." })).toHaveAttribute(
        "href",
        `/people/${JANE_UUID}`,
      );
    });

    // Three actors (two distinct emails), one network call.
    expect(postMock).toHaveBeenCalledTimes(1);
    const [, body] = postMock.mock.calls[0];
    expect((body as { filters: { emails: string[] } }).filters.emails.sort()).toEqual(
      ["jane.doe@example.com", "john.smith@example.com"].sort(),
    );
  });
});
