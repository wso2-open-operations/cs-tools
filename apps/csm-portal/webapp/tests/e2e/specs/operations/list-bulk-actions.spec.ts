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

//
// Bulk actions and export on the Change requests and Incidents lists:
// selection (never a closed record), Assign to me, Edit (n) and the Export
// menu (CSV, Excel, JSON, PDF). The two list searches, each record's detail
// and its PATCH are faked here, in memory, so nothing is written anywhere and
// the exact requests the page sends can be checked; everything else (sign-in,
// /users/me, the group and people pickers) is the real local stack.
//

import fs from "node:fs";
import type { Page, Route } from "@playwright/test";
import { test, expect, withRole } from "../../fixtures/test";

withRole(test, "crApprover");

interface FakeRecord {
  id: string;
  number: string;
  subject: string;
  state: string;
  legalNextStates?: string[];
  /** Incidents: the watch list the detail returns. */
  watchList?: Array<{ id: string; name: string; email: string }>;
}

interface FakeLists {
  /** Every PATCH the page sent, in order. */
  patches: Array<{ path: string; body: Record<string, unknown> }>;
  /** Make the PATCH of this id fail with a 400 in these words. */
  refuse(id: string, message: string): void;
}

const CHANGE_REQUESTS: FakeRecord[] = [
  { id: "00000000-0000-4000-8000-0000000c0001", number: "CHG-E2E-1", subject: "Rotate certificates", state: "new", legalNextStates: ["assess", "canceled"] },
  { id: "00000000-0000-4000-8000-0000000c0002", number: "CHG-E2E-2", subject: "Patch gateway", state: "scheduled", legalNextStates: ["implement", "canceled"] },
  { id: "00000000-0000-4000-8000-0000000c0003", number: "CHG-E2E-3", subject: "Old upgrade", state: "closed", legalNextStates: [] },
];

const INCIDENTS: FakeRecord[] = [
  {
    id: "00000000-0000-4000-8000-0000000a0001",
    number: "INC-E2E-1",
    subject: "Disk full",
    state: "IN_PROGRESS",
    watchList: [{ id: "00000000-0000-4000-8000-00000000d001", name: "Existing Watcher", email: "existing@example.com" }],
  },
  { id: "00000000-0000-4000-8000-0000000a0002", number: "INC-E2E-2", subject: "Latency spike", state: "IN_PROGRESS" },
  { id: "00000000-0000-4000-8000-0000000a0003", number: "INC-E2E-3", subject: "Resolved long ago", state: "CLOSED" },
];

async function fakeLists(page: Page): Promise<FakeLists> {
  const patches: FakeLists["patches"] = [];
  const refusals = new Map<string, string>();
  const cors = (route: Route): Record<string, string> => ({
    "access-control-allow-origin": route.request().headers()["origin"] ?? "*",
    "access-control-allow-headers": "*",
    "access-control-allow-methods": "GET,POST,PATCH,PUT,DELETE,OPTIONS",
    "access-control-allow-credentials": "true",
  });
  const json = (route: Route, body: unknown, status = 200): Promise<void> =>
    route.fulfill({ status, contentType: "application/json", headers: cors(route), body: JSON.stringify(body) });
  const apiCall = async (route: Route): Promise<boolean> => {
    const type = route.request().resourceType();
    if (type !== "fetch" && type !== "xhr") {
      await route.fallback();
      return false;
    }
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers: cors(route) });
      return false;
    }
    return true;
  };
  const crView = (r: FakeRecord): Record<string, unknown> => ({
    id: r.id,
    number: r.number,
    subject: r.subject,
    state: r.state,
    impact: "low",
    project: { id: "p-1", name: "E2E Project" },
    plannedStartOn: "2030-03-01T09:00:00Z",
    updatedOn: "2026-10-01T00:00:00Z",
    createdOn: "2026-09-01T00:00:00Z",
  });

  await page.route(
    (url) => url.pathname.endsWith("/change-requests/search"),
    async (route) => {
      if (!(await apiCall(route))) return;
      const { pagination } = route.request().postDataJSON() as { pagination: { offset: number; limit: number } };
      const page = CHANGE_REQUESTS.slice(pagination.offset, pagination.offset + pagination.limit);
      await json(route, { changeRequests: page.map(crView), total: CHANGE_REQUESTS.length, limit: pagination.limit, offset: pagination.offset });
    },
  );
  await page.route(
    (url) => /\/change-requests\/[0-9a-f-]{36}$/.test(url.pathname),
    async (route) => {
      if (!(await apiCall(route))) return;
      const id = new URL(route.request().url()).pathname.split("/").pop()!;
      const record = CHANGE_REQUESTS.find((r) => r.id === id);
      if (!record) return json(route, { code: 404, message: "change request not found" }, 404);
      if (route.request().method() === "GET") return json(route, { ...crView(record), legalNextStates: record.legalNextStates });
      if (route.request().method() === "PATCH") {
        patches.push({ path: `/change-requests/${id}`, body: route.request().postDataJSON() });
        const refusal = refusals.get(id);
        if (refusal) return json(route, { code: 400, message: refusal }, 400);
        return json(route, { id });
      }
      return route.fallback();
    },
  );
  await page.route(
    (url) => url.pathname.endsWith("/incidents/search"),
    async (route) => {
      if (!(await apiCall(route))) return;
      const { pagination } = route.request().postDataJSON() as { pagination: { offset: number; limit: number } };
      const rows = INCIDENTS.slice(pagination.offset, pagination.offset + pagination.limit).map((r) => ({
        id: r.id,
        number: r.number,
        subject: r.subject,
        state: r.state,
        priority: "MODERATE",
        caller: { id: "u-9", name: "Caller Person" },
        openedOn: "2026-10-01T00:00:00Z",
        updatedOn: "2026-10-02T00:00:00Z",
      }));
      await json(route, { incidents: rows, total: INCIDENTS.length, limit: pagination.limit, offset: pagination.offset, hasMore: false });
    },
  );
  await page.route(
    (url) => /\/incidents\/[0-9a-f-]{36}$/.test(url.pathname),
    async (route) => {
      if (!(await apiCall(route))) return;
      const id = new URL(route.request().url()).pathname.split("/").pop()!;
      if (route.request().method() === "GET") {
        const record = INCIDENTS.find((r) => r.id === id);
        if (!record) return json(route, { code: 404, message: "incident not found" }, 404);
        return json(route, { id: record.id, number: record.number, subject: record.subject, state: record.state, watchList: record.watchList ?? [] });
      }
      if (route.request().method() !== "PATCH") return route.fallback();
      patches.push({ path: `/incidents/${id}`, body: route.request().postDataJSON() });
      const refusal = refusals.get(id);
      if (refusal) return json(route, { code: 400, message: refusal }, 400);
      return json(route, { id });
    },
  );
  return { patches, refuse: (id, message) => refusals.set(id, message) };
}

/** The signed-in user's id, as the page reads it. */
async function myUserId(page: Page): Promise<string> {
  const me = await page.waitForResponse((r) => r.url().endsWith("/users/me") && r.request().method() === "GET");
  return ((await me.json()) as { id: string }).id;
}

test.describe("change request list — bulk actions", () => {
  test("selects every open change request of the page, never a closed one", async ({ page }) => {
    await fakeLists(page);
    await page.goto("/operations/change-requests");

    await expect(page.getByRole("checkbox", { name: "CHG-E2E-3 is closed and can't be selected" })).toBeDisabled();
    await page.getByRole("checkbox", { name: "Select all change requests on this page" }).check();
    const bar = page.getByRole("region", { name: "Selected rows" });
    await expect(bar.getByText("2 change requests selected")).toBeVisible();
    await expect(bar.getByRole("button", { name: "Edit (2)" })).toBeEnabled();
    await expect(bar.getByRole("button", { name: "Assign to me (2)" })).toBeEnabled();
    // Ticking a row's box does not open the change request.
    await expect(page).toHaveURL(/\/operations\/change-requests$/);
  });

  test("Assign to me assigns the open one and says why the approved one was skipped", async ({ page }) => {
    const api = await fakeLists(page);
    const me = myUserId(page);
    await page.goto("/operations/change-requests");
    const userId = await me;

    await page.getByRole("checkbox", { name: "Select all change requests on this page" }).check();
    await page.getByRole("button", { name: "Assign to me (2)" }).click();

    await expect(page.getByText("Assigned to you 1 of 2 change requests. 1 skipped.")).toBeVisible();
    await expect(page.getByRole("list", { name: "Not updated" })).toContainText(
      "CHG-E2E-2: Approved, so its assignee can no longer be changed.",
    );
    expect(api.patches).toEqual([{ path: `/change-requests/${CHANGE_REQUESTS[0].id}`, body: { assignedEngineerId: userId } }]);
  });

  test("Edit cancels both with one reason, sends the approved one no field, and reports a refusal in the backend's words", async ({ page }) => {
    const api = await fakeLists(page);
    api.refuse(CHANGE_REQUESTS[1].id, "change request is on hold; take it off hold (onHold: false) before changing its state");
    await page.goto("/operations/change-requests");

    await page.getByRole("checkbox", { name: "Select all change requests on this page" }).check();
    await page.getByRole("button", { name: "Edit (2)" }).click();
    const dialog = page.getByRole("dialog", { name: "Edit 2 change requests" });
    await expect(dialog.getByText("Only the moves every selected change request can make.")).toBeVisible();

    await dialog.getByRole("combobox", { name: "State" }).click();
    await expect(page.getByRole("option")).toHaveText(["No change", "Cancel change (Canceled)"]);
    await page.getByRole("option", { name: "Cancel change (Canceled)" }).click();
    await expect(dialog.getByRole("button", { name: "Apply to 2 change requests" })).toBeDisabled();

    await dialog.getByRole("combobox", { name: "Priority" }).click();
    await page.getByRole("option", { name: "Low" }).click();
    await dialog.getByRole("textbox", { name: /Reason \(internal work note\)/ }).fill("Superseded by CHG-E2E-9.");
    await dialog.getByRole("button", { name: "Apply to 2 change requests" }).click();

    await expect(page.getByText("Updated 1 of 2 change requests. 1 could not be updated.")).toBeVisible();
    await expect(page.getByRole("list", { name: "Not updated" })).toContainText("CHG-E2E-2: change request is on hold");
    // The PATCHes run concurrently, so their order is not fixed.
    expect(api.patches).toHaveLength(2);
    expect(api.patches).toEqual(
      expect.arrayContaining([
        { path: `/change-requests/${CHANGE_REQUESTS[0].id}`, body: { state: "canceled", priority: "low", workNote: "Superseded by CHG-E2E-9." } },
        { path: `/change-requests/${CHANGE_REQUESTS[1].id}`, body: { state: "canceled", workNote: "Superseded by CHG-E2E-9." } },
      ]),
    );
  });

  test("Edit is wide enough that no field is squeezed or scrolls sideways", async ({ page }) => {
    await fakeLists(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/operations/change-requests");

    await page.getByRole("checkbox", { name: "Select all change requests on this page" }).check();
    await page.getByRole("button", { name: "Edit (2)" }).click();
    const dialog = page.getByRole("dialog", { name: "Edit 2 change requests" });
    await expect(dialog.getByText("Only the moves every selected change request can make.")).toBeVisible();

    const box = await dialog.boundingBox();
    expect(box?.width ?? 0).toBeGreaterThanOrEqual(1100);
    // Nothing in it is wider than the dialog: no sideways scroll, no field pushed out.
    const overflow = await dialog.locator(".MuiDialogContent-root").evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    // Every rich-text panel's toolbar fits whole: none needs its "scroll right" control.
    await expect(dialog.getByRole("group", { name: "Rollback plan", exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Scroll toolbar right" })).toHaveCount(0);
  });

  test("Edit sets the plan, the hold and a comment; the approved one gets only the hold and the comment", async ({ page }) => {
    const api = await fakeLists(page);
    await page.goto("/operations/change-requests");

    await page.getByRole("checkbox", { name: "Select all change requests on this page" }).check();
    await page.getByRole("button", { name: "Edit (2)" }).click();
    const dialog = page.getByRole("dialog", { name: "Edit 2 change requests" });
    await expect(dialog.getByText("Only the moves every selected change request can make.")).toBeVisible();
    await expect(dialog.getByText(/1 of the selected change requests has been approved/)).toBeVisible();

    await dialog.getByRole("textbox", { name: "Short description" }).fill("Rotate the gateway certificates");
    await dialog.getByRole("combobox", { name: "On hold" }).click();
    await page.getByRole("option", { name: "Put on hold" }).click();
    await dialog.getByRole("textbox", { name: "On hold reason" }).fill("Waiting on the vendor's window");
    await dialog.getByRole("group", { name: "Rollback plan", exact: true }).locator(".editor-input").click();
    await page.keyboard.type("Restore the previous certificates.");
    await dialog.getByRole("textbox", { name: "Additional comment (customer-visible)" }).fill("We will rotate the certificates.");
    await dialog.getByRole("button", { name: "Apply to 2 change requests" }).click();

    await expect(page.getByText("Updated 2 of 2 change requests.")).toBeVisible();
    expect(api.patches).toHaveLength(2);
    const sentTo = (id: string): Record<string, unknown> | undefined => api.patches.find((p) => p.path.endsWith(id))?.body;
    expect(sentTo(CHANGE_REQUESTS[0].id)).toEqual({
      title: "Rotate the gateway certificates",
      onHold: true,
      onHoldReason: "Waiting on the vendor's window",
      rollbackPlan: expect.stringContaining("Restore the previous certificates."),
      comment: "We will rotate the certificates.",
    });
    expect(sentTo(CHANGE_REQUESTS[1].id)).toEqual({
      onHold: true,
      onHoldReason: "Waiting on the vendor's window",
      comment: "We will rotate the certificates.",
    });
  });
});

test.describe("incident list — bulk actions", () => {
  test("Edit resolves every selected incident with one resolution", async ({ page }) => {
    const api = await fakeLists(page);
    await page.goto("/operations/incidents");

    await expect(page.getByRole("checkbox", { name: "INC-E2E-3 is closed and can't be selected" })).toBeDisabled();
    await page.getByRole("checkbox", { name: "Select all incidents on this page" }).check();
    await page.getByRole("button", { name: "Edit (2)" }).click();
    const dialog = page.getByRole("dialog", { name: "Edit 2 incidents" });

    await dialog.getByRole("combobox", { name: "State" }).click();
    await page.getByRole("option", { name: "Resolved" }).click();
    await expect(dialog.getByText("Resolved needs a resolution code and notes.")).toBeVisible();
    await dialog.getByRole("combobox", { name: "Resolution code" }).click();
    await page.getByRole("option", { name: "Solved (Permanently)" }).click();
    await dialog.getByRole("textbox", { name: /Resolution notes/ }).fill("Freed the volume.");
    await dialog.getByRole("button", { name: "Apply to 2 incidents" }).click();

    await expect(page.getByText("Updated 2 of 2 incidents.")).toBeVisible();
    const body = { state: "RESOLVED", resolutionCode: "SOLVED_PERMANENTLY", resolutionNotes: "Freed the volume." };
    expect(api.patches).toHaveLength(2);
    expect(api.patches).toEqual(
      expect.arrayContaining([
        { path: `/incidents/${INCIDENTS[0].id}`, body },
        { path: `/incidents/${INCIDENTS[1].id}`, body },
      ]),
    );
  });

  test("Edit adds a watcher to each incident's own watch list and sets fields from several sections", async ({ page }) => {
    const api = await fakeLists(page);
    await page.goto("/operations/incidents");

    await page.getByRole("checkbox", { name: "Select all incidents on this page" }).check();
    await page.getByRole("button", { name: "Edit (2)" }).click();
    const dialog = page.getByRole("dialog", { name: "Edit 2 incidents" });

    await dialog.getByRole("textbox", { name: "Short description" }).fill("Storage alerts");
    await dialog.getByRole("combobox", { name: "Channel" }).click();
    await page.getByRole("option", { name: "Email", exact: true }).click();
    await dialog.getByRole("textbox", { name: "Additional comment (customer-visible)" }).fill("We are looking into it.");

    // The change request picker searches the (faked) change request list.
    await dialog.getByRole("combobox", { name: "Change request" }).click();
    await page.getByRole("option", { name: /CHG-E2E-1/ }).click();

    // A real person from the local directory, added to each incident's own list.
    const watch = dialog.getByRole("combobox", { name: "Watch list" });
    await watch.click();
    await watch.fill("alice");
    const alice = page.getByRole("option", { name: /alice/i }).first();
    await expect(alice).toBeVisible();
    await alice.click();
    await dialog.getByRole("heading", { name: "Watch list" }).click();

    await dialog.getByRole("button", { name: "Apply to 2 incidents" }).click();
    await expect(page.getByText("Updated 2 of 2 incidents.")).toBeVisible();

    expect(api.patches).toHaveLength(2);
    const first = api.patches.find((p) => p.path.endsWith(INCIDENTS[0].id))!;
    const second = api.patches.find((p) => p.path.endsWith(INCIDENTS[1].id))!;
    for (const p of [first, second]) {
      expect(p.body).toMatchObject({
        subject: "Storage alerts",
        contactType: "EMAIL",
        additionalComments: "We are looking into it.",
        changeRequestId: CHANGE_REQUESTS[0].id,
      });
    }
    // Each incident is sent only the person added, never its whole list: the
    // backend adds them to what each incident has, so INC-E2E-1 keeps its
    // existing watcher and nobody's concurrent change is lost.
    const aliceId = (second.body.addWatchers as string[])[0];
    expect(first.body.addWatchers).toEqual([aliceId]);
    expect(second.body.addWatchers).toEqual([aliceId]);
    expect(first.body).not.toHaveProperty("watchList");
    expect(second.body).not.toHaveProperty("watchList");
  });

  test("Assign to me assigns every selected incident", async ({ page }) => {
    const api = await fakeLists(page);
    const me = myUserId(page);
    await page.goto("/operations/incidents");
    const userId = await me;

    await page.getByRole("checkbox", { name: "Select INC-E2E-2" }).check();
    await page.getByRole("button", { name: "Assign to me (1)" }).click();
    await expect(page.getByText("Assigned to you 1 of 1 incidents.")).toBeVisible();
    expect(api.patches).toEqual([{ path: `/incidents/${INCIDENTS[1].id}`, body: { assignedEngineerId: userId } }]);
  });
});

test.describe("list export", () => {
  for (const { label, extension } of [
    { label: "CSV", extension: "csv" },
    { label: "Excel (.xlsx)", extension: "xlsx" },
    { label: "JSON", extension: "json" },
    { label: "PDF", extension: "pdf" },
  ]) {
    test(`exports the filtered change requests as ${label}`, async ({ page }) => {
      await fakeLists(page);
      await page.goto("/operations/change-requests");
      await expect(page.getByText("CHG-E2E-1")).toBeVisible();

      await page.getByRole("button", { name: "Export" }).click();
      const downloading = page.waitForEvent("download");
      await page.getByRole("menuitem", { name: label }).click();
      const download = await downloading;
      expect(download.suggestedFilename()).toMatch(new RegExp(`^change-requests-export-.+\\.${extension.replace(".", "\\.")}$`));
      const bytes = fs.readFileSync((await download.path())!);

      if (extension === "json") {
        const rows = JSON.parse(bytes.toString("utf8")) as Array<Record<string, string>>;
        expect(rows.map((r) => r.Number)).toEqual(["CHG-E2E-1", "CHG-E2E-2", "CHG-E2E-3"]);
        expect(rows[0]).toMatchObject({ Subject: "Rotate certificates", Project: "E2E Project", State: "New" });
      } else if (extension === "csv") {
        const text = bytes.toString("utf8");
        expect(text.split("\r\n")[0]).toBe(`"Number","Subject","Project","State","Impact","Planned start","Updated"`);
        expect(text).toContain(`"CHG-E2E-3","Old upgrade"`);
      } else if (extension === "xlsx") {
        expect(bytes.subarray(0, 2).toString("latin1")).toBe("PK");
        expect(bytes.toString("utf8")).toContain("<t xml:space=\"preserve\">Patch gateway</t>");
      } else {
        expect(bytes.subarray(0, 5).toString("latin1")).toBe("%PDF-");
      }
      await expect(page.getByText(`Exported 3 change requests to ${label}.`)).toBeVisible();
    });
  }

  test("exports the filtered incidents as JSON", async ({ page }) => {
    await fakeLists(page);
    await page.goto("/operations/incidents");
    await expect(page.getByText("INC-E2E-1")).toBeVisible();
    await page.getByRole("button", { name: "Export" }).click();
    const downloading = page.waitForEvent("download");
    await page.getByRole("menuitem", { name: "JSON" }).click();
    const rows = JSON.parse(fs.readFileSync((await (await downloading).path())!, "utf8")) as Array<Record<string, string>>;
    expect(rows.map((r) => r.Number)).toEqual(["INC-E2E-1", "INC-E2E-2", "INC-E2E-3"]);
  });
});
