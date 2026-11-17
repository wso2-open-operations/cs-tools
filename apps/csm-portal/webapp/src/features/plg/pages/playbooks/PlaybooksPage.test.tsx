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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { LifecycleCatalogue, Playbook, PlaybookTask, Product } from "@features/plg/api/types";
import PlaybooksPage from "./PlaybooksPage";

const asgardeo = { id: "pr1", code: "asgardeo", name: "Asgardeo" } as Product;
const products = [asgardeo];
const catalogue: LifecycleCatalogue = {
  stages: [
    {
      stage: "REGISTRATION",
      name: "Registration",
      displayOrder: 1,
      description: null,
      nextStage: "PLG_CS_ELIGIBLE",
      terminal: false,
      progressivePlaybooks: 0,
      recoveryPlaybooks: 0,
      sustainingPlaybooks: 0,
    },
  ],
};

function task(over: Partial<PlaybookTask>): PlaybookTask {
  return {
    id: over.code ?? "t",
    code: "T",
    name: "",
    description: null,
    sequenceNo: 1,
    valueType: "BOOLEAN",
    options: null,
    active: true,
    isBookend: false,
    ...over,
  };
}

const onboard: Playbook = {
  id: "pb1",
  product: asgardeo,
  name: "Onboard",
  description: null,
  lifecycleStage: "REGISTRATION",
  stageName: "Registration",
  playbookType: "PROGRESSIVE",
  displayOrder: 1,
  active: true,
  tasks: [
    task({ code: "INITIATE_PLAYBOOK", name: "Initiate playbook", isBookend: true }),
    task({ code: "SEND_WELCOME", name: "Send welcome", sequenceNo: 2 }),
    task({ code: "CLOSE_PLAYBOOK", name: "Close playbook", sequenceNo: 3, isBookend: true }),
  ],
  runCount: 0,
  activeRuns: 0,
  createdOn: "2026-10-01T00:00:00Z",
  updatedOn: "2026-10-01T00:00:00Z",
  authoredBy: null,
  updatedBy: null,
};

const listed: { playbooks: Playbook[] } = { playbooks: [] };
const create = { mutate: vi.fn(), isPending: false, error: null };
const patch = { mutate: vi.fn(), isPending: false, error: null };
const replaceTasks = { mutate: vi.fn(), isPending: false, error: null };
const remove = { mutate: vi.fn(), isPending: false, error: null };

vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({ canManagePlaybooks: true }),
}));

vi.mock("@features/plg/api/hooks", () => ({
  useProducts: () => ({ data: products }),
  useLifecycle: () => ({ data: catalogue }),
  usePlaybooks: () => ({ data: listed.playbooks, isPending: false, error: null }),
  useCreatePlaybook: () => create,
  usePatchPlaybook: () => patch,
  useReplacePlaybookTasks: () => replaceTasks,
  useDeletePlaybook: () => remove,
}));

beforeEach(() => {
  listed.playbooks = [];
  for (const m of [create, patch, replaceTasks, remove]) m.mutate.mockReset();
});

/** Picks an option of a Select-backed TextField. */
function choose(field: string, option: string) {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: field }));
  fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: option }));
}

function openNewPlaybook() {
  render(<PlaybooksPage />);
  choose("Platform", "Asgardeo");
  fireEvent.click(screen.getByRole("button", { name: "New playbook" }));
}

const NOT_SAVED = "A task with no name is not saved.";

describe("PlaybooksPage: a new playbook's tasks", () => {
  it("marks a task's name required, says an unnamed task is dropped, and creates without it", () => {
    openNewPlaybook();
    const name = screen.getByRole("textbox", { name: "Task 2" });
    expect(name).toBeRequired();
    expect(screen.getByText(NOT_SAVED)).toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Rescue" } });
    choose("Kind", "Progressive Playbook");
    choose("Runs at stage", "Registration");
    // The blank row does not hold the playbook back: it is left out.
    const createButton = screen.getByRole("button", { name: "Create playbook" });
    expect(createButton).toBeEnabled();
    fireEvent.click(createButton);
    expect(create.mutate).toHaveBeenCalledWith(expect.objectContaining({ name: "Rescue", tasks: [] }), expect.anything());

    fireEvent.change(name, { target: { value: "Call them" } });
    expect(screen.queryByText(NOT_SAVED)).not.toBeInTheDocument();
  });

  it("holds Create back while a named list task has a reason without a label", () => {
    openNewPlaybook();
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), { target: { value: "Rescue" } });
    choose("Kind", "Progressive Playbook");
    choose("Runs at stage", "Registration");
    choose("Holds", "Choose any");

    // Unnamed, the task is dropped, so its blank reasons hold nothing back and are not marked.
    const reasons = () => [1, 2].map((n) => screen.getByRole("textbox", { name: `Reason ${n}` }));
    for (const r of reasons()) {
      expect(r).not.toBeRequired();
      expect(r).not.toHaveAttribute("aria-invalid", "true");
    }
    const createButton = screen.getByRole("button", { name: "Create playbook" });
    expect(createButton).toBeEnabled();

    fireEvent.change(screen.getByRole("textbox", { name: "Task 2" }), { target: { value: "Why they left" } });
    expect(createButton).toBeDisabled();
    for (const r of reasons()) {
      expect(r).toBeRequired();
      expect(r).toHaveAttribute("aria-invalid", "true");
    }
    expect(screen.getAllByText("A reason needs a label")).toHaveLength(2);

    fireEvent.change(reasons()[0], { target: { value: "Price" } });
    expect(createButton).toBeDisabled();
    fireEvent.change(reasons()[1], { target: { value: "Features" } });
    expect(createButton).toBeEnabled();
    fireEvent.click(createButton);
    expect(create.mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        tasks: [
          {
            name: "Why they left",
            valueType: "CHECKLIST",
            options: [
              { code: "", label: "Price" },
              { code: "", label: "Features" },
            ],
          },
        ],
      }),
      expect.anything(),
    );
  });
});

describe("PlaybooksPage: editing a playbook's tasks", () => {
  it("holds Save back while a list task has a reason without a label", () => {
    listed.playbooks = [onboard];
    render(<PlaybooksPage />);
    fireEvent.click(screen.getByRole("button", { name: /^Onboard/ }));
    expect(screen.getByRole("textbox", { name: "Task 2" })).toBeRequired();
    expect(screen.queryByText(NOT_SAVED)).not.toBeInTheDocument();

    choose("Holds", "Choose one");
    const save = screen.getByRole("button", { name: "Save changes" });
    expect(save).toBeDisabled();
    fireEvent.change(screen.getByRole("textbox", { name: "Reason 1" }), { target: { value: "Yes" } });
    fireEvent.change(screen.getByRole("textbox", { name: "Reason 2" }), { target: { value: "No" } });
    expect(save).toBeEnabled();

    // A new, unnamed row is dropped on save rather than holding it back.
    fireEvent.click(screen.getByRole("button", { name: "Add task" }));
    expect(screen.getByText(NOT_SAVED)).toBeInTheDocument();
    expect(save).toBeEnabled();
    fireEvent.click(save);
    expect(replaceTasks.mutate).toHaveBeenCalledWith([
      {
        code: "SEND_WELCOME",
        name: "Send welcome",
        description: null,
        valueType: "SINGLE_SELECT",
        options: [
          { code: "", label: "Yes" },
          { code: "", label: "No" },
        ],
      },
    ]);
  });
});
