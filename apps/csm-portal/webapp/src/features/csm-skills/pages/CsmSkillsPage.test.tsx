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

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

import CsmSkillsPage from "@features/csm-skills/pages/CsmSkillsPage";
import { categories, levelTypes, mineGo, skillGo, skillKube, skillOld } from "@features/csm-skills/test/fixtures";
import type { SkillCapabilities, SkillRequest } from "@features/csm-skills/types";

/** A 409 as the real client throws it. */
const conflict = (message: string) => Object.assign(new Error(message), { status: 409 });

const ENGINEER: SkillCapabilities = { canViewDirectory: false, canReviewRequests: false, canManageCatalogue: false };
const ADMIN: SkillCapabilities = { canViewDirectory: true, canReviewRequests: true, canManageCatalogue: true, pendingRequestCount: 1 };

const defaultScale = levelTypes.map((t, i) => ({ ...t, isDefault: i === 0 }));

const pendingRequest: SkillRequest = {
  id: "req-1",
  requestedName: "Ballerina",
  description: "The Ballerina language",
  category: { id: "cat-lang", name: "Languages", categoryPath: "Engineering / Languages" },
  requestedLevel: { id: "lvl-a-intermediate", name: "Intermediate", value: 50 },
  status: "PENDING",
  requestedBy: { id: "u2", name: "Jane Doe", email: "jane@example.com" },
  createdOn: "2026-11-01T00:00:00Z",
};

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

function route({ caps = ENGINEER }: { caps?: SkillCapabilities; manager?: boolean } = {}) {
  api.get.mockImplementation(async (path: string) => {
    if (path === "/skills/me/capabilities") return caps;
    if (path === "/skills/me") return { userId: "u1", skills: [mineGo] };
    if (path === "/skills/levels") return { levelTypes: defaultScale };
    if (path === "/skills/categories") return { categories };
    if (path === "/skills/requests/mine") return { requests: [], total: 0, limit: 50, offset: 0 };
    if (path === "/skills/skill-kube") return skillKube;
    if (path.startsWith("/team-schedule/catalogue")) return { zones: [], shifts: [], absenceKinds: [], teams: [] };
    return null;
  });
  api.post.mockImplementation(async (path: string, body?: { includeInactive?: boolean; name?: string }) => {
    if (path === "/skills/search") {
      const list = body?.includeInactive ? [skillGo, skillKube, skillOld] : [skillGo, skillKube];
      return { skills: list, total: list.length, limit: 10, offset: 0 };
    }
    if (path === "/skills/directory/search") return { items: [], total: 0, limit: 25, offset: 0 };
    if (path === "/skills/requests/search") return { requests: [pendingRequest], total: 1, limit: 25, offset: 0 };
    if (path === "/skills/requests/check") {
      return body?.name?.toLowerCase() === "go"
        ? { exactMatch: { id: "skill-go", name: "Go", active: true }, similar: [] }
        : { similar: [] };
    }
    return {};
  });
}

beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset();
});

describe("CsmSkillsPage tabs", () => {
  it("shows an engineer My Skills and All Skills only, from the capabilities, with no probe", async () => {
    route();
    renderPage();
    expect(await screen.findByRole("tab", { name: "My Skills" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "All Skills" })).toBeInTheDocument();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith("/skills/me/capabilities"));
    expect(screen.queryByRole("tab", { name: "Skill Directory" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /Administration/ })).not.toBeInTheDocument();
    expect(api.post.mock.calls.filter(([p]) => p === "/skills/users/search")).toHaveLength(0);
  });

  it("shows the Skill Directory to a holder of the directory capability", async () => {
    route({ caps: { ...ENGINEER, canViewDirectory: true } });
    renderPage();
    expect(await screen.findByRole("tab", { name: "Skill Directory" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /Administration/ })).not.toBeInTheDocument();
  });

  it("lands an old Skill Manager link on the directory, which searches one row per person", async () => {
    route({ caps: { ...ENGINEER, canViewDirectory: true } });
    renderPage("/skills?tab=manager");
    expect(await screen.findByRole("table", { name: "People by skill" })).toBeInTheDocument();
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/directory/search", { pagination: { limit: 25, offset: 0 } }),
    );
    fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "jane" } });
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/directory/search", {
        searchQuery: "jane",
        pagination: { limit: 25, offset: 0 },
      }),
    );
  });

  it("falls back to My Skills for a gated deep link the caller cannot use", async () => {
    route();
    renderPage("/skills?tab=admin");
    expect(await screen.findByRole("table", { name: "My skills" })).toBeInTheDocument();
  });

  it("shows Administration with the pending count to an admin", async () => {
    route({ caps: ADMIN });
    renderPage();
    const tab = await screen.findByRole("tab", { name: /Administration/ });
    expect(within(tab).getByLabelText("1 pending requests")).toBeInTheDocument();
  });
});

describe("All Skills (tree)", () => {
  it("files skills under their categories and opens a skill beside the tree", async () => {
    route();
    renderPage("/skills?tab=all");
    const tree = await screen.findByRole("tree", { name: "Skills by category" });
    // Top-level categories start open; Kubernetes sits directly under Operations.
    expect(within(tree).getByText("Engineering")).toBeInTheDocument();
    expect(within(tree).getByText("Operations")).toBeInTheDocument();
    expect(within(tree).queryByText("Go")).not.toBeInTheDocument();
    fireEvent.click(within(tree).getByText("Languages"));
    expect(within(tree).getByText("Go")).toBeInTheDocument();
    expect(within(tree).getByLabelText("On your profile")).toBeInTheDocument();

    fireEvent.click(within(tree).getByText("Kubernetes"));
    const detail = await screen.findByRole("region", { name: "Kubernetes" });
    expect(within(detail).getByRole("button", { name: "Add to my skills" })).toBeInTheDocument();
    // An engineer without directory access sees no people, and nobody is asked for them.
    expect(within(detail).getByText(/needs access to the Skill Directory/)).toBeInTheDocument();
    expect(api.post.mock.calls.filter(([p]) => p === "/skills/directory/search")).toHaveLength(0);
    for (const [p, b] of api.post.mock.calls) if (p === "/skills/search") expect(b).not.toHaveProperty("includeInactive");

    fireEvent.click(within(detail).getByRole("button", { name: "Add to my skills" }));
    const dialog = await screen.findByRole("dialog", { name: "Add a skill" });
    await waitFor(() => expect(within(dialog).getByLabelText(/^Level/)).toBeEnabled());
  });

  it("lists the people who hold the selected skill for a directory viewer", async () => {
    route({ caps: { ...ENGINEER, canViewDirectory: true } });
    api.post.mockImplementation(async (path: string, body?: { criteria?: { skillId: string }[] }) => {
      if (path === "/skills/search") return { skills: [skillGo, skillKube], total: 2, limit: 200, offset: 0 };
      if (path === "/skills/directory/search" && body?.criteria?.[0]?.skillId === "skill-kube") {
        return {
          items: [
            {
              userId: "u9",
              userName: "Sam Lee",
              userEmail: "sam@example.com",
              teamKeys: [],
              matchedCount: 1,
              skills: [{ ...mineGo, skillId: "skill-kube", skillName: "Kubernetes", level: { id: "lvl-b-advanced", name: "Advanced", value: 2 } }],
            },
          ],
          total: 1,
          limit: 10,
          offset: 0,
        };
      }
      return { items: [], total: 0, limit: 10, offset: 0 };
    });
    renderPage("/skills?tab=all&skill=skill-kube");
    const people = await screen.findByRole("table", { name: "People with Kubernetes" });
    expect(await within(people).findByText("Sam Lee")).toBeInTheDocument();
    expect(within(people).getByText("Advanced")).toBeInTheDocument();
    expect(api.post).toHaveBeenCalledWith("/skills/directory/search", {
      criteria: [{ skillId: "skill-kube" }],
      match: "ALL",
      pagination: { limit: 10, offset: 0 },
    });
  });

  it("sends the search text to the server, not a client-side filter", async () => {
    route();
    renderPage("/skills?tab=all");
    await screen.findByRole("tree", { name: "Skills by category" });
    fireEvent.change(screen.getByLabelText("Search skills by name or keyword"), { target: { value: "gol" } });
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/search", {
        searchQuery: "gol",
        pagination: { limit: 200, offset: 0 },
      }),
    );
  });
});

describe("Requesting a skill", () => {
  it("points at an existing skill instead of requesting it", async () => {
    route();
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Request a skill" }));
    const dialog = await screen.findByRole("dialog", { name: "Request a skill" });
    fireEvent.change(within(dialog).getByLabelText(/^Skill name/), { target: { value: "Go" } });
    expect(await within(dialog).findByText(/is already in the catalogue/)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Send request" })).toBeDisabled();
  });

  it("sends the name and the level the skill is added at", async () => {
    route();
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Request a skill" }));
    const dialog = await screen.findByRole("dialog", { name: "Request a skill" });
    fireEvent.change(within(dialog).getByLabelText(/^Skill name/), { target: { value: "Ballerina" } });
    await waitFor(() => expect(within(dialog).getByLabelText(/^Your level/)).toBeEnabled());
    // Only the default scale's levels are offered.
    expect(within(dialog).queryByText("Advanced")).not.toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText(/^Your level/), { target: { value: "lvl-a-intermediate" } });
    api.post.mockImplementationOnce(async () => ({ ...pendingRequest, requestedBy: { id: "u1" } }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Send request" }));
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/requests", { name: "Ballerina", levelId: "lvl-a-intermediate" }),
    );
  });
});

describe("Administration", () => {
  it("approves a request as a new skill in the requested category", async () => {
    route({ caps: ADMIN });
    renderPage("/skills?tab=admin");
    fireEvent.click(await screen.findByRole("button", { name: "Review the request for Ballerina" }));
    const dialog = await screen.findByRole("dialog", { name: "Review request: Ballerina" });
    api.post.mockImplementationOnce(async () => ({
      ...pendingRequest,
      status: "APPROVED",
      resolvedSkill: { id: "s-new", name: "Ballerina", active: true },
      autoAddedLevel: pendingRequest.requestedLevel,
    }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Approve" }));
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/requests/req-1/approve", {
        mode: "create",
        name: "Ballerina",
        categoryId: "cat-lang",
        description: "The Ballerina language",
      }),
    );
    expect(await screen.findByText(/added to Jane Doe's skills at Intermediate/)).toBeInTheDocument();
  });

  it("requires a reason to reject, and shows a 409 when someone else decided first", async () => {
    route({ caps: ADMIN });
    renderPage("/skills?tab=admin");
    fireEvent.click(await screen.findByRole("button", { name: "Review the request for Ballerina" }));
    const dialog = await screen.findByRole("dialog", { name: "Review request: Ballerina" });
    fireEvent.click(within(dialog).getByLabelText("Reject"));
    const button = within(dialog).getByRole("button", { name: "Reject request" });
    expect(button).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText(/^Reason/), { target: { value: "Covered by Go" } });
    api.post.mockImplementationOnce(async () => {
      throw conflict("this request is already approved by Sam");
    });
    fireEvent.click(button);
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/requests/req-1/reject", { reason: "Covered by Go" }),
    );
    expect(await within(dialog).findByText("this request is already approved by Sam")).toBeInTheDocument();
  });

  it("Skills: deactivating asks for confirmation then PATCHes active=false", async () => {
    route({ caps: ADMIN });
    api.patch.mockResolvedValue({ ...skillGo, active: false });
    renderPage("/skills?tab=admin");
    fireEvent.click(await screen.findByRole("tab", { name: "Skills" }));
    fireEvent.click(await screen.findByRole("button", { name: "Deactivate Go" }));
    expect(api.patch).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog", { name: /Deactivate Go\?/ });
    fireEvent.click(dialog.querySelector("button.MuiButton-contained") as HTMLElement);
    await waitFor(() => expect(api.patch).toHaveBeenCalledWith("/skills/skill-go", { active: false }));
  });

  it("Skills: a new skill starts on the default scale", async () => {
    route({ caps: ADMIN });
    renderPage("/skills?tab=admin");
    fireEvent.click(await screen.findByRole("tab", { name: "Skills" }));
    fireEvent.click(await screen.findByRole("button", { name: "New skill" }));
    const dialog = await screen.findByRole("dialog", { name: "New skill" });
    fireEvent.change(within(dialog).getByLabelText(/^Name/), { target: { value: "Rust" } });
    api.post.mockImplementationOnce(async () => ({ ...skillGo, id: "new" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Create skill" }));
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills", { name: "Rust", levelTypeId: levelTypes[0].id, active: true }),
    );
  });

  it("Categories: adds a sub-category under a category", async () => {
    route({ caps: ADMIN });
    renderPage("/skills?tab=admin");
    fireEvent.click(await screen.findByRole("tab", { name: "Categories" }));
    fireEvent.click(await screen.findByRole("button", { name: "Add a sub-category under Engineering" }));
    const dialog = await screen.findByRole("dialog", { name: "New sub-category of Engineering" });
    fireEvent.change(within(dialog).getByLabelText(/^Name/), { target: { value: "Integration" } });
    api.post.mockImplementationOnce(async () => ({ id: "c-new", name: "Integration", isLowestCategory: true }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Create" }));
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/categories", { name: "Integration", parentId: "cat-eng" }),
    );
  });
});

describe("All Skills level scale label", () => {
  const noScaleSkill = { ...skillGo, id: "skill-bare", name: "Bare Skill", levelTypeId: null };

  it("does not claim 'No level scale' while the scales are loading", async () => {
    route();
    api.get.mockImplementation(async (path: string) => {
      if (path === "/skills/levels") return new Promise(() => {});
      return path === "/skills/me" ? { userId: "u1", skills: [] } : { categories };
    });
    renderPage("/skills?tab=all&skill=skill-go");
    await screen.findByRole("region", { name: "Go" });
    expect(screen.queryByText("No level scale")).not.toBeInTheDocument();
    expect(screen.getAllByText("Loading levels…").length).toBeGreaterThan(0);
  });

  it("does not claim 'No level scale' when the scales failed to load", async () => {
    route();
    api.get.mockImplementation(async (path: string) => {
      if (path === "/skills/levels") throw new Error("boom");
      return path === "/skills/me" ? { userId: "u1", skills: [] } : { categories };
    });
    renderPage("/skills?tab=all&skill=skill-go");
    await screen.findByRole("region", { name: "Go" });
    await waitFor(() => expect(screen.getAllByText("Levels unavailable").length).toBeGreaterThan(0));
    expect(screen.queryByText("No level scale")).not.toBeInTheDocument();
  });

  it("shows 'No level scale' for a skill that has no scale", async () => {
    route();
    api.post.mockImplementation(async () => ({ skills: [noScaleSkill], total: 1, limit: 10, offset: 0 }));
    renderPage("/skills?tab=all&skill=skill-bare");
    await screen.findByRole("region", { name: "Bare Skill" });
    expect(screen.getByText("No level scale")).toBeInTheDocument();
  });
});
