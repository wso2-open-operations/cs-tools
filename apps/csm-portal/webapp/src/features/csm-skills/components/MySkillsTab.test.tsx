import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
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

import MySkillsTab from "@features/csm-skills/components/MySkillsTab";
import {
  categories,
  levelTypes,
  mineGo,
  mineInherited,
  skillGo,
  skillKube,
} from "@features/csm-skills/test/fixtures";

function renderTab() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <MySkillsTab />
    </QueryClientProvider>,
  );
}

function route(mine: unknown) {
  api.get.mockImplementation(async (path: string) => {
    if (path === "/skills/me") return mine;
    if (path === "/skills/levels") return { levelTypes };
    if (path === "/skills/categories") return { categories };
    if (path.startsWith("/skills/me/history")) return { history: [], total: 0, limit: 20, offset: 0 };
    return null;
  });
  api.post.mockImplementation(async (path: string) => {
    if (path === "/skills/search") return { skills: [skillGo, skillKube], total: 2, limit: 50, offset: 0 };
    return {};
  });
}

beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset();
});

describe("MySkillsTab", () => {
  it("lists the caller's skills with level and categories", async () => {
    route({ userId: "u1", skills: [mineGo] });
    renderTab();
    expect(await screen.findByRole("table", { name: "My skills" })).toBeInTheDocument();
    expect(screen.getByText("Go")).toBeInTheDocument();
    expect(screen.getByText("Engineering / Languages")).toBeInTheDocument();
    expect(screen.getByLabelText("Level for Go")).toHaveValue("lvl-a-beginner");
  });

  it("shows an empty state with a way to add the first skill", async () => {
    route({ userId: "u1", skills: [] });
    renderTab();
    expect(await screen.findByText("You haven't added any skills yet.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add your first skill" })).toBeInTheDocument();
  });

  it("shows an error state with retry when the profile cannot be loaded", async () => {
    api.get.mockImplementation(async (path: string) => {
      if (path === "/skills/me") throw new Error("profile unavailable");
      return { levelTypes };
    });
    renderTab();
    expect(await screen.findByText("profile unavailable")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("offers only the levels of the skill's own scale and saves a change with PUT", async () => {
    route({ userId: "u1", skills: [mineGo] });
    api.put.mockResolvedValue({ ...mineGo });
    renderTab();
    const select = await screen.findByLabelText("Level for Go");
    const options = within(select).getAllByRole("option").map((o) => o.textContent);
    // Ordered by value, and nothing from the other scale.
    expect(options).toEqual(["Beginner", "Intermediate", "Expert"]);
    fireEvent.change(select, { target: { value: "lvl-a-expert" } });
    await waitFor(() => expect(api.put).toHaveBeenCalledWith("/skills/me/skill-go", { levelId: "lvl-a-expert" }));
  });

  it("shows the backend message when a level change is refused", async () => {
    route({ userId: "u1", skills: [mineGo] });
    api.put.mockRejectedValue(new Error("that level is not on this skill's scale"));
    renderTab();
    fireEvent.change(await screen.findByLabelText("Level for Go"), { target: { value: "lvl-a-expert" } });
    expect(await screen.findByText("that level is not on this skill's scale")).toBeInTheDocument();
  });

  it("removes a skill with DELETE only after confirming", async () => {
    route({ userId: "u1", skills: [mineGo] });
    api.del.mockResolvedValue(null);
    renderTab();
    fireEvent.click(await screen.findByRole("button", { name: "Remove Go" }));
    expect(api.del).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog", { name: /Remove Go\?/ });
    fireEvent.click(within(dialog).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(api.del).toHaveBeenCalledWith("/skills/me/skill-go"));
  });

  it("does not delete when the confirmation is cancelled", async () => {
    route({ userId: "u1", skills: [mineGo] });
    renderTab();
    fireEvent.click(await screen.findByRole("button", { name: "Remove Go" }));
    const dialog = await screen.findByRole("dialog", { name: /Remove Go\?/ });
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(api.del).not.toHaveBeenCalled();
  });

  it("makes an inherited row read-only: no level select and no remove", async () => {
    route({ userId: "u1", skills: [mineInherited] });
    renderTab();
    expect(await screen.findByText("Kubernetes")).toBeInTheDocument();
    expect(screen.getByText("Inherited")).toBeInTheDocument();
    expect(screen.queryByLabelText("Level for Kubernetes")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Remove Kubernetes" })).not.toBeInTheDocument();
    expect(screen.getByText("Basic")).toBeInTheDocument();
  });

  it("add flow: level choices are limited to the chosen skill's scale, then POSTs", async () => {
    route({ userId: "u1", skills: [mineGo] });
    api.post.mockImplementation(async (path: string, body: unknown) => {
      if (path === "/skills/search") return { skills: [skillGo, skillKube], total: 2, limit: 50, offset: 0 };
      if (path === "/skills/me") return { ...mineInherited, inherited: false, body };
      return {};
    });
    renderTab();
    fireEvent.click(await screen.findByRole("button", { name: "Add skill" }));
    const dialog = await screen.findByRole("dialog", { name: "Add a skill" });

    const level = within(dialog).getByLabelText(/^Level/);
    expect(level).toBeDisabled();

    const skillInput = within(dialog).getByRole("combobox", { name: /Skill/ });
    fireEvent.mouseDown(skillInput);
    // Go is already on the profile, so only Kubernetes is offered.
    const option = await screen.findByRole("option", { name: "Kubernetes" });
    expect(screen.queryByRole("option", { name: "Go" })).not.toBeInTheDocument();
    fireEvent.click(option);

    await waitFor(() => expect(within(dialog).getByLabelText(/^Level/)).toBeEnabled());
    const names = within(within(dialog).getByLabelText(/^Level/))
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(names).toEqual(["Select a level", "Basic", "Advanced"]);

    fireEvent.change(within(dialog).getByLabelText(/^Level/), { target: { value: "lvl-b-advanced" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add skill" }));
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/me", { skillId: "skill-kube", levelId: "lvl-b-advanced" }),
    );
  });

  it("add flow: a category narrows the skill search on the server", async () => {
    route({ userId: "u1", skills: [] });
    renderTab();
    fireEvent.click(await screen.findByRole("button", { name: "Add your first skill" }));
    const dialog = await screen.findByRole("dialog", { name: "Add a skill" });
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: /Category/ }));
    fireEvent.click(await screen.findByRole("option", { name: "Operations" }));

    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: /Skill/ }));
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/skills/search", {
        categoryIds: ["cat-ops"],
        pagination: { limit: 50, offset: 0 },
      }),
    );
  });
});
