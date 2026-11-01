import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const api = { get: vi.fn(), put: vi.fn(), del: vi.fn(), patch: vi.fn(), post: vi.fn() };
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => api,
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

let isAdmin = false;
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({ isAdmin }),
}));

import CsmSkillsPage from "@features/csm-skills/pages/CsmSkillsPage";
import { categories, levelTypes, mineGo, skillGo, skillOld } from "@features/csm-skills/test/fixtures";

/** A 403 as the real client throws it: an error carrying the HTTP status. */
const forbidden = Object.assign(new Error("not a skill manager"), { status: 403 });

function renderPage(initial = "/skills") {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initial]}>
        <CsmSkillsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function route({ manager }: { manager: boolean }) {
  api.get.mockImplementation(async (path: string) => {
    if (path === "/skills/me") return { userId: "u1", skills: [mineGo] };
    if (path === "/skills/levels") return { levelTypes };
    if (path === "/skills/categories") return { categories };
    if (path.startsWith("/team-schedule/catalogue")) return { zones: [], shifts: [], absenceKinds: [], teams: [] };
    return null;
  });
  api.post.mockImplementation(async (path: string) => {
    if (path === "/skills/users/search") {
      if (!manager) throw forbidden;
      return { items: [], total: 0, limit: 1, offset: 0 };
    }
    if (path === "/skills/search") return { skills: [skillGo, skillOld], total: 2, limit: 10, offset: 0 };
    return {};
  });
}

beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset();
  isAdmin = false;
});

describe("CsmSkillsPage tabs", () => {
  it("probes the manager search once with a minimal body and hides the Manager tab on 403", async () => {
    route({ manager: false });
    renderPage();
    expect(await screen.findByRole("tab", { name: "My Skills" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "All Skills" })).toBeInTheDocument();
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/users/search", { pagination: { limit: 1 } }),
    );
    expect(screen.queryByRole("tab", { name: "Skill Manager" })).not.toBeInTheDocument();
    const probes = api.post.mock.calls.filter(([p]) => p === "/skills/users/search");
    expect(probes).toHaveLength(1);
  });

  it("shows the Manager tab when the probe succeeds", async () => {
    route({ manager: true });
    renderPage();
    expect(await screen.findByRole("tab", { name: "Skill Manager" })).toBeInTheDocument();
  });

  it("falls back to My Skills for a deep link to the Manager tab when the caller is not a manager", async () => {
    route({ manager: false });
    renderPage("/skills?tab=manager");
    expect(await screen.findByRole("table", { name: "My skills" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Skill Manager" })).not.toBeInTheDocument();
  });

  it("opens the Manager tab from a deep link once the probe succeeds", async () => {
    route({ manager: true });
    renderPage("/skills?tab=manager");
    expect(await screen.findByRole("table", { name: "People and their skills" })).toBeInTheDocument();
  });
});

describe("All Skills admin actions", () => {
  it("hides create, edit and deactivate from a non-admin and never asks for inactive skills", async () => {
    route({ manager: false });
    renderPage("/skills?tab=all");
    expect(await screen.findByText("Go")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "New skill" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit Go" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Deactivate Go" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Show inactive")).not.toBeInTheDocument();
    const searchBodies = api.post.mock.calls.filter(([p]) => p === "/skills/search").map(([, b]) => b);
    expect(searchBodies.length).toBeGreaterThan(0);
    for (const b of searchBodies) expect(b).not.toHaveProperty("includeInactive");
  });

  it("shows an admin create, edit and deactivate, and marks inactive skills", async () => {
    isAdmin = true;
    route({ manager: false });
    renderPage("/skills?tab=all");
    expect(await screen.findByRole("button", { name: "Edit Go" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New skill" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Deactivate Go" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Activate Legacy Tool" })).toBeInTheDocument();
    expect(screen.getByText("Inactive")).toBeInTheDocument();
  });

  it("deactivating asks for confirmation then PATCHes active=false", async () => {
    isAdmin = true;
    route({ manager: false });
    api.patch.mockResolvedValue({ ...skillGo, active: false });
    renderPage("/skills?tab=all");
    fireEvent.click(await screen.findByRole("button", { name: "Deactivate Go" }));
    expect(api.patch).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog", { name: /Deactivate Go\?/ });
    fireEvent.click(dialog.querySelector("button.MuiButton-contained") as HTMLElement);
    await waitFor(() => expect(api.patch).toHaveBeenCalledWith("/skills/skill-go", { active: false }));
  });

  it("sends the search text to the server, not a client-side filter", async () => {
    route({ manager: false });
    renderPage("/skills?tab=all");
    await screen.findByText("Go");
    fireEvent.change(screen.getByLabelText("Search skills by name or keyword"), { target: { value: "gol" } });
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/search", {
        searchQuery: "gol",
        pagination: { limit: 10, offset: 0 },
      }),
    );
  });

  it("creates a skill with POST /skills from the admin form", async () => {
    isAdmin = true;
    route({ manager: false });
    api.post.mockImplementation(async (path: string) => {
      if (path === "/skills/users/search") throw forbidden;
      if (path === "/skills/search") return { skills: [skillGo], total: 1, limit: 10, offset: 0 };
      return { ...skillGo, id: "new" };
    });
    renderPage("/skills?tab=all");
    fireEvent.click(await screen.findByRole("button", { name: "New skill" }));
    const dialog = await screen.findByRole("dialog", { name: "New skill" });
    const create = within(dialog).getByRole("button", { name: "Create skill" });
    expect(create).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText(/^Name/), { target: { value: "Rust" } });
    await waitFor(() => expect(within(dialog).getByLabelText(/^Level scale/)).toBeEnabled());
    fireEvent.change(within(dialog).getByLabelText(/^Level scale/), { target: { value: levelTypes[0].id } });
    fireEvent.click(create);
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills", { name: "Rust", levelTypeId: levelTypes[0].id, active: true }),
    );
  });
});

describe("All Skills level scale label", () => {
  const noScaleSkill = { ...skillGo, id: "skill-bare", name: "Bare Skill", levelTypeId: null };

  it("does not claim 'No level scale' while the scales are loading", async () => {
    route({ manager: false });
    api.get.mockImplementation(async (path: string) => {
      if (path === "/skills/levels") return new Promise(() => {});
      return path === "/skills/me" ? { userId: "u1", skills: [] } : { categories };
    });
    renderPage("/skills?tab=all");
    await screen.findByText("Go");
    expect(screen.queryByText("No level scale")).not.toBeInTheDocument();
    expect(screen.getAllByText("Loading levels…").length).toBeGreaterThan(0);
  });

  it("does not claim 'No level scale' when the scales failed to load", async () => {
    route({ manager: false });
    api.get.mockImplementation(async (path: string) => {
      if (path === "/skills/levels") throw new Error("boom");
      return path === "/skills/me" ? { userId: "u1", skills: [] } : { categories };
    });
    renderPage("/skills?tab=all");
    await screen.findByText("Go");
    await waitFor(() => expect(screen.getAllByText("Levels unavailable").length).toBeGreaterThan(0));
    expect(screen.queryByText("No level scale")).not.toBeInTheDocument();
  });

  it("shows 'No level scale' for a skill that has no scale", async () => {
    route({ manager: false });
    api.post.mockImplementation(async (path: string) => {
      if (path === "/skills/users/search") throw forbidden;
      return { skills: [noScaleSkill], total: 1, limit: 10, offset: 0 };
    });
    renderPage("/skills?tab=all");
    await screen.findByText("Bare Skill");
    expect(screen.getByText("No level scale")).toBeInTheDocument();
  });
});
