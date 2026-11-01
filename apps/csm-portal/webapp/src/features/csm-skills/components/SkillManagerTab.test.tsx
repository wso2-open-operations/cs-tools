import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const api = { get: vi.fn(), put: vi.fn(), del: vi.fn(), patch: vi.fn(), post: vi.fn() };
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => api,
  BackendApiError: class BackendApiError extends Error {},
}));

import SkillManagerTab from "@features/csm-skills/components/SkillManagerTab";
import { categories, levelTypes, skillGo, skillKube } from "@features/csm-skills/test/fixtures";

const rowFor = (i: number) => ({
  userId: `00000000-0000-0000-0000-${String(i).padStart(12, "0")}`,
  userName: `Person ${i}`,
  userEmail: `person.${i}@example.com`,
  teamKeys: ["vega"],
  skillId: "skill-go",
  skillName: "Go",
  level: { id: "lvl-a-expert", name: "Expert", value: 100 },
  inherited: false,
  updatedOn: "2026-02-01T00:00:00Z",
});

function renderTab() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <SkillManagerTab />
    </QueryClientProvider>,
  );
}

const lastPeopleBody = () =>
  api.post.mock.calls.filter(([p]) => p === "/skills/users/search").map(([, b]) => b).at(-1);

beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset();
  api.get.mockImplementation(async (path: string) => {
    if (path === "/skills/levels") return { levelTypes };
    if (path === "/skills/categories") return { categories };
    if (path.startsWith("/team-schedule/catalogue")) {
      return {
        zones: [],
        shifts: [],
        absenceKinds: [],
        teams: [
          { key: "vega", name: "Vega", family: "CRE", sortOrder: 1, members: [] },
          { key: "apollo", name: "Apollo", family: "SRE", sortOrder: 2, members: [] },
        ],
      };
    }
    return null;
  });
  api.post.mockImplementation(async (path: string, body: { pagination?: { limit: number; offset: number } }) => {
    if (path === "/skills/search") return { skills: [skillGo, skillKube], total: 2, limit: 50, offset: 0 };
    if (path === "/skills/users/search") {
      const { limit = 25, offset = 0 } = body.pagination ?? {};
      return { items: [rowFor(offset + 1), rowFor(offset + 2)], total: 120, limit, offset };
    }
    return {};
  });
});

describe("SkillManagerTab", () => {
  it("loads the first page with only pagination and no filters", async () => {
    renderTab();
    expect(await screen.findByText("Person 1")).toBeInTheDocument();
    expect(lastPeopleBody()).toEqual({ pagination: { limit: 25, offset: 0 } });
    expect(screen.getAllByText("Vega").length).toBeGreaterThan(0);
  });

  it("pages on the server by sending the next offset", async () => {
    renderTab();
    await screen.findByText("Person 1");
    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() => expect(lastPeopleBody()).toEqual({ pagination: { limit: 25, offset: 25 } }));
    expect(await screen.findByText("Person 26")).toBeInTheDocument();
  });

  it("sends the name search to the server and goes back to the first page", async () => {
    renderTab();
    await screen.findByText("Person 1");
    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() => expect(lastPeopleBody()).toEqual({ pagination: { limit: 25, offset: 25 } }));
    fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "jane" } });
    await waitFor(() =>
      expect(lastPeopleBody()).toEqual({ searchQuery: "jane", pagination: { limit: 25, offset: 0 } }),
    );
  });

  it("sends the minimum level as a number", async () => {
    renderTab();
    await screen.findByText("Person 1");
    await waitFor(() => expect(within(screen.getByLabelText("Minimum level")).getAllByRole("option").length).toBeGreaterThan(1));
    fireEvent.change(screen.getByLabelText("Minimum level"), { target: { value: "50" } });
    await waitFor(() =>
      expect(lastPeopleBody()).toEqual({ minLevelValue: 50, pagination: { limit: 25, offset: 0 } }),
    );
  });

  it("sends picked skills and teams as id and key lists", async () => {
    renderTab();
    await screen.findByText("Person 1");

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Skills" }));
    fireEvent.click(await screen.findByRole("option", { name: "Go" }));
    await waitFor(() =>
      expect(lastPeopleBody()).toEqual({ skillIds: ["skill-go"], pagination: { limit: 25, offset: 0 } }),
    );

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Teams" }));
    fireEvent.click(await screen.findByRole("option", { name: /Apollo/ }));
    await waitFor(() =>
      expect(lastPeopleBody()).toEqual({
        skillIds: ["skill-go"],
        teamKeys: ["apollo"],
        pagination: { limit: 25, offset: 0 },
      }),
    );
  });

  it("shows an empty state when nobody matches", async () => {
    api.post.mockImplementation(async (path: string) =>
      path === "/skills/users/search" ? { items: [], total: 0, limit: 25, offset: 0 } : { skills: [], total: 0, limit: 50, offset: 0 },
    );
    renderTab();
    expect(await screen.findByText("No one matches these filters.")).toBeInTheDocument();
  });

  it("shows an error state when the search fails", async () => {
    api.post.mockImplementation(async (path: string) => {
      if (path === "/skills/users/search") throw new Error("search failed");
      return { skills: [], total: 0, limit: 50, offset: 0 };
    });
    renderTab();
    expect(await screen.findByText("search failed")).toBeInTheDocument();
  });
});
