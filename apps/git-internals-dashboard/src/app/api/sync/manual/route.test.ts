import { NextRequest } from "next/server";
import { afterEach, describe, expect, it } from "vitest";
import { jobLock } from "@/server/jobs/lock";
import { POST } from "./route";

// Helper to create a NextRequest for the POST /api/sync/manual endpoint
function req(): NextRequest {
  return new NextRequest("https://app.example/api/sync/manual", { method: "POST" });
}

// POST /api/sync/manual
// 400 when the token is unset (no seeded DB rows needed — SyncTokenMissingError
// is the very first thing runIncrementalSync throws, inside the lock's fn);
// 409 when the job lock is already held. The first case still needs a live
// Postgres connection because jobLock.tryRun acquires a real advisory lock
// before running fn — same DB the rest of the suite already depends on.
describe("POST /api/sync/manual", () => {
  const originalToken = process.env.SEED_GITHUB_TOKEN;
  const originalMode = process.env.AUTH_MODE;

  afterEach(() => {
    process.env.SEED_GITHUB_TOKEN = originalToken;
    process.env.AUTH_MODE = originalMode;
    jobLock.running = false;
  });

  it("returns 400 sync_token_missing when SEED_GITHUB_TOKEN is unset", async () => {
    process.env.AUTH_MODE = "stub";
    process.env.SEED_GITHUB_TOKEN = "";
    const res = await POST(req(), { params: Promise.resolve({}) });
    expect(res.status).toBe(400);
    expect(await res.json()).toMatchObject({ error: "sync_token_missing" });
  });

  it("returns 409 sync_in_progress when the job lock is already held", async () => {
    process.env.AUTH_MODE = "stub";
    jobLock.running = true; // simulate a hanging job, in-process fast path
    const res = await POST(req(), { params: Promise.resolve({}) });
    expect(res.status).toBe(409);
    expect(await res.json()).toEqual({ error: "sync_in_progress" });
  });
});
