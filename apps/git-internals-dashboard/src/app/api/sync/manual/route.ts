import { runIncrementalSync, SyncTokenMissingError } from "@/server/db";
import { authorizeSync, requireAuth } from "@/server/auth";
import { jobLock } from "@/server/jobs/lock";
import { runTickOnce } from "@/server/jobs/recompute";
import { logger } from "@/server/lib/logger";
import { RateLimiter } from "@/server/lib/rate-limit";

// 3 manual syncs per user per minute — this route causes GitHub API egress
// (shared rate-limit budget across all users) and DB writes; the job lock
// already prevents concurrent runs, this bounds sequential hammering.
const syncRateLimiter = new RateLimiter(3, 60_000);

// POST /api/sync/manual — incremental GitHub fetch, then an immediate recompute tick.
// jobLock.tryRun is itself async (it awaits a Postgres advisory-lock round
// trip before deciding), so — unlike v2's synchronous in-process check — the
// busy/free decision can only be read off the *resolved* value, not the
// return of the call itself.
export const POST = requireAuth(async (_req, _ctx, user) => {
  if (!authorizeSync(user)) {
    return Response.json(
      { error: "not_authorized", message: "You are not authorized to trigger a sync." },
      { status: 403 },
    );
  }
  if (!syncRateLimiter.allow(user.sub)) {
    return Response.json({ error: "rate_limited", message: "Too many sync requests — try again shortly." }, { status: 429 });
  }
  try {
    const summary = await jobLock.tryRun(async () => {
      const s = await runIncrementalSync(logger, user.sub || undefined);
      await runTickOnce(logger);
      return s;
    });
    if (summary == null) return Response.json({ error: "sync_in_progress" }, { status: 409 });
    return Response.json(summary);
  } catch (err) {
    if (err instanceof SyncTokenMissingError) {
      return Response.json({ error: "sync_token_missing", message: err.message }, { status: 400 });
    }
    throw err;
  }
});
